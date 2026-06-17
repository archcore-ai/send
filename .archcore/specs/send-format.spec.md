---
title: "Send Format Specification (send.v1)"
status: accepted
---

## Purpose

Define the normative format of a **send** (`send.v1`): its parts, manifests, encoding pipeline, sizes, and identifiers. This is the contract shared by the client ([[skill-implementation]]) and stored opaquely by the server ([[backend-http-api]]). Keywords MUST/SHOULD/MAY per RFC 2119.

## Scope

Covers the local plaintext workdir, part kinds/ids, the encrypted private `manifest`, server-visible public metadata, the compress→encrypt pipeline, checksums, versioning, and the `compact` template. Excludes transport ([[backend-http-api]]) and skill UX ([[skill-contract]]). Caps live in [[size-limits]]; content rules in [[content-policy]].

## Normative Behavior

### Versioning
- Format version is `send.v1`, recorded in the private manifest as `"version":"send.v1"`.
- A loader MUST refuse an unknown major version with `UNSUPPORTED_VERSION`.

### Local plaintext workdir (pre-encryption)
```text
send-workdir/
├── manifest.json            # private manifest (semantic map)
├── compact.md               # required, load-by-default
├── evidence/
│   ├── errors.md
│   ├── decisions.md
│   └── files.md
└── details/
    ├── full-diff.patch
    └── test-output.log
```
Temp-file perms and deletion are governed by [[security-privacy]].

### Part model
Each part has:
- a **semantic id** (client-meaningful): `compact`, `evidence.errors`, `detail.full-diff`, …
- an **opaque transport id** the server sees: `manifest` (reserved) and `part_0001`, `part_0002`, … (zero-padded, assigned in creation order).
- `kind` ∈ `manifest | markdown | patch | log | json | text | binary`.
- `required` (bool), `load_by_default` (bool).

The transport id `manifest` is reserved for the encrypted private manifest — the only non-opaque id, and it reveals nothing beyond "this is the index". The server MUST NOT receive semantic ids, kinds, or `required` flags in clear ([[zero-knowledge-backend]]).

### Encoding pipeline (per part)
```text
plaintext bytes → gzip → age -r <ephemeral recipient> → ciphertext (.age)
```
- Compression MUST precede encryption; ciphertext MUST NOT be compressed.
- Each part is encrypted independently to the same per-send ephemeral recipient ([[e2ee-link-key-model]]).
- `ciphertext_sha256` MUST be computed over final ciphertext and sent on upload for integrity.

### Private manifest (encrypted → part `manifest`)
```json
{
  "version": "send.v1",
  "title": "Auth debugging handoff",
  "created_at": "2026-06-11T12:00:00Z",
  "source": { "agent": "claude-code", "repo": "api-service", "branch": "fix-auth-cache", "commit": "abc123" },
  "policy": {
    "raw_transcript_included": false,
    "secrets_included": false,
    "default_load": ["compact", "evidence.errors", "evidence.decisions"]
  },
  "parts": [
    { "id": "compact",          "transport_id": "part_0001", "kind": "markdown", "required": true,  "load_by_default": true,  "plaintext_size": 42000 },
    { "id": "evidence.errors",  "transport_id": "part_0002", "kind": "markdown", "required": true,  "load_by_default": true,  "plaintext_size": 18000 },
    { "id": "detail.full-diff", "transport_id": "part_0003", "kind": "patch",    "required": false, "load_by_default": false, "plaintext_size": 2100000 }
  ]
}
```
`source.*` fields are OPTIONAL and MUST be omittable for privacy.

### Public metadata (server-visible)
Returned by `GET /v1/sends/{id}` — no semantics:
```json
{
  "id": "snd_01J...", "status": "finalized", "one_time": true,
  "expires_at": "2026-06-12T12:00:00Z", "part_count": 3, "total_encrypted_size": 2133644,
  "parts": [
    { "part_id": "manifest",  "encrypted_size": 1200,    "sha256": "..." },
    { "part_id": "part_0001", "encrypted_size": 28412,   "sha256": "..." },
    { "part_id": "part_0003", "encrypted_size": 2104032, "sha256": "..." }
  ]
}
```

### `compact.md` template
```md
# Context: <title>
## Goal
## Current state
## Current hypothesis
## What was already tried
## Decisions made
## Relevant files
- `path/to/file.ts` — why relevant
## Current diff summary
## Important errors
## Open questions
## Suggested next steps
## Exclusions / redactions
- Raw transcript not included. Secrets/env not included. Full logs are optional details only.
```

### Loader behavior
- A loader MUST fetch and decrypt `manifest` first, then `compact` plus parts with `load_by_default: true`.
- A loader MUST NOT auto-fetch `load_by_default: false` parts; those load only via `--load-detail` ([[skill-contract]]).

## Constraints
- Per-part and per-send size caps are normative in [[size-limits]]; enforced client-side before upload AND server-side on upload.
- Default exclusions and the never-include list are in [[content-policy]].

## Invariants
- **INV-1** — The server reconstructs nothing semantic from public metadata alone.
- **INV-2** — `manifest` is always present and required.
- **INV-3** — Every non-`manifest` part has a unique `part_NNNN` transport id and a unique semantic id.
- **INV-4** — `ciphertext_sha256` in public metadata matches stored bytes.

## Error Handling
- Unknown version → `UNSUPPORTED_VERSION`; missing `manifest` → `DECRYPTION_FAILED`; hash mismatch → `INTEGRITY_FAILED`. See [[error-catalog]].

## Conformance
A conforming implementation:
- [ ] gzips then age-encrypts each part independently;
- [ ] emits only opaque transport ids + sizes + sha256 to the server;
- [ ] keeps all semantics inside the encrypted `manifest`;
- [ ] loads compact-first and never auto-loads details;
- [ ] enforces [[size-limits]] and [[content-policy]] before upload;
- [ ] rejects unknown `send.vN`.