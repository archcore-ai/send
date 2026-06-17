---
title: "Practical one-time redemption (grant window)"
status: accepted
---

## Context

One-time links are a core privacy promise, but **strict** "first GET consumes forever" breaks multipart/staged loading and burns the link on a network blip mid-download.

## Decision

Adopt **practical one-time**:

- The first `POST /v1/sends/{id}/redeem` **atomically** consumes the public link and opens a **short-lived download grant** (default **10 minutes**).
- During the grant, `manifest`, `compact`, and selected `detail.*` parts may be fetched with a hashed `redeem_token`.
- "One-time" means **one redemption session**, not one HTTP response.

User-facing wording: *"The link opens once. After opening, parts stay available for 10 minutes."*

## Alternatives

- **Strict one-time** (consume on first part GET): simple and strong-sounding, but hostile to staged loads and fragile under network loss. Rejected as the default.

## Consequences

- Requires a `redeem_grants` table and **hashed** redeem tokens ([[backend-http-api]], [[security-privacy]]).
- Atomic redeem implemented as a conditional `UPDATE ... WHERE consumed_at IS NULL` ([[server-implementation]]).
- Interrupted downloads recover within the grant window.
- The grant duration is a tunable in [[size-limits]].