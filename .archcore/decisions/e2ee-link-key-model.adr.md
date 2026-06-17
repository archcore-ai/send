---
title: "End-to-end encryption via age link-key mode"
status: accepted
---

## Context

Sends carry sensitive working context — code excerpts, diffs, logs, internal notes. The server must never be trusted with plaintext, and onboarding must work cross-agent with **zero prior key setup**. The crypto surface must be auditable by a non-cryptographer.

## Decision

Use **`age`** in **link-key mode**:

- For each send, the client generates an **ephemeral `age` identity** (`age-keygen`).
- Every part is encrypted to the derived recipient (`age -r <recipient>`).
- The **private identity is placed in the URL fragment**: `https://send.example.com/s/snd_...#agekey=AGE-SECRET-KEY-...`.
- The recipient script parses the fragment **locally**, writes a temp identity file (`0600`), decrypts, and deletes it.

Rules:

- **Compress then encrypt** per part (`gzip → age`). Never compress ciphertext.
- Default fragment encoding `#agekey=<age-secret>`; fall back to `#k=<base64url>` only if shell quoting/length forces it (see [[cli-contract]]).
- The server only ever sees `/{id}` paths and ciphertext.

## Alternatives

- **Recipient public-key mode** (`--to bob`): stronger (link alone cannot decrypt) but needs key management/identity setup. Deferred to v1 ([[roadmap]]).
- **Password-derived key (scrypt)**: weaker (user-chosen entropy) and adds UX friction. Rejected for v0.
- **Server-side key escrow / KMS**: breaks zero-knowledge. Rejected.
- **WebCrypto / AES-GCM via Node**: forces a JS runtime and is less auditable than a narrow purpose-built tool. Rejected ([[age-dependency-no-bundled-binary]]).

## Consequences

- Anyone with the **full URL including the fragment** can decrypt → "treat the full link like a secret." Stated honestly in [[threat-model]].
- Scripts MUST parse the fragment locally and never transmit it ([[security-privacy]], SR-2). This forbids a remote MCP "load" path.
- The entire crypto surface is `age` + `gzip` — easy to audit; satisfies [[zero-knowledge-backend]].
- Migrating to recipient-key mode later changes only key handling, not the [[send-format]] or [[backend-http-api]].