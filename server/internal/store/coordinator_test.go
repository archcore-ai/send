package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func newCoordinator(t *testing.T) (*Coordinator, *clock) {
	t.Helper()
	cl := &clock{t: time.Unix(1_700_000_000, 0).UTC()}
	st, err := OpenSQLiteState(filepath.Join(t.TempDir(), "c.db"), 10*time.Minute, cl.now)
	if err != nil {
		t.Fatalf("OpenSQLiteState: %v", err)
	}
	blob, err := OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystemBlob: %v", err)
	}
	c := NewCoordinator(st, blob, 1<<20)
	t.Cleanup(func() { _ = c.Close() })
	return c, cl
}

// createWith declares a one-part send whose single part has the given bytes.
func createWith(t *testing.T, c *Coordinator, body []byte) (sendID, partID string) {
	t.Helper()
	in := CreateSendInput{Version: "send.v1", OneTime: true, TTL: time.Hour, Parts: []PartMeta{
		{PartID: "part_0001", EncryptedSize: int64(len(body)), SHA256: sha256hex(body)},
	}}
	rec, err := c.CreateSend(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateSend: %v", err)
	}
	return rec.ID, "part_0001"
}

func TestCoordinatorRoundTrip(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	body := []byte("opaque-ciphertext")
	id, pid := createWith(t, c, body)

	if _, err := c.PutPart(ctx, id, pid, bytes.NewReader(body), PartMeta{SHA256: sha256hex(body)}); err != nil {
		t.Fatalf("PutPart: %v", err)
	}
	if err := c.FinalizeSend(ctx, id); err != nil {
		t.Fatalf("FinalizeSend: %v", err)
	}
	grant, err := c.RedeemSend(ctx, id)
	if err != nil {
		t.Fatalf("RedeemSend: %v", err)
	}
	rc, err := c.GetPart(ctx, id, pid, grant.Token)
	if err != nil {
		t.Fatalf("GetPart: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, body) {
		t.Errorf("downloaded = %q, want %q", got, body)
	}
}

func TestCoordinatorIntegrityRollback(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	body := []byte("declared-bytes")
	id, pid := createWith(t, c, body)

	// Upload bytes that don't match the declaration → 422, and no part recorded.
	if _, err := c.PutPart(ctx, id, pid, bytes.NewReader([]byte("tampered")), PartMeta{}); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("PutPart mismatch = %v, want ErrIntegrity", err)
	}
	// Finalize must still see the part as missing.
	if err := c.FinalizeSend(ctx, id); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("FinalizeSend after rollback = %v, want ErrIncomplete", err)
	}
	// No orphan blob should have been published.
	stored, _ := c.blob.List(ctx)
	if len(stored) != 0 {
		t.Errorf("blob published despite integrity failure: %v", stored)
	}
}

func TestCoordinatorUploadToFinalizedIsConflict(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	body := []byte("x")
	id, pid := createWith(t, c, body)
	if _, err := c.PutPart(ctx, id, pid, bytes.NewReader(body), PartMeta{}); err != nil {
		t.Fatalf("PutPart: %v", err)
	}
	if err := c.FinalizeSend(ctx, id); err != nil {
		t.Fatalf("FinalizeSend: %v", err)
	}
	if _, err := c.PutPart(ctx, id, pid, bytes.NewReader(body), PartMeta{}); !errors.Is(err, ErrConflict) {
		t.Errorf("upload after finalize = %v, want ErrConflict", err)
	}
}

func TestCoordinatorGetPartRequiresGrant(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	body := []byte("x")
	id, pid := createWith(t, c, body)
	_, _ = c.PutPart(ctx, id, pid, bytes.NewReader(body), PartMeta{})
	_ = c.FinalizeSend(ctx, id)
	if _, err := c.GetPart(ctx, id, pid, "red_bogus"); !errors.Is(err, ErrInvalidGrant) {
		t.Errorf("GetPart with bad token = %v, want ErrInvalidGrant", err)
	}
}

func TestCoordinatorOrphanSweep(t *testing.T) {
	c, _ := newCoordinator(t)
	ctx := context.Background()
	// Publish a blob with no corresponding part row (simulates a crash mid-upload).
	orphan := []byte("orphan")
	if _, err := c.blob.Put(ctx, StorageKey("snd_ghost", "part_0001"), bytes.NewReader(orphan), want(orphan), 1<<20); err != nil {
		t.Fatalf("seed orphan: %v", err)
	}
	n, err := c.OrphanSweep(ctx)
	if err != nil {
		t.Fatalf("OrphanSweep: %v", err)
	}
	if n != 1 {
		t.Errorf("swept = %d, want 1", n)
	}
}
