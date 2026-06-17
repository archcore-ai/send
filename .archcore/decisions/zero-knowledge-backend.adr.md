---
title: "Backend is a dumb ciphertext rendezvous (zero-knowledge)"
status: accepted
---

## Context

The product promise is that a **backend compromise exposes ciphertext + metadata, never plaintext context**. Anything the server can do with plaintext is something an attacker (or a subpoena) can do.

## Decision

The server is a **dumb ciphertext rendezvous**. It:

- accepts, stores, and serves **encrypted bytes**;
- enforces TTL, one-time redemption, size caps, rate limits;
- runs garbage collection.

It MUST NOT decrypt, summarize, redact, semantically index, parse diffs/logs, scan secrets, compress, merge, or store keys. Part **semantics** live only in the encrypted `manifest`. The server keeps **minimal public metadata**: opaque part ids (`part_0001`), encrypted sizes, ciphertext `sha256`, timestamps, lifecycle status.

## Alternatives

- **Smart backend** (server-side preview/redaction/search): impossible under zero-knowledge and a privacy liability. Rejected.
- **Plaintext titles / semantic part names in metadata**: leaks context shape. Rejected — public part ids are generic; the `manifest` maps them.

## Consequences

- All semantic work happens client-side: sender **pre-encryption**, recipient **post-decryption**.
- The server stays small, auditable, cheap, and portable across storage backends ([[storage-abstraction]]).
- Residual **metadata leakage** (size, timing, frequency) is real → mitigations mandated in [[security-privacy]] and analyzed in [[threat-model]].
- Secret scanning is therefore a **client** responsibility before encryption ([[content-policy]]).