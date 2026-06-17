---
title: "Send Error Catalog (client + server)"
status: accepted
---

## Purpose

Single source of truth for error codes across the client scripts and the server — status, message intent, and remediation — so the two never diverge. Keywords per RFC 2119.

## Scope

Client-side (script) error objects, server-side HTTP errors, the mapping between them, and error-logging rules. Referenced by [[cli-contract]], [[backend-http-api]], [[security-privacy]], [[content-policy]].

## Normative Behavior

### Client error objects
Shape: `{ "ok":false, "error_code":"…", "message":"…", "remediation":"…" }`, with an exit code per [[cli-contract]].

| error_code | When | exit | Remediation |
|---|---|---|---|
| `AGE_NOT_FOUND` | `age` missing | 3 | macOS `brew install age`; Windows `winget install FiloSottile.age`; Linux distro pkg |
| `GZIP_NOT_FOUND` | no gzip and no fallback | 3 | install gzip, or use PowerShell compression |
| `CURL_NOT_FOUND` | no HTTP client | 3 | install curl, or use PowerShell `Invoke-WebRequest` |
| `SECRET_DETECTED` | high-confidence secret pre-encrypt | 4 | redact, or pass `--allow-secrets` |
| `SEND_TOO_LARGE` | exceeds hard cap | 5 | drop details, or pass `--include-large` |
| `FRAGMENT_MISSING` | url has no `#agekey` | 7 | re-copy the full link including the fragment |
| `DECRYPTION_FAILED` | `age` decrypt failed | 7 | wrong/truncated fragment or corrupt download |
| `INTEGRITY_FAILED` | sha256 mismatch on download | 7 | re-download; the link may be corrupted |
| `UNSUPPORTED_VERSION` | unknown `send.vN` | 1 | update the skill |
| `SERVER_UNREACHABLE` | network/DNS/TLS failure | 6 | check `--server` / connectivity ([[self-host-deploy]]) |

### Server HTTP errors
Shape: `{ "error_code":"…", "message":"…" }` + HTTP status.

| status | error_code | When |
|---|---|---|
| 400 | `BAD_REQUEST` / `INCOMPLETE` | invalid metadata / finalize before all parts uploaded |
| 403 | `INVALID_REDEEM` | missing / expired / invalid grant token |
| 404 | `SEND_NOT_FOUND` | unknown or deleted id |
| 409 | `SEND_FINALIZED` | mutate after finalize / upload to a non-`creating` send |
| 410 | `SEND_EXPIRED` | TTL has passed |
| 410 | `SEND_ALREADY_REDEEMED` | one-time link already consumed |
| 413 | `PAYLOAD_TOO_LARGE` | part or body over cap |
| 422 | `INTEGRITY_FAILED` | declared sha256 ≠ received bytes |
| 429 | `RATE_LIMITED` | abuse controls tripped |
| 500 | `STORAGE_ERROR` | backend failure |

### Error logging
- The server MUST log only `error_code`, request id, send id, part id, and status — never bodies, fragments, or tokens ([[security-privacy]]).
- Scripts MUST log only secret **counts/types**, never values ([[content-policy]]).

## Constraints
- Messages MUST be actionable and contain no sensitive data.
- Decrypt failures are client-side and MUST NOT be auto-reported to the server unless the user opts in ([[security-privacy]]).

## Invariants
- **INV-1** — Every client failure yields exactly one `error` object plus the matching exit code.
- **INV-2** — No error payload contains plaintext, fragments, or tokens.

## Conformance
- [ ] every code is reachable in tests;
- [ ] messages carry no secrets;
- [ ] client and server codes don't semantically collide;
- [ ] remediation text resolves the common cause.