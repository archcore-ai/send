---
title: "Go conventions for the Send server"
status: accepted
---

## Rule

Go is used **only** for the Send server ([[go-single-binary-server]]); the skill contains no Go ([[repo-layout-and-skill-packaging]]). These conventions are **aligned with the Archcore CLI** — its `.archcore/code-quality/go-code-quality.rule.md` and `strict-go-naming-conventions.rule.md` are the upstream source of truth; restated here so both codebases read the same. New code MUST conform; a deviation needs a one-line comment naming the reason.

### Toolchain & build
- Go 1.25+ (track the CLI toolchain). Server module rooted at `server/`, e.g. `github.com/<org>/archcore-send`.
- **Static binaries**: `CGO_ENABLED=0`. State store uses `modernc.org/sqlite` (pure-Go, cgo-free) — never a cgo SQLite driver.
- **Version via ldflags** (mirrors CLI `version-injection-via-ldflags.rule`): `main.go` declares `var ( version = "dev"; commit = "none" )`; GoReleaser injects `-s -w -X main.version={{.Version}} -X main.commit={{.Commit}}`. No hardcoded version, no `version` subcommand.
- Release: tag-driven GoReleaser, cross-compile linux/darwin/windows × amd64/arm64, `checksums.txt` (sha256). Primary target is linux amd64/arm64 ([[self-host-deploy]]).

### Error handling
- Wrap with context: `fmt.Errorf("context: %w", err)` — never bare from exported funcs; `%v` only when intentionally non-unwrappable.
- Validate-then-act: check caps/sha256/preconditions and return **before** any side effect.
- Bound reads of untrusted input with `io.LimitReader` (bodies capped per [[size-limits]]).
- Sentinel errors `Err<Cause>`; custom types `<cause>Error`. Messages lowercase, no trailing punctuation (proper nouns keep case: "S3 upload failed").
- Handlers map errors to the status/code in [[error-catalog]] — never leak internal error text to the client.

### Naming (per CLI strict naming)
- Acronyms one case: `URL`, `ID`, `SHA256`, `HTTP`, `API`, `TTL`. Wire tags stay as protocol dictates (`json:"part_id"`).
- `New<Type>` constructors; `Build<Type>` for multi-step assembly. Single-letter receivers, uniform per type.
- Typed string enums + const block + map validation: `type SendStatus string` (`SendStatusCreating/Finalized/Expired/Deleted`). No untyped closed-set strings; no `default: false` switch.
- Files snake_case; tests alongside as `_test.go`. Packages lowercase single-word — no `util`/`common`/`helpers`.

### Concurrency (server differs from the single-threaded CLI)
- The server is an HTTP service: per-request goroutines are expected. **Shared mutable state MUST be synchronized**; prefer none — the store is the single source of truth.
- One-time redemption's **only** consume path is the atomic conditional `UPDATE … WHERE consumed_at IS NULL … RETURNING` ([[practical-one-time-redemption]], [[backend-http-api]]) — never read-then-write.
- Propagate `r.Context()` to all store/IO calls; `signal.NotifyContext` in `main` for graceful shutdown. `SendStore` impls MUST be concurrency-safe.

### File / path / IO
- Reject `..` and absolute paths from any input-derived path; `filepath.Clean` then prefix-check the blob dir. `0o644` files, `0o755` dirs. Atomic writes (`.tmp` + `os.Rename`).
- Stream large parts (`io.Copy`); never buffer a whole detail in memory.

### Dependencies (minimal — this is a security tool)
- stdlib `net/http` (or a thin router like `chi`); `modernc.org/sqlite`; `minio-go`/AWS SDK only when S3 is enabled.
- **No** assertion libs (testify) — stdlib `testing` only ([[testing]]). **No** heavy logging framework; a thin logger that obeys [[security-privacy]] R5 (never log bodies/tokens/fragments).

## Rationale

Send's server is a sibling of the Archcore CLI; shared conventions mean one mental model, the same `review-go` tooling, and no per-repo bikeshedding. Static cgo-free builds keep self-hosting trivial ([[go-single-binary-server]]); a minimal dependency set keeps the audit surface small — central to a privacy tool ([[threat-model]]).

## Examples

```go
// %w wrap + validate-then-act
if err := validateCaps(in); err != nil { return fmt.Errorf("create send: %w", err) }

// typed enum + exhaustive map (no default:false)
type SendStatus string
const ( SendStatusCreating SendStatus = "creating"; SendStatusFinalized SendStatus = "finalized" )
var validStatus = map[SendStatus]bool{ SendStatusCreating: true, SendStatusFinalized: true }

// atomic one-time redeem — the ONLY consume path
// UPDATE sends SET consumed_at=? WHERE id=? AND consumed_at IS NULL AND status='finalized' AND expires_at>? RETURNING id
```
Bad: `return err` (bare); cgo SQLite; `const Version = "1.2.3"`; read-then-write redeem; `testify`.

## Enforcement

- CI: `go vet ./...`, `.golangci.yml` (gofumpt, revive, stylecheck ST1003, errcheck, errname, exhaustive), `go test ./... -race`.
- GoReleaser builds with `CGO_ENABLED=0`. Review against this rule and the CLI's `go-code-quality` / `strict-go-naming` rules (same checks).