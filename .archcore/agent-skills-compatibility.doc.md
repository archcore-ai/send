---
title: "Agent Skills cross-agent compatibility (SKILL.md open standard)"
status: accepted
---

## Overview

The Send **skill** ships as a standard **Agent Skill**: a self-contained folder
(`SKILL.md` + `scripts/` + `references/` + `assets/`), no binaries, no agent-specific
code ([[repo-layout-and-skill-packaging]], [[skill-contract]] INV-4). The Agent Skills
format originated at Anthropic and is now an **open cross-agent standard**, which
validates [[skill-implementation]] PORT-2: a single `skill/send/` folder installs
unmodified across most agents — only the destination directory differs.

This doc records the **verified** compatibility surface as of **2026-06-12**. Every
claim below was checked against each tool's **official documentation** (not blogs).
The full source-cited matrix and one-liners live in `skill/send/INSTALL.md` — that
file is the operator-facing copy; this doc is the context-base record.

## Content

### Support tiers (verified 2026-06-12)

**Tier A — native `SKILL.md`, drop-in (19):** Claude Code, Claude Agent SDK,
GitHub Copilot / VS Code agent, Cursor 2.4+, Windsurf, Zed, Cline, Roo Code,
Codex CLI, Gemini CLI, Junie (JetBrains), Kiro (AWS), opencode, Amp (Sourcegraph),
Warp, Augment, Crush (Charmbracelet), Factory (droid), Devin (Cognition).

**Tier B — supported with a caveat (2):**
- **Claude.ai web / Desktop** — uploaded as a `.zip` via Settings → Features; runs in
  Anthropic's **sandbox VM**, so `send.sh` reaching a self-hosted Send server may be
  blocked by sandbox network policy. Strict `name` validation.
- **Goose (Block)** — not in core; via extension only. Old "Skills" ext deprecated
  (v1.16–1.24) → replaced by "Summon" (v1.25+), which still reads `…/SKILL.md`.

**Tier C — no native skills → thin shim on the script (3):** Continue
(`.continue/rules/*.md`), Aider (conventions + `/run bash scripts/send.sh`), Qodo
(publishes skills, not documented as a consumer). The script is self-sufficient, so a
3-line command/rules file pointing at `scripts/send.sh` covers these.

### Directory convention

- **`.agents/skills/<name>/`** is the broadest shared path — read by Codex, Gemini,
  Cursor, Zed, Roo, Amp, Warp, Devin, Augment, opencode, Copilot/VS Code.
- **`.claude/skills/<name>/`** is honored as a compat path by Copilot/VS Code, Cursor,
  Windsurf, Cline, opencode, Amp, Augment, Warp, Devin.
- Installing into **both** reaches nearly every Tier-A tool; the rest use their own
  dir (`.opencode/`, `.junie/`, `.kiro/`, `.factory/`, `~/.config/crush/`, …).

### Portability rules (carried into `SKILL.md`)

1. **Required frontmatter is `name` + `description` only.** Extra keys
   (`argument-hint`, `disable-model-invocation`) are documented-tolerated only by
   opencode (ignored), Cursor, and Codex; supported-in-practice by Claude Code,
   VS Code, Zed, Factory, Devin; **unverified elsewhere** (harmless — ignored).
2. **`name` must equal the folder name** on opencode, Augment, Roo, Cline, Zed
   (Send uses `send`/`send/`).
3. **The framework does not run the script — the agent does.** No tool auto-`chmod`s a
   shipped script, so `SKILL.md` instructs invocation via `bash "<skill-dir>/scripts/send.sh"`
   (not `./`), with **LF** endings, path resolved from the skill's own directory.
4. **Side-effecting `send` is gated behind explicit invocation** via
   `disable-model-invocation: true` (honored by Claude Code, VS Code, Zed, Cursor,
   Factory); on tools that ignore it, the script still confirms before upload unless
   `--yes` ([[security-privacy]]).

## Examples

- Drop-in: `cp -R skill/send ~/.claude/skills/send` (Claude/Copilot/Cursor/opencode/Amp…).
- Standard-compliant: `mkdir -p .agents/skills && cp -R skill/send .agents/skills/send`
  (Codex/Gemini/Zed/Devin/Warp…).
- Fallback (no skill support): a command/rules file that runs
  `bash skill/send/scripts/send.sh …` — no logic duplicated.
