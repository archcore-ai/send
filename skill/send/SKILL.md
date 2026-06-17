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
invent a narrative. Git and repo files are an *optional supplement* (to confirm the
diff summary or which files are relevant), never the primary source.

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
| `/send` | Build a workdir, then `send.sh send <workdir>` → return the link |
| `/send --inspect` | Build a workdir, then `send.sh inspect <workdir>` → preview only, no upload |
| `/send --load <url>` | `send.sh load <url>` → import compact + required evidence |
| `/send --load-detail <url> <part-id>` | `send.sh load-detail <url> <part-id>` → pull one optional detail |
| `/send --doctor` | `send.sh doctor` → check age/curl/gzip + server reachability |

Flags forwarded to the script: `--ttl <dur>` (default `24h`, max `7d`),
`--one-time` / `--no-one-time` (default one-time), `--yes`, `--dry-run`,
`--allow-secrets`, `--include-large`, `--server <url>`.

The server URL comes from `SEND_SERVER_URL` (or `--server`). For team mode, set
`SEND_TEAM_TOKEN` (sent as `Authorization: Bearer`).

## Send mode — assembling the workdir

**Before building anything, confirm there is genuine context to package — and what.**
The test is whether you can name something concrete (a goal, a bug, a decision, a plan,
the state of a branch), **not** how many messages were exchanged:

- **You can name it** — whether from live work this conversation *or* from memory/recall
  about ongoing work: that is real context. Don't refuse it. Briefly **confirm scope**
  with the user (recalled context can be broad or stale), then summarize what you
  genuinely know into the workdir.
- **You genuinely cannot** (cold open, nothing recalled): say there's nothing to package
  yet and ask the user what they want to send.

Either way, never **manufacture** context by digging into git/repo state to invent a
narrative you don't actually have — git stays optional corroboration.

Create a temp workdir with this exact layout (the script discovers parts by it):

```text
<workdir>/
├── compact.md            # REQUIRED — structured working context, loaded by default
├── evidence/             # small, high-signal — loaded by default
│   ├── errors.md
│   └── decisions.md
└── details/              # large diffs/logs — OPTIONAL, lazy-loaded only
    ├── full-diff.patch
    └── test-output.log
```

Convention applied by the script: `compact.md` + everything in `evidence/` are
**required & load-by-default**; everything in `details/` is **optional & lazy**.

Write `compact.md` from this template (the first line sets the send title):

```md
# Context: <short title>
## Goal
## Current state
## Current hypothesis
## What was already tried
## Decisions made
## Relevant files
- `path/to/file.ts` — why relevant
## Current diff summary
## Important errors
## Open questions
## Suggested next steps
## Exclusions / redactions
- Raw transcript not included. Secrets/env not included. Full logs are optional details only.
```

Then:

1. Write `compact.md` by **summarizing the session** (the template sections map to it);
   keep it ≤ ~30 KB (hard cap 50 KB) and put large material in `details/`.
2. Put **small** excerpts (key errors, decisions, a few file snippets) in `evidence/`.
3. Put **large** diffs/logs in `details/` — never inline them into compact.
4. Git is **optional corroboration only** — inspect it (when useful and permitted)
   to fill `## Current diff summary` / `## Relevant files`, never to source the
   context. Never include secrets, `.env`, keys, raw transcripts, or hidden reasoning
   (see `references/security.md`).
5. Run `bash "<skill-dir>/scripts/send.sh" send <workdir> --ttl <dur> [--one-time]`.
6. Show the preview (stderr) and **confirm** unless `--yes` was given.
7. Return the `url` and the `included` / `optional_parts` summary from the JSON.

The returned `url` ends with `#agekey=…`. **Treat the full link like a secret** —
anyone with it can decrypt. Never paste it into untrusted remote tools.

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
