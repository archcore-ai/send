package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ivklgn/archcore-send/cmd/sendd"
)

// version and commit are injected at build time (GoReleaser -ldflags).
var (
	version = "dev"
	commit  = "none"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := sendd.Run(ctx, version, commit); err != nil {
		log.Fatalf("sendd: %v", err)
	}
}
