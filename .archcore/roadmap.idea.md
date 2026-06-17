---
title: "Archcore Send — Roadmap & Future Paths"
status: draft
---

## Idea

Archcore Send v0 is a sealed, one-time **transfer** primitive. This document captures — deliberately lightly — the paths beyond v0 so today's decisions stay forward-compatible without over-building. Nothing here is in scope for v0; each item is a future option gated on real demand. The anchor is the [[archcore-send]] PRD non-goals list.

## Value

Recording the forward path now lets v0 keep a **stable public API** ([[backend-http-api]]) and a **storage abstraction** ([[storage-abstraction]]) that can absorb these changes without breaking `/send` UX. It also prevents premature adoption of heavy primitives (Durable Streams, Electric, CRDT) before their strengths are needed.

## Possible Implementation

Each phase is additive and backward-compatible. The public API stays storage-agnostic so internals can change underneath.

**v0.1 — hardening & ergonomics**
- Richer preview, better secret scanner ([[content-policy]]), `--load-detail` polish.
- Optional management/revoke token (`/m/{id}#mgmt=...`) printed once on create.
- Larger single-detail support.

**v1 — recipient public-key mode**
- `/send --to <recipient>` encrypts to a persistent `age` recipient key; the URL no longer carries the decryption key, so forwarding the link alone becomes insufficient.
- Optional team key directory; optional sender auth / API token enabling revoke + "my sends" listing.
- Strengthens the enterprise story. Supersedes part of [[e2ee-link-key-model]] (key handling only — format/API unchanged).

**v2 — progressive / streaming storage (Durable Streams)**
- Add a `DurableStreamsStore` implementation of [[storage-abstraction]]: a send becomes an append-only encrypted event stream (manifest event, compact event, evidence events, chunked detail events, sealed event).
- Enables resumable upload/download, offset reads, large chunked sends, and "append findings".
- E2EE invariant unchanged: the stream stores encrypted events only; the public API still enforces TTL / one-time / redeem.

**v3 — live context workspace (collaboration layer)**
- A separate product layer, not a backend tweak: presence, multi-reader live sends, forks.
- Postgres for users/teams/ACL/metadata; Electric Shapes to sync metadata to a web UI; Durable Streams for live event logs; Yjs/CRDT for collaborative editing.
- Crosses the transfer→collaboration boundary deliberately drawn in v0.

**Adjacent — web dashboard**
- "My sends", "shared with me", audit log, revoke UI. Requires sender auth (v1) + metadata sync (Electric).

## Risks

- **Scope creep** — every future phase is tempting; v0's value is *smallness*. Gate each phase on demonstrated demand.
- **Privacy regressions** — recipient-key mode, dashboards, and live mode add metadata and identity surface. Re-run the [[threat-model]] before each.
- **Primitive lock-in** — Durable Streams / Electric / Yjs are powerful but heavy. Keep them behind [[storage-abstraction]] and the stable HTTP API so adoption stays reversible.
- **RF availability** — any future hosted default must keep the [[hosting-posture-rf-availability]] constraint (no Cloudflare-sole-endpoint).