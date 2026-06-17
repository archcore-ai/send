---
title: "Send Threat Model"
status: accepted
---

## Overview

Threat model for Archcore Send v0: who sees what, what is protected, what explicitly is not, and the residual risks. Pairs with [[security-privacy]] (rules) and [[zero-knowledge-backend]] (posture).

## Content

### Actors
| Actor | Sees plaintext? |
|---|---|
| Sender user / agent | yes (creates it) |
| Local send scripts | yes (before encrypt / after decrypt) |
| `age` binary (local) | yes (performs crypto) |
| Send server / operator | **no** — ciphertext + metadata only |
| Network observer | no — HTTPS; maybe coarse metadata |
| Recipient user / agent | yes (after local decrypt) |

### Assets
source excerpts · current diff · architecture notes · error logs / stack traces · internal references · private task context · secrets accidentally captured · the `age` key in the URL fragment · transient plaintext on disk.

### Protect against
```text
operator reading content · DB / object-storage leak exposing plaintext · server logs containing context
replay of expired/consumed sends · id guessing · accidental oversized plaintext upload · fragment leaking to backend
```

### Does NOT protect against
```text
sending/receiving agent seeing plaintext · the LLM provider processing plaintext
recipient copying or forwarding decrypted content · recipient forwarding the full URL
malware / compromised age / hostile shell on a local machine · terminal or browser history storing the full URL
```

### Precise privacy statement
> Archcore Send protects your context **from the Send backend**. Encryption and decryption happen locally; the backend stores only ciphertext and operational metadata. The AI environments you intentionally use to create or load a send still see plaintext.

### Attack scenarios → mitigations
| Scenario | Mitigation |
|---|---|
| Backend DB/blob leak | E2EE; only ciphertext + metadata at rest ([[e2ee-link-key-model]], [[zero-knowledge-backend]]) |
| Operator inspects a send | cannot decrypt; no key server-side |
| Fragment leaks via remote tool | local-only parse; no remote-MCP load ([[security-privacy]] R2) |
| Link replay after use | practical one-time + TTL; atomic redeem ([[practical-one-time-redemption]]) |
| Id guessing | unguessable `snd_` ids; rate limits |
| Secret baked into a send | client secret scan pre-encrypt ([[content-policy]]) |
| Oversized plaintext to recipient | compact-first + caps ([[size-limits]]) |
| Metadata correlation (size/time/freq) | minimize / hash / short retention ([[security-privacy]] R6) |
| Token theft from DB | tokens stored hashed; short grant window |

### Residual risks (accepted, documented)
- Anyone with the **full URL incl. fragment** can decrypt → "treat the link like a secret"; mitigated later by recipient-key mode ([[roadmap]]).
- The backend learns coarse metadata (sizes, timing, frequency).
- Endpoints to local plaintext (agents, disk) are out of scope by design.

## Examples
- **Leak drill** — dump the DB + blob store: an attacker holds `age` ciphertext + sizes/timestamps, no readable context. Pass.
- **Forwarding drill** — a user pastes the full URL into an untrusted web tool: the key may leak. This is a documented non-guarantee, not a backend break.