---
title: "Archcore Send — Architecture Overview"
status: accepted
---

## Overview

Archcore Send moves **structured working context** between AI coding agents as an end-to-end-encrypted **send**. The system has two parts that share nothing but an HTTP API carrying ciphertext:

- **Send skill (client)** — a portable Agent Skill (`SKILL.md` + `send.sh` / `send.ps1`) that builds, encrypts (`age`), uploads, downloads, and decrypts sends. → [[skill-implementation]]
- **Send server (backend)** — a Go single-binary **ciphertext rendezvous**: store/serve encrypted parts, enforce TTL + one-time redemption + limits, run GC. → [[server-implementation]]

**Core invariant:** plaintext and the decryption key exist only on the sender's and recipient's machines. The server holds ciphertext + minimal operational metadata. See [[zero-knowledge-backend]] and [[e2ee-link-key-model]].

This document is the architecture hub — component model, trust boundary, the two data flows, the send lifecycle, and the glossary. Normative detail lives in the specs and rules it links.

## Content

### Component model

```mermaid
flowchart LR
  subgraph Sender["Sender machine (trusted, sees plaintext)"]
    SA[Sender agent] --> SK["/send skill<br/>send.sh / send.ps1"]
    SK --> AGEs[age]
  end
  subgraph Server["Send server (untrusted for plaintext)"]
    API["HTTP API<br/>/v1/sends"] --> SS["SendStore"]
    SS --> META[("State DB<br/>SQLite / Postgres")]
    SS --> BLOB[("Blob store<br/>filesystem / S3")]
    GC["GC worker"] --> SS
  end
  subgraph Recipient["Recipient machine (trusted, sees plaintext)"]
    RK["/send --load skill"] --> AGEr[age]
    RK --> RA[Recipient agent]
  end
  SK -- "ciphertext parts + metadata" --> API
  API -- "ciphertext parts" --> RK
  SK -. "link with #agekey (out-of-band)" .-> RK
```

### Trust boundary

The boundary is the network edge. Everything semantic happens locally; only ciphertext crosses.

```mermaid
flowchart TB
  K{{"#agekey fragment (decryption key)"}}
  subgraph Local["LOCAL — plaintext + key"]
    direction LR
    P[plaintext parts] --> Z[gzip] --> E[age encrypt]
    D[age decrypt] --> G[gunzip] --> I[import to agent]
  end
  subgraph Remote["REMOTE — ciphertext only"]
    C[("encrypted parts")]
    M[("operational metadata")]
  end
  E -- upload ciphertext --> C
  C -- download ciphertext --> D
  K -. never crosses .-> Remote
```

### Data flow — `/send`

```mermaid
sequenceDiagram
  participant A as Sender agent
  participant S as send.sh
  participant G as age
  participant B as Server
  A->>S: build workdir (manifest, compact, evidence, details)
  S->>S: secret scan + size check + preview (confirm)
  S->>G: age-keygen ephemeral identity
  loop each part
    S->>S: gzip part
    S->>G: age -r <recipient> (encrypt)
  end
  S->>B: POST /v1/sends (part ids, sizes, sha256)
  B-->>S: id, upload URLs, expires_at
  loop each part
    S->>B: PUT /v1/sends/{id}/parts/{part}
  end
  S->>B: POST /v1/sends/{id}/finalize
  B-->>S: public_url
  S->>A: public_url + "#agekey=..." (appended locally)
```

### Data flow — `/send --load`

```mermaid
sequenceDiagram
  participant U as Recipient agent
  participant S as send --load
  participant B as Server
  participant G as age
  U->>S: /send --load <url>
  S->>S: split url -> base + #agekey (local only)
  S->>B: POST /v1/sends/{id}/redeem
  B-->>S: redeem_token (10-min grant) + part ids
  S->>B: GET manifest, compact, required evidence
  B-->>S: ciphertext parts
  S->>G: age --decrypt -i <ephemeral key>
  S->>U: compact + required evidence (+ list optional details)
  Note over U,S: optional details lazy-loaded on demand within the grant window
```

### Send lifecycle

```mermaid
stateDiagram-v2
  [*] --> creating: POST /v1/sends
  creating --> finalized: finalize (all parts uploaded)
  creating --> deleted: GC unfinished (> 1h)
  finalized --> consumed: first redeem (one-time)
  finalized --> expired: TTL passes
  consumed --> expired: TTL passes
  consumed --> deleted: grant expires + GC
  expired --> deleted: GC
```

### Glossary

| Term | Meaning |
|---|---|
| **send** | The encrypted multipart handoff artifact. ID prefix `snd_`. |
| **part** | An independently gzipped+encrypted unit of a send. |
| **manifest** | Reserved part `manifest`: the encrypted private index mapping opaque part ids → semantic ids/kinds. |
| **public metadata** | The non-secret lifecycle data the server sees (part count, encrypted sizes, sha256, timestamps). |
| **compact** | The required, load-by-default working context (≤ ~8k tokens). |
| **evidence** | Small supporting facts (errors, decisions, file excerpts), loaded by default if small. |
| **detail** | Large optional parts (full diff, logs), never auto-loaded. |
| **redeem** | The single one-time consumption of a send's public link. |
| **grant** | The short-lived (10-min) download session opened by redeem. |
| **link-key mode** | Per-send ephemeral `age` identity; private key carried in the URL fragment. |
| **fragment** | The `#agekey=...` URL part; parsed locally, never sent to the server. |
| **SendStore** | The Go storage interface (state + blob) the server is built around. |

### Where to look

- Contracts: [[send-format]], [[backend-http-api]], [[skill-contract]], [[error-catalog]]
- Rules: [[security-privacy]], [[content-policy]], [[size-limits]]
- Build: [[skill-implementation]], [[server-implementation]], [[self-host-deploy]]
- Conventions & packaging: [[repo-layout-and-skill-packaging]], [[go-server-conventions]], [[skill-scripting-conventions]], [[testing]]
- Risk: [[threat-model]]

## Examples

Condensed end-to-end handoff:

```text
# Sender
User: /send --ttl 24h
Agent: builds compact context (goal, state, hypothesis, files, decisions,
       open questions, next steps) + selected evidence. Shows preview.
Preview: compact 38 KB · evidence 64 KB · optional full-diff 1.9 MB ·
         skipped pnpm-lock.yaml, coverage/, .env · server receives ciphertext only.
Proceed? y
=> https://send.example.com/s/snd_01J...#agekey=AGE-SECRET-KEY-...  (expires 24h, one-time)

# Recipient
User: /send --load https://send.example.com/s/snd_01J...#agekey=AGE-SECRET-KEY-...
Agent: Loaded "Staging auth failure handoff".
       Goal, relevant files (src/auth/session.ts, src/middleware/jwt.ts),
       open questions imported. Available details: detail.full-diff, detail.test-output.
       I will not modify files until you confirm.
```