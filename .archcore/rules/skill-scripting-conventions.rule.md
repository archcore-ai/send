---
title: "Skill scripting conventions (shell & PowerShell)"
status: accepted
---

## Rule

The skill's only executables are `send.sh` (POSIX bash) and `send.ps1` (PowerShell). They MUST be readable, auditable, dependency-light, and **binary-free** ([[age-dependency-no-bundled-binary]], [[repo-layout-and-skill-packaging]]). The patterns below mirror the Archcore CLI installers (`install.sh` / `install.ps1`).

### Bash (`send.sh`)
- Shebang `#!/usr/bin/env bash`; guard `BASH_VERSION`; `set -euo pipefail`.
- TTY-aware color; **logging helpers** `info/success/warn/error_exit` via `printf` (never raw `echo` for formatted output). Human text → **stderr**; the single JSON result → **stdout** ([[skill-contract]]).
- `need_cmd <tool>` preflight for `age`, `curl`, `gzip` → `AGE_NOT_FOUND` etc. ([[error-catalog]]).
- Temp via `mktemp -d`; `umask 077`; **cleanup trap** `trap 'rm -rf "$tmp"' EXIT INT TERM` ([[security-privacy]] R3).
- Injection-safe: fixed-string matching (`grep -F`), quote every expansion, no `eval`; never `set -x` around secrets/plaintext.
- `curl -fsS --retry 3 --retry-delay 2`; atomic file ops (`.tmp` + `mv`).
- Exit codes per [[skill-contract]].

### PowerShell (`send.ps1`)
- `#Requires -Version 5.1`; `Set-StrictMode -Version Latest`; `$ErrorActionPreference='Stop'`; force TLS 1.2.
- ANSI via `[char]27` (works on PS 5.1); logging functions `Write-Info/Success/WarnMsg/ErrExit`; errors → `[Console]::Error`.
- `Invoke-WebRequest -UseBasicParsing` with a retry loop; `Get-FileHash -Algorithm SHA256`.
- Atomic temp/install (`*.tmp.$PID` + `Move-Item`); `try/catch/finally` cleanup of the temp dir; quote/escape paths.
- Mirror the same subcommands/flags/JSON/exit codes as `send.sh` ([[skill-contract]]).

### Security overlay (both)
- Parse the `#agekey` fragment **locally**; it MUST NOT appear in any request, log, or argv ([[security-privacy]] R2).
- Run the secret scan **before** encryption; block by default ([[content-policy]]).
- Delete temp plaintext and the ephemeral identity on success AND failure; never persist the key.

### Lightweight mandate
No bundled binaries; no Python/Node/Go; no package installs; no `curl | sh`; no auto-update; no dynamic remote code. The scripts only orchestrate `age`/`curl`/`gzip` and talk to the server ([[backend-http-api]]).

## Rationale

The scripts run on the sender/recipient machine and see plaintext before/after crypto — they must be trivially auditable. Reusing the CLI installer idioms (strict mode, traps, atomic ops, injection-safe parsing, retry) gives battle-tested patterns and a consistent feel across Archcore tools.

## Examples

**Good (bash)**
```sh
set -euo pipefail
base="${url%%#*}"; key="${url#*#agekey=}"     # split fragment locally
umask 077; tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT INT TERM
need_cmd age; need_cmd curl
printf '%s\n' "$json" ; >&2 info "uploaded"   # JSON→stdout, logs→stderr
```
**Bad**: `curl "$url"` (leaks `#agekey`); `echo "key=$key"`; `set -x` near secrets; bundling `age`.

## Enforcement

- `shellcheck send.sh` clean; `Invoke-ScriptAnalyzer send.ps1` clean (CI gate).
- Tests (bats / Pester) per [[testing]]: fragment-never-sent, secret-block-by-default, temp cleanup, exit codes, single-JSON stdout.