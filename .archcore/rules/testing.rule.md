---
title: "Testing standard (skill + server)"
status: accepted
---

## Rule

Both parts MUST be covered by meticulous automated tests. Aligned with the Archcore CLI testing culture (its `.archcore/code-quality/unit-testing-patterns.guide.md` and `in-process-mcp-integration-tests.adr.md`).

### Server (Go) — unit
- Tests in the **same package** (test unexported); **table-driven**, `name` first field, loop var `tt`; pure stdlib `testing` — **no testify**.
- `t.TempDir()` for FS; `t.Helper()` setup helpers; `httptest.NewServer` for HTTP; minimal interface + mock struct in the test file; `t.Parallel()` when there is no global state.
- Run `go test ./... -race -count=1`.
- **Must cover** — every error path and invariant: store (`create/put/finalize/redeem/getpart/deleteexpired`); atomic one-time redeem incl. expired & already-consumed; size/TTL cap rejection (`413`); sha256 mismatch (`422`); grant validity/expiry (`403`); GC of expired/consumed/unfinished; **log scrubbing** (no body/token/fragment, [[security-privacy]] R5); public metadata carries no semantics ([[zero-knowledge-backend]]).

### Server (Go) — integration (in-process, no build tag)
- An `integration/` package wires the **real** handlers + real `SendStore` via `httptest.NewServer` and exercises **multi-step flows** end-to-end: create → upload → finalize → redeem → download → `age`-decrypt round-trips back to the original plaintext.
- **Concurrency** test: N parallel redeems → exactly one `200`, the rest `410` (atomicity, under `-race`).
- **At-rest** test: stored blobs are valid `age` ciphertext; metadata holds no plaintext titles.
- Runs on every `go test ./...` (cheap, in-process — mirrors the CLI's Layer-A decision).

### Skill (shell / PowerShell)
- `shellcheck` / `Invoke-ScriptAnalyzer` clean (lint gate, [[skill-scripting-conventions]]).
- **Unit**: `bats` (bash) / `Pester` (ps1) for pure functions — fragment split, secret scan, size check, JSON emit, arg parse. Stub `age`/`curl` with fakes on `PATH`.
- **Integration**: run the scripts against a **locally-started real `sendd`** — full `send` → `load` round-trip; assert compact-first load, details NOT auto-fetched, exit codes, single-JSON stdout.
- **Security tests (mandatory)**: capture outbound requests and assert the `#agekey` fragment never appears; a planted AWS key is blocked by default; temp plaintext + ephemeral identity are gone after success AND failure.

### E2E canary (deterministic, no LLM)
- Drive `send.sh`/`send.ps1` against a real in-process `sendd` across the full send→load journey; assert filesystem/JSON state. Runs in CI. (Real-agent / LLM-level testing is out of scope.)

### CI gates
- `server/`: `go vet`, `.golangci.yml`, `go test ./... -race`.
- `skill/`: `shellcheck` + `bats`; PowerShell `Invoke-ScriptAnalyzer` + `Pester`.
- Both green required to merge; release tags additionally run the canary.

## Rationale

Send moves sensitive context with E2EE and one-time semantics — correctness and security regressions are high-cost. Atomic-redeem, fragment-handling, secret-scan, and at-rest-ciphertext invariants are exactly what isolated unit tests miss, so the integration + security layers are **mandatory**, not optional. Matching the CLI's table-driven, stdlib-only, in-process-integration culture keeps both repos reviewable the same way.

## Examples

- **Good**: table-driven redeem test (`tt`, `t.TempDir`, `t.Parallel`); in-process create→load round-trip; a bats test asserting the fragment never leaves the host.
- **Bad**: testify asserts; an HTTP test without `defer srv.Close()`; a skill change merged with no `shellcheck`/`bats`; relying on a unit test where only an integration test exercises the real path.

## Enforcement

- Coverage of every code in [[error-catalog]] and every `INV-*` in [[backend-http-api]], [[send-format]], [[skill-contract]].
- PRs touching `server/` or `skill/` without corresponding tests are review-blocking.