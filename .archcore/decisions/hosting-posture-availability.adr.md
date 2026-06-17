---
title: "Self-host-first hosting & availability posture"
status: accepted
---

## Context

The service must be usable by a broad, possibly global audience. A fully managed edge stack (serverless compute + managed object storage + edge KV) is often the best raw technical and cost fit, but managed edges vary in network reachability, per-connection limits, and latency across regions and networks. Relying on a single managed edge as the *only* endpoint is therefore risky whenever broad reach matters: if that edge is slow or unreachable from part of your audience, the whole service is.

## Decision

**Self-host-first.** The reference deployment is **VPS + SQLite + object storage** — a single small box an operator fully controls: transparent, cheap, and dependency-light. The design stays storage- and provider-agnostic ([[storage-abstraction]]) so an operator can pick whatever is reachable and performant for *their* audience. A managed edge MUST NOT be the only endpoint where broad reach matters; if used, pair it with a direct (non-edge) fallback.

Provider choice is criteria-based, not prescriptive:

| Need | Pick |
|---|---|
| Cheap, transparent default | any mainstream VPS + S3-compatible object storage |
| Friendly DX | a managed droplet/app platform with attached object storage |
| Global deploy DX | a platform with multi-region deploy and no egress fees |
| Broad reach is the blocker | a provider with good connectivity to your audience's networks |

Validate the choice empirically with the availability test plan in [[self-host-deploy]] rather than assuming.

## Alternatives

- **Managed-edge-only** — best architecture/price, but single-endpoint reachability risk for part of the audience. Rejected as the *sole* endpoint; acceptable when paired with a direct fallback.
- **BaaS-first** — SQL transparency but egress-dominated cost and a heavier baseline. Deferred.

## Consequences

- Operators run a tiny VPS service; ops guidance in [[self-host-deploy]].
- An **availability & latency test plan** gates any hosted default — measure from the regions and networks your audience actually uses ([[self-host-deploy]]).
- Provider choice is a **config/deploy** concern, not a code concern — protected by [[storage-abstraction]].