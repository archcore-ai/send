package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

func sha256hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func newBlob(t *testing.T) *FilesystemBlob {
	t.Helper()
	b, err := OpenFilesystemBlob(t.TempDir())
	if err != nil {
		t.Fatalf("OpenFilesystemBlob: %v", err)
	}
	return b
}

func want(b []byte) PartMeta {
	return PartMeta{EncryptedSize: int64(len(b)), SHA256: sha256hex(b)}
}

func TestBlobPutAndOpen(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	data := []byte("ciphertext-bytes")
	n, err := b.Put(ctx, "snd_x/part_0001", bytes.NewReader(data), want(data), 1<<20)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if int(n) != len(data) {
		t.Fatalf("Put returned n=%d, want %d", n, len(data))
	}
	rc, err := b.Open(ctx, "snd_x/part_0001")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, data) {
		t.Errorf("read = %q, want %q", got, data)
	}
}

func TestBlobIntegrityMismatchNotPublished(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	data := []byte("hello")
	bad := PartMeta{EncryptedSize: int64(len(data)), SHA256: sha256hex([]byte("different"))}
	if _, err := b.Put(ctx, "snd_x/p", bytes.NewReader(data), bad, 1<<20); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Put sha mismatch = %v, want ErrIntegrity", err)
	}
	if _, err := b.Open(ctx, "snd_x/p"); !errors.Is(err, ErrNotFound) {
		t.Errorf("blob was published despite mismatch: %v", err)
	}
}

func TestBlobSizeMismatch(t *testing.T) {
	b := newBlob(t)
	data := []byte("hello")
	bad := PartMeta{EncryptedSize: 999, SHA256: sha256hex(data)}
	if _, err := b.Put(context.Background(), "snd_x/p", bytes.NewReader(data), bad, 1<<20); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("Put size mismatch = %v, want ErrIntegrity", err)
	}
}

func TestBlobTooLarge(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	data := bytes.Repeat([]byte("x"), 100)
	if _, err := b.Put(ctx, "snd_x/p", bytes.NewReader(data), want(data), 50); !errors.Is(err, ErrPartTooLarge) {
		t.Fatalf("Put over limit = %v, want ErrPartTooLarge", err)
	}
	if _, err := b.Open(ctx, "snd_x/p"); !errors.Is(err, ErrNotFound) {
		t.Errorf("oversize blob was published: %v", err)
	}
}

func TestBlobTraversalRejected(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	for _, key := range []string{"../escape", "/etc/passwd", "a/../../b", ""} {
		if _, err := b.Put(ctx, key, bytes.NewReader([]byte("x")), want([]byte("x")), 1<<20); err == nil {
			t.Errorf("Put(%q) accepted a traversal/invalid key", key)
		}
	}
}

func TestBlobListAndDelete(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	for _, k := range []string{"snd_a/manifest", "snd_a/part_0001", "snd_b/manifest"} {
		d := []byte(k)
		if _, err := b.Put(ctx, k, bytes.NewReader(d), want(d), 1<<20); err != nil {
			t.Fatalf("Put %s: %v", k, err)
		}
	}
	keys, err := b.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("List = %d keys, want 3 (%v)", len(keys), keys)
	}
	if err := b.Delete(ctx, "snd_a/part_0001"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	keys, _ = b.List(ctx)
	if len(keys) != 2 {
		t.Errorf("after delete List = %d keys, want 2", len(keys))
	}
	// Deleting a missing key is not an error (idempotent GC).
	if err := b.Delete(ctx, "snd_a/part_0001"); err != nil {
		t.Errorf("re-delete: %v", err)
	}
}

func TestBlobIdempotentOverwrite(t *testing.T) {
	b := newBlob(t)
	ctx := context.Background()
	data := []byte("same-bytes")
	for range 2 {
		if _, err := b.Put(ctx, "snd_x/p", bytes.NewReader(data), want(data), 1<<20); err != nil {
			t.Fatalf("Put: %v", err)
		}
	}
	rc, _ := b.Open(ctx, "snd_x/p")
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, data) {
		t.Errorf("read = %q, want %q", got, data)
	}
}
