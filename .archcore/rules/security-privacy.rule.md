---
title: "Security & Privacy Rules"
status: accepted
---

## Rule

Non-negotiable security/privacy rules for Archcore Send. They bind both parts.

- **R1 — Local crypto only.** Encryption/decryption MUST happen on sender/recipient machines. The server receives only ciphertext + operational metadata ([[zero-knowledge-backend]]).
- **R2 — Fragment never leaves the client.** The `#agekey=…` (or `#k=…`) fragment MUST be parsed locally and MUST NOT appear in any HTTP request, redirect, log, telemetry, or argv handed to a remote tool. This forbids a remote-MCP "load" path; load MUST be local script logic ([[e2ee-link-key-model]]).
- **R3 — Temp plaintext hygiene.** Plaintext/identity temp files MUST live in the OS temp dir (never under the project root), be created `0600`, be deleted on success AND failure (trap/finally), never be printed, and never be produced under shell tracing (`set -x` off around secrets/plaintext).
- **R4 — Key not persisted.** The ephemeral `age` identity MUST NOT be written to persistent disk beyond the short-lived temp file needed to decrypt, removed immediately after.
- **R5 — Server logging.** The server MUST log only: request id, send id, part id, status, encrypted size, duration, coarse `error_code`. NEVER: bodies, full URLs, fragments, `Authorization`/redeem/management tokens, referrers ([[error-catalog]]).
- **R6 — Metadata minimization.** Hash/truncate IP and user-agent; short retention; generic public part ids; no plaintext titles/semantic names; no recipient identities in link-key mode.
- **R7 — Token handling.** Redeem/management tokens MUST be random, stored **hashed**, compared in constant time, expired promptly.
- **R8 — Honest claims.** Ship the precise statement below; never overclaim.

### Guarantees (MAY claim)
- The server does not receive plaintext send content.
- The server does not receive the `age` key / URL fragment.
- Encryption/decryption are local; the server stores ciphertext + operational metadata only.

### Non-guarantees (MUST NOT claim)
- That the AI provider can't see the context (the agents you use see plaintext by design).
- That sender/recipient can't copy plaintext, or that one-time prevents screenshots.
- That the server can redact encrypted data.
- That a full link (with fragment) is safe to paste into untrusted remote tools.

## Rationale

The product's value is a small, auditable trust boundary ([[archcore-send]]). Each rule keeps the backend — its operator, logs, or a leak — from seeing plaintext, and keeps honest the one real residual risk: **anyone with the full URL can decrypt** ([[threat-model]]).

## Examples

**Good**
```sh
base="${url%%#*}"; frag="${url#*#}"; key="${frag#agekey=}"   # split locally
umask 077; idfile="$(mktemp)"                                # 0600
trap 'rm -f "$idfile" "$tmp"/*' EXIT INT TERM                # always clean up
curl -fsS -X POST "$SEND_SERVER_URL/v1/sends/$id/redeem"     # base only; no fragment
```

**Bad**
```sh
curl "$url"                       # leaks #agekey to server/logs
echo "key=$key"                   # prints secret
set -x                            # traces plaintext/secrets
cp "$plain" ./tmp/                # plaintext under project root
```

## Enforcement

- Review gate on any code touching URLs, temp files, or logging.
- Tests: (a) request-capture asserts no fragment in any outbound request; (b) server log-scrubbing test asserts no body/token/fragment; (c) at-rest test asserts blobs are valid `age` ciphertext and metadata holds no plaintext titles.
- `/send --doctor` and CI reject configurations that would persist plaintext.