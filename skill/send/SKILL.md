---
name: send
description: Send or load end-to-end-encrypted context for AI coding sessions. Use when the user asks to share, hand off, package, send, import, or load current working context.
argument-hint: "[--load <url>] [--load-detail <url> <part-id>] [--doctor] [--ttl 24h] [--yes]"
disable-model-invocation: true
---

# Send

`/send` packages the **working context** of the current AI coding session, encrypts
it locally with `age`, uploads only ciphertext to a Send server, and returns a
**one-time link**. The recipient runs `/send --load <url>` to pull a compact,
high-signal context into their session. The server never sees plaintext or the key.

**Working context = what this session genuinely knows.** Primarily it is what you and
the user have been doing in *this conversation*: the goal, the current state, what was
tried, what was decided, the errors you hit, the open questions. It **also** includes
concrete, specific context the session has surfaced from memory/recall about ongoing
work — a named bug, a plan, the state of a branch. That is real context, not filler.
You build it by **summarizing what you actually know** — never by mining the repo to
invent a narrative. Git and repo files are an *optional supplement* — only when the
user explicitly asks for a diff — never the primary source and never a substitute for
session memory.

You (the agent) **assemble the plaintext workdir and write files**. The script
(`scripts/send.sh`, or `scripts/send.ps1` on Windows) does **everything else**:
crypto, transport, secret scanning, size checks, and temp-file hygiene. The script
emits exactly **one JSON object on stdout**; parse that. Human text goes to stderr.

> **Boundary (do not cross):** the script never summarizes, never reads arbitrary
> repo files, and never mutates project files. Building good context is *your* job;
> encrypting and moving bytes is *its* job.

## Running the bundled script

The script ships **inside this skill's directory** and may not be marked executable.
Always invoke it through an interpreter, resolving its path **from this skill's own
directory** — never assume the current working directory, and never use `./`:

- **POSIX/macOS/Linux:** `bash "<skill-dir>/scripts/send.sh" <args>`
- **Windows:** `pwsh -File "<skill-dir>/scripts/send.ps1" <args>`

`<skill-dir>` is the folder containing this `SKILL.md`. On Claude Code it is
`${CLAUDE_SKILL_DIR}`; on other agents use whatever skill-root variable they expose,
or the absolute path of this skill folder. In the steps below, `send.sh <args>` is
shorthand for exactly this invocation.

## Modes

| Invocation | What you do |
|---|---|
| `/send` | Write one lean `compact.md`, then `send.sh send <wd> --ttl 24h --yes` → return the link |
| `/send --inspect` | Build a workdir, then `send.sh inspect <workdir>` → preview only, no upload |
| `/send --load <url>` | `send.sh load <url>` → import compact + required evidence |
| `/send --load-detail <url> <part-id>` | `send.sh load-detail <url> <part-id>` → pull one optional detail |
| `/send --doctor` | `send.sh doctor` → check age/curl/gzip + server reachability |

Flags forwarded to the script: `--ttl <dur>` (default `24h`, max `7d`),
`--one-time` / `--no-one-time` (default one-time), `--yes`, `--dry-run`,
`--allow-secrets`, `--include-large`, `--server <url>`.

The server URL comes from `SEND_SERVER_URL` (or `--server`). For team mode, set
`SEND_TEAM_TOKEN` (sent as `Authorization: Bearer`).

## Send mode — the fast path (default)

`/send` is a **quick handoff, not an investigation.** Two steps: write one lean
`compact.md`, then `send.sh send … --yes`. Nothing else by default — **no `doctor`,
no `inspect`, no git, no `evidence/`** unless a real artifact demands it. Template-
padding, git reconciliation, and a separate preview are the ceremony that made `/send`
slow; skip them.

**Is there anything to send?** You must be able to name something concrete — a goal, a
bug, a decision, a plan, a branch's state — from this conversation *or* from recalled
context about ongoing work. If you genuinely can't (cold open, nothing recalled), stop
and ask. Never mine git/repo to manufacture a narrative.

**Step 1 — write one file.** Make a temp dir and write `compact.md` into it (the script
only requires this one file):

```text
wd=$(mktemp -d)        # then Write "$wd/compact.md"
```

Use this **lean** template. The first line sets the send title and is required; **write
only the sections you can fill without inventing or generalizing — omit the rest.** A
section with a placeholder or a vague sentence is worse than no section. Keep it ≤ ~30 KB
(hard cap 50 KB).

```md
# Context: <short title>
## Goal
## Current state
## Next steps
## Open questions      # only if real
## Relevant files      # only if real — `path` — why it matters
```

Summarize from what you **already know**. Don't open git, read repo files, or reconcile
against the working tree — that round-trip is the ceremony to avoid.

**Step 2 — send it (one call):**

```text
bash "<skill-dir>/scripts/send.sh" send "$wd" --ttl 24h --yes
```

`--yes` creates the link immediately, no confirm. The script still **secret-scans and
size-checks first** and refuses on a high-confidence secret (override only with
`--allow-secrets`); the preview still prints to stderr, so you see exactly what went.

**Step 3 — return the link.** Parse the JSON; return `url` plus the `included` /
`optional_parts` summary. The `url` ends in `#agekey=…` — **treat the whole link like a
secret**; anyone with it can decrypt. Never paste it into untrusted remote tools.

### Opt-in only — default is one file, no extras

The two-step fast path is the complete default. Stop there unless one of these is
**literally true** — if you're only evaluating whether it *might* apply, skip it:

- You have an actual log or diff file the recipient must read → `details/<name>.log` /
  `.patch` (lazy-loaded, never inlined into compact).
- You have a concrete, bounded error or decision artifact → `evidence/<name>.md`
  (loaded by default, alongside compact).
- User asked to preview before upload → `/send --inspect`, or drop `--yes` for the
  interactive confirm.
- A dependency is broken → `/send --doctor`.

Git is **never** a source of context — touch it only to copy a specific diff the
recipient explicitly requested. Never put secrets, `.env`, keys, raw transcripts, or
hidden reasoning into the workdir (see `references/security.md`).

## Load mode

1. Run `bash "<skill-dir>/scripts/send.sh" load <url>`.
2. Import **only** `compact_context` + `required_evidence` from the JSON.
3. Summarize what loaded; list `available_details` (do not fetch them).
4. Ask the user before loading a large detail. Do not modify project files until
   the user confirms what they want done with the context.

## Load-detail mode

Run `bash "<skill-dir>/scripts/send.sh" load-detail <url> <part-id>` for one optional part. If the
content is large (a big diff/log), summarize it rather than dumping it verbatim
into the conversation.

## Output contract

- stdout is always a single JSON object; parse it, don't echo it raw.
- Exit codes: `0` ok · `1` generic · `2` usage · `3` missing dependency ·
  `4` secret blocked · `5` size blocked · `6` network/server · `7` decryption.
- On any non-zero exit the JSON is `{ "ok": false, "error_code": …, "message": …,
  "remediation": … }` — surface the remediation to the user.

## References

- `references/format.md` — the `send.v1` format (parts, manifest, pipeline).
- `references/security.md` — security & privacy rules (fragment handling, temp hygiene).
- `references/backend.md` — the server HTTP API.
