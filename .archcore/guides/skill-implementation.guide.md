---
title: "Implementing the Send Skill (client)"
status: draft
---

## Prerequisites

- `age` installed (client crypto); `curl` + `gzip` (or PowerShell equivalents).
- A reachable Send server ([[backend-http-api]]); default via `SEND_SERVER_URL`.
- Familiarity with the Agent Skills format (`SKILL.md` + scripts/references/assets).
- Contracts to obey: [[skill-contract]], [[send-format]]. Rules: [[content-policy]], [[security-privacy]], [[size-limits]].

## Steps

### 1. Skill layout
```text
send/
├── SKILL.md
├── scripts/
│   ├── send.sh
│   └── send.ps1
├── references/
│   ├── format.md      # mirrors send-format.spec
│   ├── security.md    # mirrors security-privacy.rule
│   └── backend.md     # mirrors backend-http-api.spec
└── assets/
    └── send.schema.json   # optional: private-manifest schema
```

### 2. `SKILL.md` frontmatter
```yaml
---
name: send
description: Send or load end-to-end-encrypted context for AI coding sessions. Use when the user asks to share, hand off, package, send, import, or load current working context.
argument-hint: "[--load <url>] [--load-detail <url> <part-id>] [--doctor] [--ttl 24h] [--yes]"
disable-model-invocation: true
---
```
`disable-model-invocation: true` — send/load is a side-effecting, user-invoked workflow.

### 3. Send-mode instructions (SKILL.md body)
On `/send`, the agent should:
1. **Summarize what the session genuinely knows** into compact working context (goal, state, hypothesis, tried, decisions, relevant files, errors, open questions, next steps) → `compact.md`, within caps ([[size-limits]]). The source is *this conversation* **plus** any concrete context recalled from memory about ongoing work (a named bug, a plan, a branch's state) — recalled context is real, so **confirm its scope** with the user rather than refusing it. Only when you can name nothing concrete (cold open, nothing recalled) stop and ask the user; never manufacture context from git or repo state.
2. Add small `evidence/*` (errors, decisions, file excerpts); put large diffs/logs in `details/*` ([[content-policy]]).
3. Write `manifest.json` (semantic map) per [[send-format]].
4. Git is **optional corroboration only** — inspect it (when useful and permitted) to fill the diff summary / relevant files, never to source the context.
5. Run `scripts/send.sh send <workdir> --ttl … [--one-time]`.
6. Show the preview; require confirmation unless `--yes`.
7. Return the URL + included/optional summary from the script's JSON.

### 4. Load-mode instructions
On `/send --load <url>`:
1. Run `scripts/send.sh load <url>`.
2. Import only `compact_context` + `required_evidence` from the JSON.
3. Summarize what loaded; list `available_details`.
4. Ask before loading large details; do not modify project files until the user confirms.

### 5. Load-detail mode
On `/send --load-detail <url> <part-id>`: run the script, summarize if large, avoid dumping huge logs/diffs into context.

### 6. Script responsibilities (`send.sh` / `send.ps1`)
Per [[skill-contract]]: locate `age`/`curl`/`gzip`; secret-scan ([[content-policy]]); size-check ([[size-limits]]); `gzip`+`age` per part; keygen ephemeral identity; create/upload/finalize via [[backend-http-api]]; append `#agekey` locally; on load — redeem, download, decrypt; emit one JSON object on stdout; clean temp files ([[security-privacy]]). **Non-responsibilities:** no summarization, no arbitrary repo reads, no project mutation.

### 7. OpenCode / other agents
Optional shim `.opencode/commands/send.md` that tells OpenCode to use the `send` skill with the given args. Source of truth stays `send/SKILL.md` ([[archcore-send]] PORT-2).

## Verification
- `/send --doctor` reports age/curl/gzip + server reachability.
- `/send --inspect` / `--dry-run` builds a workdir and prints a preview, uploads nothing.
- Two-machine round-trip: send → copy URL → `--load` → compact appears, details listed (not injected).
- Request capture confirms no `#agekey` leaves the machine ([[security-privacy]] R2).

## Common Issues
- **age missing** → `AGE_NOT_FOUND`; doctor prints install hints ([[error-catalog]]).
- **Windows quoting** of the fragment → prefer `#k=<base64url>` encoding; test in `send.ps1` ([[e2ee-link-key-model]]).
- **Huge details injected** → ensure load is compact-first and details are lazy.
- **Fragment stripped by a tool** → never pass the full URL to remote tools; load locally.
- **False "nothing to package"** → the trigger to stop-and-ask is *can you name concrete context* (live **or** recalled), not *how many messages were exchanged*. Package genuine recalled work with its scope confirmed; only a true cold open stops and asks — never mine git for filler.