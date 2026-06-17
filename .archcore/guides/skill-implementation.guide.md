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

### 3. Send-mode instructions (SKILL.md body) — the fast path
`/send` is a **quick handoff, not an investigation.** The default is a two-step,
low-ceremony flow — **one Write + one `send --yes`** — so a simple handoff doesn't make
the user wait. The drag to avoid: template-padding, git reconciliation, a `doctor`
preflight, and a separate `inspect` preview. None of those run by default.

1. **Anything to send?** The agent must be able to name something concrete — a goal,
   bug, decision, plan, or branch state — from *this conversation* **or** from recalled
   context about ongoing work. If it genuinely can't (cold open, nothing recalled), stop
   and ask. Never mine git/repo to manufacture a narrative.
2. **Write one lean `compact.md`** into a temp dir (`mktemp -d`). Title (required, sets
   the send title) plus only the sections that carry real content — goal, current state,
   next steps; open questions / relevant files **only if real**. Summarize from what the
   session already knows; don't open git or read repo files. Caps per [[size-limits]].
3. **Send in one call:** `scripts/send.sh send <wd> --ttl 24h --yes`. `--yes` is the
   default skill behavior — the link is created immediately. The script still
   secret-scans ([[content-policy]]) and size-checks first and **refuses** on a
   high-confidence secret; the preview prints to stderr so the agent sees what went.
4. Return the URL + included/optional summary from the JSON. The link ends in
   `#agekey=…` — treat the whole link as a secret ([[security-privacy]]).

**Opt-in extras, only when the material genuinely exists** (not the default):
`evidence/*` for a few key errors/decisions to load by default; `details/*` for a real
big diff/log (lazy, never inlined); `/send --inspect` or dropping `--yes` for a look
before the link goes live; `/send --doctor` when deps look broken. Git stays **optional
corroboration only** — used to fill a diff summary the recipient explicitly needs, never
to source the context.

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
- Default `/send` is two steps (one Write + one `send --yes`) — no doctor/inspect/git preflight.
- Two-machine round-trip: send → copy URL → `--load` → compact appears, details listed (not injected).
- Request capture confirms no `#agekey` leaves the machine ([[security-privacy]] R2).

## Common Issues
- **age missing** → `AGE_NOT_FOUND`; doctor prints install hints ([[error-catalog]]).
- **Windows quoting** of the fragment → prefer `#k=<base64url>` encoding; test in `send.ps1` ([[e2ee-link-key-model]]).
- **Huge details injected** → ensure load is compact-first and details are lazy.
- **Fragment stripped by a tool** → never pass the full URL to remote tools; load locally.
- **Slow / heavy `/send`** → the default is the two-step fast path (one lean `compact.md` + `send --yes`). `doctor`, `inspect`, `evidence/`, `details/`, and git are **opt-in**, not preflight ceremony. Don't reconcile a draft against the working tree; summarize from what the session already knows.
- **False "nothing to package"** → the trigger to stop-and-ask is *can you name concrete context* (live **or** recalled), not *how many messages were exchanged*. Package genuine recalled work with its scope confirmed; only a true cold open stops and asks — never mine git for filler.