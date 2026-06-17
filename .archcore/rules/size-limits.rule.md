---
title: "Size Limits & Quotas"
status: accepted
---

## Rule

Canonical size/quota limits for Archcore Send. Other docs reference these numbers; do not redefine them elsewhere.

### Content caps
| Scope | Soft cap | Hard cap |
|---|---:|---:|
| `compact` (plaintext) | 30 KB | 50 KB |
| required evidence (plaintext, total) | 300 KB | 800 KB |
| single `detail.*` (plaintext) | 10 MB | 50 MB |
| total send (encrypted) | 10 MB | 25 MB |

### Lifecycle limits
| Setting | Default | Max |
|---|---|---|
| TTL | 24h | 7d |
| Redeem grant window | 10 min | fixed (v0) |
| Unfinished-upload GC | 1h | — |
| Part count per send | — | configurable (e.g. 64) |

### Behavior
- **Soft cap exceeded** → show preview and require explicit confirmation (`--yes` or interactive).
- **Hard cap exceeded** → reject with `SEND_TOO_LARGE` (exit 5) unless `--include-large` (single-detail / total only). `compact` and evidence hard caps are **not** overridable — split overflow into `detail.*`.
- Enforced **client-side** before upload AND **server-side** on create/upload ([[backend-http-api]] → `413`).

### Rate limits (server)
- Anonymous mode: strict per-IP create/upload limits → `429 RATE_LIMITED`.
- Team / self-host mode: configurable.

## Rationale

The dominant risk is not upload bandwidth but **flooding the recipient agent's context window** with plaintext ([[multipart-send-format]]). Compact-first caps keep default-loaded context near ~8k tokens ([[archcore-send]] metrics). Total/TTL/rate caps also bound abuse of a ciphertext-only service as generic file hosting ([[threat-model]]).

## Examples

**Good** — a 2.1 MB diff goes in `detail.full-diff` (optional), not inlined; compact stays 38 KB.

**Bad** — a 60 MB `detail.full-diff` without `--include-large` (rejected); a 120 KB compact (hard-cap violation — must be split).

## Enforcement

- Client computes sizes during workdir assembly and blocks per the behavior table.
- Server validates declared sizes on `POST /v1/sends` and actual bytes on `PUT …/parts`.
- Config keys (`SEND_MAX_TTL`, `SEND_MAX_TOTAL_BYTES`, `SEND_MAX_PART_BYTES`, `SEND_RATE_*`) documented in [[self-host-deploy]].