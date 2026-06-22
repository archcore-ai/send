// Package gc runs the background sweeper that deletes expired sends, consumed
// one-time sends past their grant window, stale unfinished uploads, and orphan
// blobs. It drives the store's Reaper on a fixed interval.
package gc

import (
	"context"
	"time"

	"github.com/ivklgn/archcore-send/internal/store"
)

// Worker periodically reaps dead sends and orphan blobs.
type Worker struct {
	reaper   store.Reaper
	interval time.Duration
	now      func() time.Time
	report   func(sends, orphans int, err error)
}

// New builds a worker. now may be nil (time.Now); report may be nil (no logging).
func New(reaper store.Reaper, interval time.Duration, now func() time.Time, report func(sends, orphans int, err error)) *Worker {
	if now == nil {
		now = time.Now
	}
	return &Worker{reaper: reaper, interval: interval, now: now, report: report}
}

// Run sweeps once immediately, then every interval, until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	t := time.NewTicker(w.interval)
	defer t.Stop()
	w.Sweep(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			w.Sweep(ctx)
		}
	}
}

// Sweep performs a single reap pass. Exported so tests can drive it deterministically.
func (w *Worker) Sweep(ctx context.Context) {
	sends, err := w.reaper.DeleteExpired(ctx, w.now())
	if err != nil {
		w.notify(0, 0, err)
		return
	}
	orphans, err := w.reaper.OrphanSweep(ctx)
	w.notify(sends, orphans, err)
}

func (w *Worker) notify(sends, orphans int, err error) {
	if w.report != nil && (sends > 0 || orphans > 0 || err != nil) {
		w.report(sends, orphans, err)
	}
}
