// Package sendd wires configuration, storage, the HTTP API, and the GC worker into
// a runnable server with graceful shutdown.
package sendd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/ivklgn/archcore-send/internal/api"
	"github.com/ivklgn/archcore-send/internal/config"
	"github.com/ivklgn/archcore-send/internal/gc"
	"github.com/ivklgn/archcore-send/internal/logx"
	"github.com/ivklgn/archcore-send/internal/ratelimit"
	"github.com/ivklgn/archcore-send/internal/store"
)

const shutdownTimeout = 10 * time.Second

// Run loads config, opens the stores, starts the GC worker and HTTP server, and
// blocks until ctx is cancelled, then shuts down gracefully.
func Run(ctx context.Context, version, commit string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logw := io.Writer(os.Stderr)
	if cfg.RequestLog != "" {
		f, err := os.OpenFile(cfg.RequestLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open request log: %w", err)
		}
		defer f.Close()
		logw = f
	}
	reqLog := logx.New(logw)

	state, err := store.OpenSQLiteState(cfg.DBPath, cfg.GrantTTL, nil)
	if err != nil {
		return fmt.Errorf("state store: %w", err)
	}
	defer state.Close()
	blob, err := store.OpenFilesystemBlob(cfg.BlobDir)
	if err != nil {
		return fmt.Errorf("blob store: %w", err)
	}
	coord := store.NewCoordinator(state, blob, cfg.MaxPartBytes)

	createLim := ratelimit.New(cfg.RateCreatePerMin, cfg.RateWindow, nil)
	uploadLim := ratelimit.New(cfg.RateUploadPerMin, cfg.RateWindow, nil)
	srv := api.New(coord, cfg, reqLog, createLim, uploadLim)

	worker := gc.New(coord, cfg.GCInterval, nil, func(sends, orphans int, err error) {
		if err != nil {
			log.Printf("gc: %v", err)
			return
		}
		log.Printf("gc: reaped %d sends, %d orphan blobs", sends, orphans)
	})
	go worker.Run(ctx)

	ln, err := net.Listen("tcp", cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Listen, err)
	}
	// The bound URL on stdout lets ephemeral-port callers (tests) discover the port;
	// all human logging goes to stderr so stdout stays a single clean line.
	fmt.Printf("http://%s\n", ln.Addr().String())
	log.Printf("sendd %s (%s) listening on http://%s", version, commit, ln.Addr())

	// Bound slow-loris and stuck connections. Read/Write timeouts are generous
	// enough to stream a max-size (25 MiB) part over a slow link while still
	// capping connections that never make progress.
	httpSrv := &http.Server{
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       120 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown: %w", err)
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	}
}
