package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FilesystemBlob stores ciphertext parts as files under a single directory, keyed
// by "<sendID>/<partID>". Writes are streamed (never buffered whole), verified,
// then atomically published via rename.
type FilesystemBlob struct {
	dir string
}

// OpenFilesystemBlob ensures dir exists and returns a blob store rooted there.
func OpenFilesystemBlob(dir string) (*FilesystemBlob, error) {
	if dir == "" {
		return nil, fmt.Errorf("blob dir must not be empty")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs blob dir: %w", err)
	}
	return &FilesystemBlob{dir: filepath.Clean(abs)}, nil
}

// resolve maps an opaque storage key to a safe absolute path strictly under dir,
// rejecting traversal and absolute keys.
func (b *FilesystemBlob) resolve(key string) (string, error) {
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		return "", fmt.Errorf("invalid storage key")
	}
	p := filepath.Join(b.dir, filepath.Clean("/"+key))
	if p != b.dir && !strings.HasPrefix(p, b.dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage key escapes blob dir")
	}
	return p, nil
}

func (b *FilesystemBlob) Put(ctx context.Context, key string, r io.Reader, want PartMeta, limit int64) (err error) {
	p, err := b.resolve(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return fmt.Errorf("mkdir blob parent: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	published := false
	defer func() {
		if !published {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	h := sha256.New()
	// limit+1 so a stream exactly at limit passes but one byte over trips.
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(r, limit+1))
	if err != nil {
		return fmt.Errorf("write blob: %w", err)
	}
	if n > limit {
		return ErrPartTooLarge
	}

	// Verify BEFORE publishing: a mismatched blob never becomes visible.
	if n != want.EncryptedSize || hex.EncodeToString(h.Sum(nil)) != want.SHA256 {
		return ErrIntegrity
	}
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		return fmt.Errorf("publish blob: %w", err)
	}
	published = true
	return nil
}

func (b *FilesystemBlob) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	p, err := b.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("open blob: %w", err)
	}
	return f, nil
}

func (b *FilesystemBlob) Delete(ctx context.Context, key string) error {
	p, err := b.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blob: %w", err)
	}
	// Best-effort: drop an emptied send directory.
	_ = os.Remove(filepath.Dir(p))
	return nil
}

// List returns every stored blob's key ("<sendID>/<partID>") for the GC orphan sweep.
func (b *FilesystemBlob) List(ctx context.Context) ([]string, error) {
	var keys []string
	err := filepath.WalkDir(b.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".tmp-") {
			return nil
		}
		rel, err := filepath.Rel(b.dir, path)
		if err != nil {
			return err
		}
		keys = append(keys, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list blobs: %w", err)
	}
	return keys, nil
}
