---
title: "Content Policy: secret scanning & exclusions"
status: accepted
---

## Rule

- **R1 — Secret scan before encryption.** `send` MUST run a local regex secret scan on the assembled workdir **before** encryption (the server can't scan ciphertext — [[zero-knowledge-backend]]). High-confidence matches BLOCK upload by default. Override only with explicit `--allow-secrets`, which MUST log counts/types — never values.
- **R2 — Default exclusions** (referenceable by path/hash, not inlined):
  ```text
  node_modules/ dist/ build/ coverage/ .git/
  *.lock package-lock.json pnpm-lock.yaml yarn.lock
  *.min.js *.map  *.png *.jpg *.jpeg *.gif *.pdf
  *.zip *.tar *.gz *.sqlite *.db  .env *.pem *.key id_rsa
  ```
- **R3 — Never include by default:**
  ```text
  .env contents · API keys · OAuth/JWT/private tokens · private keys · credentials
  unrelated chat transcript · hidden chain-of-thought / private reasoning
  full customer PII · full raw logs (unless explicitly asked) · generated/minified/lock files
  ```
  *Carve-out:* the ban targets the **full / unrelated / hidden** transcript — not the session's own short, **visible** exchange when that exchange *is* the working context (a thin or greeting-only session). Such a short exchange MAY be quoted close to verbatim in `compact` (see R4); it is not a "raw transcript". Hidden reasoning and any full or unrelated transcript stay excluded regardless.
- **R4 — Compact-first content.** `compact` carries structured working context (goal, state, hypothesis, decisions, files, errors, next steps), not raw transcript. Large diffs/logs are `detail.*` (optional), never inlined into compact ([[size-limits]]). **Thin sessions:** when the session is mostly a short visible exchange, `compact` MAY carry that exchange close to verbatim in a `## Conversation` section — that *is* the structured context for such a session; the manifest `raw_transcript_included` flag stays `false`, because a short curated exchange is not the full raw transcript.
- **R5 — Preview discloses.** The send preview MUST list included / optional / skipped items and state that the server receives ciphertext only ([[skill-contract]] UX-1).

### Secret patterns (minimum set)
```text
-----BEGIN (RSA |EC |OPENSSH |)PRIVATE KEY-----
AKIA[0-9A-Z]{16}                                   # AWS access key id
(SECRET|TOKEN|API_KEY|PASSWORD)\s*=\s*\S+          # env assignments
ghp_[0-9A-Za-z]{36} | github_pat_[0-9A-Za-z_]{59} # GitHub
xox[baprs]-[0-9A-Za-z-]+                           # Slack
eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+ # JWT
sk-[A-Za-z0-9]{20,}                                # OpenAI-style
postgres://|mysql://|mongodb(\+srv)?://            # DB connection strings
```

## Rationale

Because the backend is zero-knowledge, **the client is the only place** secrets can be caught. Blocking pre-encryption protects the sender from baking credentials into an artifact the server can never redact ([[security-privacy]], [[threat-model]]). Default exclusions keep sends compact and high-signal.

## Examples

**Good** — preview shows `compact 38 KB · evidence 64 KB · optional detail.full-diff 1.9 MB · skipped pnpm-lock.yaml, coverage/, .env`; a planted `AKIA…` blocks with `SECRET_DETECTED`.

**Bad** — inlining `.env` into compact; pasting the full chat transcript; embedding a 40 MB `server.log` instead of an excerpt.

## Enforcement

- Scanner runs in `send.sh` / `send.ps1` pre-encrypt; failure → `SECRET_DETECTED` (exit 4) ([[error-catalog]]).
- Tests with planted secrets (AWS, GitHub, JWT, PEM) MUST block by default and pass only with `--allow-secrets`.
- Preview snapshot test asserts skipped/optional disclosure.
- Log tests assert no secret values are emitted.