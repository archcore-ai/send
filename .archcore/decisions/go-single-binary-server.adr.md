---
title: "Go single static binary for the Send server"
status: accepted
---

## Context

The server must be maximally easy to **self-host** and **audit**, with zero runtime dependencies and a small attack surface. Stack decision locked with the project owner.

## Decision

Implement the server in **Go**, shipped as a **single static binary** in a `scratch`/`distroless` container:

- standard-library HTTP (or a thin router such as `chi`);
- `modernc.org/sqlite` (cgo-free) for the state store → static builds stay trivial;
- filesystem blob store by default; S3 via `minio-go`/AWS SDK when enabled;
- no `age` on the server (crypto is client-side only).

## Alternatives

- **TypeScript / Node (Fastify)**: ecosystem-consistent but adds a runtime, `node_modules`, and a larger supply-chain surface for a privacy tool. Rejected for v0.
- **Rust (Axum)**: excellent safety/perf but slower iteration and a smaller contributor pool. Rejected for v0.

## Consequences

- `docker run` with **no runtime deps**; easy cross-compilation for ARM/x86; tiny images.
- cgo-free SQLite keeps fully static builds simple.
- The team follows the Go layout and patterns in [[server-implementation]].
- Aligns with the boring, minimal posture of [[zero-knowledge-backend]].