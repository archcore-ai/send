---
title: "Send CLI & Skill Contract"
status: accepted
---

## Purpose

Define the `/send` command surface, the `send.sh` / `send.ps1` script contract, structured JSON I/O, and exit codes — the boundary between the **agent** (builds content) and the **scripts** (crypto + transport). Keywords per RFC 2119.

## Scope

Command modes and flags, script subcommands, JSON output schemas, exit codes, env/config, and the fragment-parsing rule. Excludes the format ([[send-format]]) and HTTP ([[backend-http-api]]).

## Normative Behavior

### Command surface (agent-facing)
```text
/send                                   # create + upload (default)
/send --load <url>                      # load manifest + compact + required evidence
/send --load-detail <url> <part-id>     # lazy-load one optional part
/send --doctor                          # environment diagnostics
/send --inspect                         # preview what WOULD be sent (no upload)
```
Flags: `--ttl <dur>` (default 24h, max 7d), `--one-time` / `--no-one-time` (default one-time), `--yes` (skip confirm), `--dry-run`, `--allow-secrets`, `--include-large`, `--server <url>`.

### Script subcommands (script-facing)
```text
send.sh doctor
send.sh send <workdir> --ttl 24h --one-time [--yes] [--allow-secrets] [--include-large] [--server URL]
send.sh load <url> [--server URL]
send.sh load-detail <url> <part-id>
send.sh inspect <workdir>
```
PowerShell mirrors with `-Ttl`, `-OneTime`, `-Yes`, `-AllowSecrets`, `-IncludeLarge`, `-Server`.

### Agent ⇄ script boundary
- The **agent** assembles the workdir (`compact`, `evidence/*`, `details/*`, `manifest.json`) from session context, applies [[content-policy]], and writes plaintext files.
- The **script** performs crypto, transport, secret-scan enforcement, size checks, temp-file hygiene, and prints JSON. The script MUST NOT semantically summarize, read arbitrary repo files on its own, or mutate project files ([[age-dependency-no-bundled-binary]]).

### Fragment rule
For `load` / `load-detail`, the script MUST split `<url>` into base + `#agekey=…` **locally** and issue backend requests only to the base path. The fragment MUST NOT appear in any network request, log line, or argv visible to the server ([[security-privacy]], SR-2).

### Structured output (stdout = one JSON object; human text → stderr)
`send`:
```json
{ "ok":true, "url":"https://…/s/snd_…#agekey=AGE-SECRET-KEY-…", "expires_at":"…",
  "one_time":true, "included":["manifest","compact","evidence.errors"],
  "optional_parts":["detail.full-diff"] }
```
`load`:
```json
{ "ok":true, "title":"Auth debugging handoff", "compact_context":"…markdown…",
  "required_evidence":[{"part_id":"evidence.errors","content":"…"}],
  "available_details":[{"part_id":"detail.full-diff","kind":"patch","encrypted_size":2104032}] }
```
`doctor`:
```json
{ "ok":true, "age":{"found":true,"version":"1.2.0"}, "curl":true, "gzip":true,
  "server":{"url":"…","reachable":true} }
```
`error` (any mode): `{ "ok":false, "error_code":"…", "message":"…", "remediation":"…" }` ([[error-catalog]]).

### Exit codes
`0` ok · `1` generic · `2` usage · `3` missing dependency · `4` secret blocked · `5` size blocked · `6` network/server · `7` decryption.

### Config / env
- `SEND_SERVER_URL` (default server), overridable by `--server`.
- Optional `SEND_TEAM_TOKEN` → `Authorization: Bearer` (team mode).
- No persistent secrets written; key handling per [[security-privacy]].

## Constraints
- Output MUST be a single JSON object on stdout; human messages go to stderr for reliable parsing.
- Any failure MUST exit non-zero and emit an `error` object.
- `send` MUST show a preview and require `--yes` or interactive confirm unless `--dry-run`.

## Invariants
- **INV-1** — No mode transmits the fragment to the server.
- **INV-2** — `load` never fetches `load_by_default:false` parts.
- **INV-3** — `send` never uploads when the secret scan blocks (absent `--allow-secrets`) or the size hard cap is exceeded (absent `--include-large`).

## Error Handling
Each failure maps to a code in [[error-catalog]] and an exit code above.

## Conformance
- [ ] `doctor` detects a missing `age`;
- [ ] `--dry-run` uploads nothing;
- [ ] `load` is compact-first;
- [ ] fragment never transmitted (verified by request capture);
- [ ] stdout JSON is parseable; exit codes correct.