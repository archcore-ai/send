---
title: "Add a new SendStore storage backend"
status: accepted
---

## What

Implement a new `SendStore` backend (e.g. S3-compatible blob store, Postgres state) behind the existing interface, **without** changing the public API or `/send` UX.

## When

- Moving from filesystem to object storage for scale/durability.
- Switching providers (Hetzner OS → DO Spaces → Tigris → B2 → S3).
- Adding a Postgres state store for multi-node.

## Steps

1. Read [[storage-abstraction]] and [[backend-http-api]]; the impl MUST satisfy `SendStore` exactly.
2. Create `internal/store/<name>.go` implementing every method. Keep **state** (lifecycle/redeem) and **blob** (bytes) concerns separate — you usually swap only one.
3. Preserve invariants: atomic one-time redeem ([[practical-one-time-redemption]]); sha256 verification; streaming for large parts; never expose storage keys/ids ([[zero-knowledge-backend]]).
4. Wire via config (`SEND_S3_*` / `SEND_DB_*`); select the impl at startup in `cmd/sendd`.
5. Use a private bucket and least-privilege creds; if presigning, only **post-redeem** with a short TTL ([[security-privacy]], [[self-host-deploy]]).
6. Run the same conformance tests the FS/SQLite impls pass (round-trip; concurrent-redeem atomicity; at-rest ciphertext-only; GC).
7. Update provider notes in [[self-host-deploy]].

## Example

Add `S3Blob`: implement `PutPart` / `GetPart` / blob-delete against `minio-go`; keep `SQLiteState` for lifecycle; set `SEND_S3_ENDPOINT/BUCKET/...`; run the round-trip + concurrency suite against a private bucket.

## Pitfalls

- Reintroducing a read-then-write redeem (breaks atomicity) — keep the conditional `UPDATE` in the **state** store.
- Buffering whole parts in memory — stream instead.
- Public or long-lived object URLs — forbidden ([[security-privacy]]).
- Leaking storage keys / DB ids through the API — the public contract is [[backend-http-api]] only.
- Forgetting GC / orphan cleanup for the new backend.