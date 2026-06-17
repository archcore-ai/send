---
title: "Multipart encrypted send format"
status: accepted
---

## Context

Context size ranges from tiny handoffs to multi-MB debugging sessions. A single encrypted blob forces all-or-nothing download, risks dumping huge plaintext into the recipient's context window, and makes one-time semantics hostile to network failures.

## Decision

A send is **multipart**. Part kinds:

- `manifest` — reserved, encrypted private index (semantic map). Loaded first.
- `compact` — required working context, loaded by default.
- `evidence.*` — small supporting facts, loaded by default if small.
- `detail.*` — large optional parts, never auto-loaded.

Each part is **independently gzipped then `age`-encrypted** and uploaded/downloaded separately, all to the same ephemeral recipient. The full normative format is in [[send-format]].

## Alternatives

- **Single blob** (`send.json.gz.age`): simplest, but blocks staged loading, breaks lazy details, and is fragile under one-time + network interruption. Rejected for v0.
- **Encrypted tar of all parts**: still all-or-nothing on download. Rejected.

## Consequences

- Enables **compact-first loading** and lazy details → bounded recipient context (a core [[archcore-send]] metric).
- Requires a part registry and **practical one-time redemption** ([[practical-one-time-redemption]]).
- Slightly more API surface (per-part upload/download), but a clean future mapping to Durable-Streams events ([[roadmap]]).
- The server treats parts as opaque blobs; semantic mapping lives only in the encrypted `manifest` ([[zero-knowledge-backend]]).