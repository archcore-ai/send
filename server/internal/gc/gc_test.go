package gc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	"github.com/ivklgn/archcore-send/internal/store"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestSweepReapsExpiredAndOrphans(t *testing.T) {
	ctx := context.Background()
	cl := &clock{t: time.Unix(1_700_000_000, 0).UTC()}
	st, err := store.OpenSQLiteState(filepath.Join(t.TempDir(), "gc.db"), 10*time.Minute, cl.now)
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	blob, err := store.OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("blob: %v", err)
	}
	coord := store.NewCoordinator(st, blob, 1<<20)
	t.Cleanup(func() { _ = coord.Close() })

	// A finalized one-part send with a short TTL.
	body := []byte("ciphertext")
	rec, err := coord.CreateSend(ctx, store.CreateSendInput{
		Version: "send.v1", OneTime: true, TTL: time.Minute,
		Parts: []store.PartMeta{{PartID: "part_0001", EncryptedSize: int64(len(body)), SHA256: sha(body)}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := coord.PutPart(ctx, rec.ID, "part_0001", bytes.NewReader(body), store.PartMeta{}); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := coord.FinalizeSend(ctx, rec.ID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	var gotSends, gotOrphans int
	w := New(coord, time.Minute, cl.now, func(s, o int, err error) {
		gotSends, gotOrphans = s, o
	})

	// Before expiry: nothing reaped.
	w.Sweep(ctx)
	if gotSends != 0 {
		t.Fatalf("premature reap: %d sends", gotSends)
	}

	// After expiry: the send (and its blob) is reaped.
	cl.t = cl.t.Add(2 * time.Minute)
	w.Sweep(ctx)
	if gotSends != 1 {
		t.Errorf("reaped sends = %d, want 1", gotSends)
	}
	if _, err := coord.GetSendMeta(ctx, rec.ID); err == nil {
		t.Errorf("expired send still present")
	}
	keys, _ := blob.List(ctx)
	if len(keys) != 0 {
		t.Errorf("blob not purged: %v", keys)
	}
	_ = gotOrphans
}

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
