---
title: "Self-host-first hosting; RF availability posture"
status: accepted
---

## Context

The service must be usable globally **and from Russia**. Cloudflare (Workers + R2 + Durable Objects) is the best technical/cost fit, but Cloudflare officially reports Russian-ISP throttling (down to ~16 KB per connection), which makes it unsuitable as the **sole** endpoint where RF reach matters.

## Decision

**Self-host-first.** The reference deployment is **VPS + SQLite + object storage**, default **Hetzner** (EU, transparent, cheap, no Cloudflare dependency). The design stays storage- and provider-agnostic ([[storage-abstraction]]) so an operator can pick the provider that fits their audience. Cloudflare MUST NOT be the only endpoint where RF reach matters; if used, pair it with a non-Cloudflare fallback.

Provider guidance (full notes in [[self-host-deploy]]):

| Need | Pick |
|---|---|
| Default, transparent, cheap | Hetzner VPS + Object Storage |
| Friendly DX backup | DigitalOcean Droplet + Spaces |
| Global deploy DX | Fly.io + Tigris (no egress fees) |
| RF + world is the blocker | Gcore |
| RF-first / RF-primary | Yandex Cloud / Selectel |

## Alternatives

- **Cloudflare-only**: best architecture/price, unacceptable RF availability risk. Rejected as sole endpoint.
- **Supabase / BaaS-first**: SQL transparency but egress-dominated cost and a heavier baseline. Deferred.
- **Gcore-first by default**: more expensive and platform-specific; kept as an escalation, not the default.

## Consequences

- Operators run a tiny VPS service; ops guidance in [[self-host-deploy]].
- An **availability test plan** (RF + global) gates any hosted default ([[self-host-deploy]]).
- Provider choice is a **config/deploy** concern, not a code concern — protected by [[storage-abstraction]].