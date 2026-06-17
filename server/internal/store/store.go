// Package store defines the Send server's storage contract and its sentinel
// errors. A send's lifecycle and one-time redemption are coordinated by a
// StateStore (SQLite); the opaque ciphertext bytes live in a BlobStore
// (filesystem). The Coordinator composes both into the SendStore the API layer
// consumes. The server is zero-knowledge: nothing here decrypts, parses, or
// inspects part contents — parts are opaque blobs keyed by an opaque transport id.
package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"time"
)

// SendStatus is the lifecycle state of a send.
type SendStatus string

const (
	SendStatusCreating  SendStatus = "creating"
	SendStatusFinalized SendStatus = "finalized"
	SendStatusExpired   SendStatus = "expired"
	SendStatusDeleted   SendStatus = "deleted"
)

// Sentinel errors. The API layer maps these to HTTP status + error_code per the
// error-catalog spec; internal error text is never surfaced to clients.
var (
	ErrNotFound        = errors.New("send not found")
	ErrNotFinalized    = errors.New("send not finalized")
	ErrAlreadyRedeemed = errors.New("send already redeemed")
	ErrExpired         = errors.New("send expired")
	ErrIncomplete      = errors.New("send incomplete")
	ErrConflict        = errors.New("send not in creating state")
	ErrUnknownPart     = errors.New("unknown part id")
	ErrIntegrity       = errors.New("integrity check failed")
	ErrPartTooLarge    = errors.New("part exceeds size cap")
	ErrInvalidGrant    = errors.New("invalid or expired redeem grant")
)

// PartMeta is the opaque, server-visible metadata for one part: a transport id,
// the ciphertext byte count, and its hex sha256. No semantics.
type PartMeta struct {
	PartID        string
	EncryptedSize int64
	SHA256        string
}

// CreateSendInput is the validated input to create a send.
type CreateSendInput struct {
	Version string
	OneTime bool
	TTL     time.Duration
	Parts   []PartMeta
}

// SendRecord is returned on create: the assigned id, expiry, and declared parts.
type SendRecord struct {
	ID        string
	OneTime   bool
	ExpiresAt time.Time
	Parts     []PartMeta
}

// RedeemGrant is the short-lived download session opened by a successful redeem.
// Token is returned to the client exactly once; only its hash is stored.
type RedeemGrant struct {
	Token     string
	ExpiresAt time.Time
	Parts     []PartMeta
}

// SendMeta is the public, server-visible metadata for GET /v1/sends/{id}.
type SendMeta struct {
	ID                 string
	Status             SendStatus
	OneTime            bool
	ExpiresAt          time.Time
	PartCount          int
	TotalEncryptedSize int64
	Parts              []PartMeta
}

// SendStore is the contract the API layer depends on.
type SendStore interface {
	CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error)
	PutPart(ctx context.Context, sendID, partID string, r io.Reader, declared PartMeta) error
	FinalizeSend(ctx context.Context, sendID string) error
	RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error)
	GetPart(ctx context.Context, sendID, partID, redeemToken string) (io.ReadCloser, error)
	GetSendMeta(ctx context.Context, sendID string) (SendMeta, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

// Reaper is the subset the GC worker drives.
type Reaper interface {
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
	OrphanSweep(ctx context.Context) (int, error)
}

// StateStore coordinates lifecycle + redemption. Implementations MUST be
// concurrency-safe and MUST consume a one-time send via a single atomic
// conditional UPDATE (never read-then-write).
type StateStore interface {
	CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error)
	SendStatusOf(ctx context.Context, sendID string) (SendStatus, error)
	DeclaredPart(ctx context.Context, sendID, partID string) (meta PartMeta, uploaded bool, err error)
	MarkUploaded(ctx context.Context, sendID, partID string) error
	FinalizeSend(ctx context.Context, sendID string) error
	RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error)
	ValidateGrant(ctx context.Context, sendID, tokenHash string) error
	GetSendMeta(ctx context.Context, sendID string) (SendMeta, error)
	DeleteExpired(ctx context.Context, now time.Time) (sends int, purgeKeys []string, err error)
	LiveStorageKeys(ctx context.Context) ([]string, error)
	Close() error
}

// BlobStore stores opaque ciphertext keyed by an opaque storage key. Put streams
// r into a temp file (enforcing limit bytes, ErrPartTooLarge if exceeded; sha256
// computed during the copy), verifies the written bytes match want BEFORE
// atomically publishing the blob, and discards the temp file returning
// ErrIntegrity on mismatch. So a blob becomes visible only if it is correct.
type BlobStore interface {
	Put(ctx context.Context, key string, r io.Reader, want PartMeta, limit int64) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context) ([]string, error)
}

// StorageKey is the deterministic blob key for a part. Both the state store
// (persisted) and the coordinator (blob ops) derive it the same way.
func StorageKey(sendID, partID string) string {
	return sendID + "/" + partID
}

// HashToken returns the hex sha256 of a redeem/management token. Only the hash
// is ever persisted (security-privacy R7).
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
