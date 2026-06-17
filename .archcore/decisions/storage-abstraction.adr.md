---
title: "Storage abstraction: the SendStore interface"
status: accepted
---

## Context

v0 should be trivially self-hostable (SQLite + filesystem) yet able to grow to hosted scale (Postgres + S3/R2) and eventually Durable Streams — **without changing `/send` UX or the public API**.

## Decision

Split storage into a **state store** (lifecycle/redemption coordination) and a **blob store** (encrypted parts), behind one Go interface. v0 ships `SQLiteState` + `FilesystemBlob`; an `S3Blob` impl is a drop-in. The public HTTP API never exposes storage keys, DB ids, or stream internals.

```go
type SendStore interface {
    CreateSend(ctx context.Context, in CreateSendInput) (SendRecord, error)
    PutPart(ctx context.Context, sendID, partID string, r io.Reader, m PartMeta) error
    FinalizeSend(ctx context.Context, sendID string) error
    RedeemSend(ctx context.Context, sendID string) (RedeemGrant, error)      // atomic one-time
    GetPart(ctx context.Context, sendID, partID, redeemToken string) (io.ReadCloser, error)
    DeleteExpired(ctx context.Context, now time.Time) (int, error)
}
```

## Alternatives

- **Object storage only** (metadata as JSON in a bucket): cannot do atomic one-time redeem reliably. Rejected.
- **Couple the API to S3 presigned URLs directly**: leaks storage and complicates one-time semantics. Rejected — presign only *after* redeem with a very short TTL, if used at all.

## Consequences

- Migration between Hetzner Object Storage / DO Spaces / OVH / Tigris / B2 / S3 is a **blob-impl swap** ([[self-host-deploy]]).
- A future `DurableStreamsStore` fits the same interface ([[roadmap]]).
- Adding a backend is a bounded, testable task ([[add-storage-backend]]).
- Slightly more upfront structure; justified by portability. Normative API stays as [[backend-http-api]].