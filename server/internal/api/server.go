// Package api implements the Send server's HTTP surface: the /v1 contract plus
// health and (in landing.go) the public pages. Handlers validate-then-act and map
// store sentinel errors to the status/code in the error-catalog spec, never
// leaking internal error text. The server is zero-knowledge — it moves opaque
// ciphertext and never inspects part contents.
package api

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ivklgn/archcore-send/internal/config"
	"github.com/ivklgn/archcore-send/internal/logx"
	"github.com/ivklgn/archcore-send/internal/store"
)

// RateLimiter gates anonymous create/upload by client key. A nil limiter allows all.
type RateLimiter interface {
	Allow(key string) bool
}

// Server is the HTTP handler for the Send API.
type Server struct {
	store         store.SendStore
	cfg           config.Config
	log           *logx.Logger
	createLimiter RateLimiter
	uploadLimiter RateLimiter
	handler       http.Handler
}

// New builds a Server. The limiters and log may be nil (nil limiter = unlimited).
// cmd/sendd always supplies both, keyed by hashed client per security-privacy R6.
func New(st store.SendStore, cfg config.Config, log *logx.Logger, createLimiter, uploadLimiter RateLimiter) *Server {
	s := &Server{store: st, cfg: cfg, log: log, createLimiter: createLimiter, uploadLimiter: uploadLimiter}
	s.handler = s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	// Write endpoints are gated by the team token when SEND_TEAM_TOKEN is set.
	// Read/redeem stay anonymous: the unguessable id + #agekey fragment is the
	// capability (link mode), and redeem/download already enforce one-time + grant.
	mux.HandleFunc("POST /v1/sends", s.requireTeamToken(s.handleCreate))
	mux.HandleFunc("PUT /v1/sends/{id}/parts/{part_id}", s.requireTeamToken(s.handleUpload))
	mux.HandleFunc("POST /v1/sends/{id}/finalize", s.requireTeamToken(s.handleFinalize))
	mux.HandleFunc("POST /v1/sends/{id}/redeem", s.handleRedeem)
	mux.HandleFunc("GET /v1/sends/{id}/parts/{part_id}", s.handleDownload)
	mux.HandleFunc("GET /v1/sends/{id}", s.handleMeta)
	mux.HandleFunc("GET /s/{id}", s.handleLinkPage)
	mux.HandleFunc("GET /{$}", s.handleLanding)
	return s.recoverer(s.logging(mux))
}

// logging wraps the handler to emit one R5-compliant request line.
func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if s.log != nil {
			s.log.Request(r.Method, r.URL.Path, sw.statusOr200(), sw.bytes, time.Since(start), sw.errCode)
		}
	})
}

// recoverer turns a handler panic into a 500 without leaking internals.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeError(w, http.StatusInternalServerError, "STORAGE_ERROR", "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter records the status, bytes written, and a coarse error_code for logging.
type statusWriter struct {
	http.ResponseWriter
	status  int
	bytes   int64
	errCode string
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) statusOr200() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

// requireTeamToken gates a mutating handler behind SEND_TEAM_TOKEN. When the
// token is unset the server is in anonymous public mode and the handler runs
// unguarded; when set, the request must carry a matching Authorization: Bearer
// (compared in constant time per security-privacy). The client sends this header
// on create/upload/finalize only — never on the anonymous redeem/download path.
func (s *Server) requireTeamToken(next http.HandlerFunc) http.HandlerFunc {
	if s.cfg.TeamToken == "" {
		return next
	}
	want := []byte(s.cfg.TeamToken)
	return func(w http.ResponseWriter, r *http.Request) {
		var got string
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			got = strings.TrimPrefix(h, "Bearer ")
		}
		if subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "team token required")
			return
		}
		next(w, r)
	}
}

// allow applies the rate limiter (if any) for the given bucket, keyed by client.
func (s *Server) allow(r *http.Request, bucket string) bool {
	var lim RateLimiter
	switch bucket {
	case "create", "finalize", "redeem":
		lim = s.createLimiter
	case "upload", "download", "meta":
		lim = s.uploadLimiter
	}
	if lim == nil {
		return true
	}
	return lim.Allow(s.clientKey(r))
}

// clientKey derives the rate-limit key from the client address, honoring
// X-Forwarded-For only when behind a trusted proxy (SEND_TRUST_PROXY). The IP is
// hashed so raw addresses are never held in memory (security-privacy R6).
func (s *Server) clientKey(r *http.Request) string {
	var ip string
	if s.cfg.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			first, _, _ := strings.Cut(xff, ",")
			ip = strings.TrimSpace(first)
		}
	}
	if ip == "" {
		ip = r.RemoteAddr
		if i := strings.LastIndexByte(ip, ':'); i >= 0 {
			ip = ip[:i]
		}
	}
	sum := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(sum[:8])
}

// publicBase returns the external base URL ("scheme://host") for building links.
// SEND_PUBLIC_URL wins; otherwise it is derived from the request.
func (s *Server) publicBase(r *http.Request) string {
	if s.cfg.PublicURL != "" {
		return strings.TrimRight(s.cfg.PublicURL, "/")
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// bearerToken extracts the redeem grant from the Authorization header only. The
// token is never accepted as a query parameter, so it cannot leak via proxy
// access logs, browser history, or Referer headers.
func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}
