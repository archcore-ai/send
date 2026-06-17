---
title: "Archcore Send — Product Requirements (v0)"
status: accepted
---

## Vision

Archcore Send is a privacy-first **context-handoff primitive** for AI coding agents. A developer working in one agent session packages the *useful* slice of working context into an end-to-end-encrypted **send** and shares one short-lived link. Another developer — or another agent session — loads it and resumes from a compact, structured starting point instead of re-deriving everything from a raw transcript.

The defining constraint is auditability of the trust boundary: a readable client skill, `age` as the only crypto dependency, and a deliberately minimal server that stores **ciphertext only**. A user should be able to understand the entire system in one sitting.

> **Terminology.** The encrypted artifact is a **send** (noun: "a send", "the send", "sends"). The product/system is **Archcore Send**. The single command is **`/send`**. (Earlier `__export__/` drafts called the artifact a "capsule"; that term is retired.)

## Problem

AI coding sessions accumulate context that is expensive to reconstruct: the task, what was tried, the current hypothesis, relevant files, the working diff, key errors, decisions already made, open questions, and next steps. When work must move to another person or another agent session, every existing option is bad:

- manually re-summarize the chat;
- paste a long transcript (noisy, leaks secrets, blows up the recipient's context window);
- send screenshots or raw logs;
- re-run the whole investigation from scratch.

There is no lightweight, privacy-preserving way to move *structured working context* between agents/people. Existing tools are either heavyweight collaboration platforms or plaintext pastebins.

## Goals & Metrics

**Product goals (v0)**

1. One command (`/send`) packages current context into an encrypted send and returns a shareable link.
2. One command (`/send --load <url>`) imports a *compact-first* working context into the receiving agent.
3. The server never receives plaintext or the decryption key.
4. The whole client is auditable: readable scripts, no bundled binary, no Python/Node runtime.
5. Anyone can self-host the server with a single container.

**Non-goals (v0)** — explicitly out of scope:

- collaborative workspace / live shared session / CRDT;
- permanent team memory or semantic search;
- full transcript recording;
- recipient public-key directories;
- background daemon or always-on MCP service that receives plaintext;
- web dashboard.

**Success metrics**

| Metric | Target |
|---|---|
| Time-to-handoff (sender invoke → link) | < 15 s for a normal send |
| Recipient context injected by default | ≤ ~8k tokens (compact + required evidence) |
| Plaintext bytes reaching the server | 0 |
| Decryption-key bytes reaching the server | 0 |
| Client runtime deps beyond `age` | 0 required (curl/gzip optional/standard) |
| Self-host cold start (clone → running) | < 10 min |

## Requirements

Requirements are normative for v0. Each maps to a spec/rule/guide that defines it precisely.

**Functional (FR)**

- **FR-1** — `/send` builds a structured send from the current session and uploads encrypted parts. → [[send-format]], [[skill-contract]]
- **FR-2** — `/send --load <url>` loads manifest + compact + required evidence by default; lists optional details without injecting them. → [[skill-contract]]
- **FR-3** — `/send --load-detail <url> <part-id>` lazy-loads a single optional part. → [[skill-contract]]
- **FR-4** — `/send --doctor` diagnoses the environment (age, curl/gzip, connectivity). → [[skill-contract]]
- **FR-5** — Sends are multipart; each part is compressed then encrypted independently. → [[multipart-send-format]], [[send-format]]
- **FR-6** — Sends expire (TTL) and are one-time by default (practical one-time redemption). → [[practical-one-time-redemption]], [[backend-http-api]]

**Security (SR)** — full set in [[security-privacy]] and [[threat-model]].

- **SR-1** — Encryption/decryption happen locally; the server stores ciphertext + operational metadata only. → [[zero-knowledge-backend]]
- **SR-2** — The decryption key lives in the URL fragment, is parsed locally, and is never sent to the server. → [[e2ee-link-key-model]]
- **SR-3** — High-confidence secrets are blocked before encryption unless explicitly overridden. → [[content-policy]]
- **SR-4** — Temporary plaintext is created with restrictive permissions and deleted on success/failure. → [[security-privacy]]
- **SR-5** — No bundled binary, no auto-install, no remote code loading. → [[age-dependency-no-bundled-binary]]

**UX**

- **UX-1** — `/send` shows a preview (sizes; included/optional/skipped; "server receives ciphertext only") and requires confirmation unless `--yes`.
- **UX-2** — `/send --load` summarizes what was imported and never modifies project files without confirmation.
- **UX-3** — Errors are actionable with remediation. → [[error-catalog]]

**Portability**

- **PORT-1** — macOS/Linux via `send.sh`; Windows via `send.ps1`.
- **PORT-2** — Works across agents that read `SKILL.md` (Claude Code, OpenCode, Codex/Gemini-style) with an optional command shim.

**Self-host**

- **SELF-1** — Single Go binary / container; SQLite + filesystem default; S3-compatible object storage optional. → [[storage-abstraction]], [[self-host-deploy]]
- **SELF-2** — Configurable TTL caps, size caps, rate limits. → [[size-limits]]

## Personas

- **Sender developer** — finishes/pauses a debugging or feature session and wants a clean handoff.
- **Sender agent** — builds the structured send from session context (sees plaintext by definition).
- **Recipient developer / agent** — resumes work from compact context.
- **Self-host operator** — runs the server for a team; cares about deployment simplicity, retention, abuse controls.

## Scope: the two parts

Archcore Send v0 is exactly two deliverables:

1. **The Send skill (client)** — `SKILL.md` + `send.sh`/`send.ps1` + references. Builds, encrypts, uploads, downloads, decrypts, and imports. → [[skill-implementation]]
2. **The Send server (backend)** — Go single binary, ciphertext rendezvous, lifecycle + redemption + abuse controls. → [[server-implementation]], [[self-host-deploy]]

They communicate only over the HTTP API in [[backend-http-api]], exchanging ciphertext and operational metadata.

## Locked decisions (v0)

| Axis | Decision | Record |
|---|---|---|
| Crypto | `age`, ephemeral identity per send, key in URL fragment | [[e2ee-link-key-model]] |
| Format | multipart, compress-then-encrypt per part | [[multipart-send-format]] |
| Redemption | practical one-time, 10-min grant | [[practical-one-time-redemption]] |
| Backend posture | zero-knowledge, ciphertext only | [[zero-knowledge-backend]] |
| Storage | `SendStore` abstraction; SQLite + FS first | [[storage-abstraction]] |
| Server stack | Go, single static binary | [[go-single-binary-server]] |
| Dependency | `age` only; no bundled binary | [[age-dependency-no-bundled-binary]] |
| Hosting | self-host-first; Hetzner default; avoid Cloudflare-sole-endpoint (RF) | [[hosting-posture-rf-availability]] |
| Naming | `/send` command; "send" artifact | this PRD |

## Acceptance criteria (v0 done)

The full conformance checklists live in the specs; v0 is "done" when:

- send → link → load round-trips across two machines with `age` installed;
- optional details are never auto-injected into the recipient agent;
- the server, inspected at rest, holds only ciphertext + metadata (no plaintext, no fragment key);
- one-time redemption and TTL expiry are enforced atomically;
- the secret scanner blocks a planted AWS key by default;
- `/send --doctor` explains a missing-`age` environment clearly;
- a fresh operator self-hosts via container in < 10 min.