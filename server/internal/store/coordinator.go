package store

import (
	"context"
	"io"
	"time"
)

// Coordinator composes a StateStore (lifecycle/redemption) and a BlobStore
// (ciphertext bytes) into the SendStore the API layer uses and the Reaper the GC
// worker drives. It owns the cross-store ordering invariants.
type Coordinator struct {
	state        StateStore
	blob         BlobStore
	maxPartBytes int64
}

// NewCoordinator wires a state + blob store. maxPartBytes caps a single uploaded part.
func NewCoordinator(state StateStore, blob BlobStore, maxPartBytes int64) *Coordinator {
	return &Coordinator{state: state, blob: blob, maxPartBytes: maxPartBytes}
}

var (
	_ SendStore = (*Coordinator)(nil)
	_ Reaper    = (*Coordinator)(nil)
)

func (c *Coordinator) CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error) {
	return c.state.CreateSend(ctx, in)
}

// PutPart verifies the streamed ciphertext against the create-time declaration and
// publishes the blob BEFORE recording the part as uploaded. A crash between the two
// leaves a blob whose part row has uploaded_at IS NULL → finalize stays INCOMPLETE
// and GC reaps both. The inverse order could expose a part row with missing bytes.
func (c *Coordinator) PutPart(ctx context.Context, sendID, partID string, r io.Reader, declared PartMeta) (int64, error) {
	status, err := c.state.SendStatusOf(ctx, sendID)
	if err != nil {
		return 0, err // ErrNotFound
	}
	if status != SendStatusCreating {
		return 0, ErrConflict // upload to a finalized/expired send
	}
	stored, _, err := c.state.DeclaredPart(ctx, sendID, partID)
	if err != nil {
		return 0, err // ErrUnknownPart
	}
	// The PUT header's sha (declared) must agree with what was declared at create.
	if declared.SHA256 != "" && declared.SHA256 != stored.SHA256 {
		return 0, ErrIntegrity
	}
	// blob.Put streams + verifies actual bytes against the create-time size+sha,
	// publishing only on a match (ErrIntegrity / ErrPartTooLarge otherwise).
	want := PartMeta{PartID: partID, EncryptedSize: stored.EncryptedSize, SHA256: stored.SHA256}
	n, err := c.blob.Put(ctx, StorageKey(sendID, partID), r, want, c.maxPartBytes)
	if err != nil {
		return 0, err
	}
	if err := c.state.MarkUploaded(ctx, sendID, partID); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *Coordinator) FinalizeSend(ctx context.Context, sendID string) error {
	return c.state.FinalizeSend(ctx, sendID)
}

func (c *Coordinator) RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error) {
	return c.state.RedeemSend(ctx, sendID)
}

// GetPart returns the ciphertext for a part, only with a valid unexpired grant.
func (c *Coordinator) GetPart(ctx context.Context, sendID, partID, redeemToken string) (io.ReadCloser, error) {
	if err := c.state.ValidateGrant(ctx, sendID, HashToken(redeemToken)); err != nil {
		return nil, err // ErrInvalidGrant
	}
	if _, _, err := c.state.DeclaredPart(ctx, sendID, partID); err != nil {
		return nil, err // ErrUnknownPart
	}
	return c.blob.Open(ctx, StorageKey(sendID, partID))
}

func (c *Coordinator) GetSendMeta(ctx context.Context, sendID string) (SendMeta, error) {
	return c.state.GetSendMeta(ctx, sendID)
}

// DeleteExpired purges expired/consumed/stale sends and their blobs.
func (c *Coordinator) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	sends, keys, err := c.state.DeleteExpired(ctx, now)
	if err != nil {
		return 0, err
	}
	for _, k := range keys {
		_ = c.blob.Delete(ctx, k) // best-effort; orphan sweep is the backstop
	}
	return sends, nil
}

// OrphanSweep deletes blobs no longer referenced by any part row (e.g. a crash
// between a published blob and its metadata commit).
func (c *Coordinator) OrphanSweep(ctx context.Context) (int, error) {
	live, err := c.state.LiveStorageKeys(ctx)
	if err != nil {
		return 0, err
	}
	stored, err := c.blob.List(ctx)
	if err != nil {
		return 0, err
	}
	liveSet := make(map[string]struct{}, len(live))
	for _, k := range live {
		liveSet[k] = struct{}{}
	}
	n := 0
	for _, k := range stored {
		if _, ok := liveSet[k]; !ok {
			_ = c.blob.Delete(ctx, k)
			n++
		}
	}
	return n, nil
}

func (c *Coordinator) Close() error {
	return c.state.Close()
}
