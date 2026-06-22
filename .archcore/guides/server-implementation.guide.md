---
title: "Implementing the Send Server (Go)"
status: accepted
---

## Prerequisites

- Go (recent stable). No `age` server-side (crypto is client-only).
- Contracts: [[backend-http-api]], [[send-format]]. Decisions: [[go-single-binary-server]], [[storage-abstraction]], [[zero-knowledge-backend]]. Rules: [[security-privacy]], [[size-limits]].

## Steps

### 1. Package layout
```text
cmd/sendd/main.go        # wire config, store, router, GC; ListenAndServe
internal/api/            # handlers: create, upload, finalize, redeem, download, meta, health
internal/store/
  store.go               # SendStore interface + types
  sqlite.go              # SQLiteState (modernc.org/sqlite, cgo-free)
  fsblob.go              # FilesystemBlob
  s3blob.go              # S3Blob (optional)
internal/gc/             # background sweeper
internal/config/         # env config
```

### 2. SendStore (Go)
```go
type CreateSendInput struct { Version string; OneTime bool; TTL time.Duration; Parts []PartMeta }
type PartMeta   struct { PartID string; EncryptedSize int64; SHA256 string }
type SendRecord struct { ID string; ExpiresAt time.Time; OneTime bool }
type RedeemGrant struct { Token string; ExpiresAt time.Time; Parts []PartMeta }

type SendStore interface {
    CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error)
    PutPart(ctx context.Context, sendID, partID string, r io.Reader, m PartMeta) error
    FinalizeSend(ctx context.Context, sendID string) error
    RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error)        // atomic one-time
    GetPart(ctx context.Context, sendID, partID, redeemToken string) (io.ReadCloser, error)
    DeleteExpired(ctx context.Context, now time.Time) (int, error)
}
```

### 3. Schema
```sql
CREATE TABLE sends (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('creating','finalized','expired','deleted')),
  one_time INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL, finalized_at TEXT, expires_at TEXT NOT NULL, consumed_at TEXT,
  total_encrypted_size INTEGER NOT NULL DEFAULT 0, part_count INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE send_parts (
  send_id TEXT NOT NULL REFERENCES sends(id) ON DELETE CASCADE,
  part_id TEXT NOT NULL, storage_key TEXT NOT NULL,
  encrypted_size INTEGER NOT NULL, sha256 TEXT NOT NULL, uploaded_at TEXT,
  PRIMARY KEY (send_id, part_id)
);
CREATE TABLE redeem_grants (
  token_hash TEXT PRIMARY KEY, send_id TEXT NOT NULL REFERENCES sends(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL, expires_at TEXT NOT NULL, client_ip_hash TEXT, user_agent_hash TEXT
);
```
```mermaid
erDiagram
  sends ||--o{ send_parts : has
  sends ||--o{ redeem_grants : opens
  sends { string id PK }
  send_parts { string send_id FK; string part_id; string storage_key; string sha256 }
  redeem_grants { string token_hash PK; string send_id FK; string expires_at }
```

### 4. Atomic one-time redeem
```sql
UPDATE sends SET consumed_at = :now
 WHERE id = :id AND one_time = 1 AND consumed_at IS NULL
   AND status = 'finalized' AND expires_at > :now
RETURNING id;
```
A returned row → insert a `redeem_grants` row (SHA-256 of the token, +10 min) and return the token **once**. No row → map to `410 SEND_ALREADY_REDEEMED` / `SEND_EXPIRED`. Store only token hashes; compare in constant time ([[security-privacy]] R7).

### 5. Handlers
One per endpoint in [[backend-http-api]]: validate caps ([[size-limits]]); verify upload `sha256` + byte count; **stream** blobs (don't buffer large details); set `Content-Type: application/octet-stream`. Never log bodies/tokens/fragments ([[security-privacy]] R5).

### 6. GC worker (`internal/gc`)
Periodic: delete `expired`; delete `consumed` past the grant window; delete `creating` older than 1h; remove orphan blobs. Built on `DeleteExpired` + a blob sweep.

### 7. Config (env)
```text
SEND_LISTEN=:8080            SEND_PUBLIC_URL=https://send.example.com
SEND_DB_PATH=/data/sends.db  SEND_BLOB_DIR=/data/blobs
SEND_MAX_TTL=168h            SEND_DEFAULT_TTL=24h
SEND_MAX_TOTAL_BYTES=26214400  SEND_MAX_PART_BYTES=52428800
SEND_RATE_CREATE_PER_MIN=…   SEND_TEAM_TOKEN=… (optional)
# S3 (optional): SEND_S3_ENDPOINT / BUCKET / REGION / ACCESS_KEY / SECRET_KEY
```

## Verification
- `curl /healthz` → ok.
- Scripted round-trip: create → put parts → finalize → redeem → get parts (bytes are valid `age` ciphertext).
- Concurrency test: N parallel redeems → exactly one `200`, the rest `410` (atomicity).
- At-rest check: blobs are `age` files; metadata holds no plaintext titles.

## Common Issues
- **Static build / cgo** → use `modernc.org/sqlite` (pure Go) ([[go-single-binary-server]]).
- **Redeem race** → the conditional `UPDATE … RETURNING` MUST be the only consume path; no read-then-write.
- **Large-detail memory** → stream `GetPart`/`PutPart`.
- **Presigned URLs bypassing one-time** → issue only post-redeem, short TTL ([[backend-http-api]]).