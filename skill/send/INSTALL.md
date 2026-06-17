# Installing `send` on any agent — verified compatibility matrix

`send` is a standard **Agent Skill**: a self-contained folder (`SKILL.md` +
`scripts/` + `references/` + `assets/`) with **no binaries** and **no agent-specific
code**. The Agent Skills format started at Anthropic and is now an open standard, so
the *same* `skill/send/` folder installs unmodified across the tools below.

Every row was checked against the tool's **official documentation** (not third-party
blogs). Rows marked **partial** / **shim** carry a verified caveat.

> **Bottom line:** drop `skill/send/` into the agent's skills directory — only the
> *destination path* changes. For the broadest reach, install once into
> **`.agents/skills/send/`** (the shared convention) and/or **`.claude/skills/send/`**
> (honored as a compat path by most others). On tools with no skill support, point a
> 3-line command/rules file at `scripts/send.sh` — the script is the product.

## Tier A — native `SKILL.md`, drop-in (verified)

| Agent | Project dir(s) | Global dir(s) | Invoke | Source |
|---|---|---|---|---|
| **Claude Code** | `.claude/skills/send/` | `~/.claude/skills/send/` | `/send` | code.claude.com/docs/en/skills |
| **Claude Agent SDK** | `.claude/skills/` | `~/.claude/skills/` | `Skill` tool¹ | code.claude.com/docs/en/agent-sdk/skills |
| **GitHub Copilot / VS Code agent** | `.github/skills/`, `.claude/skills/`, `.agents/skills/` | `~/.copilot/skills/`, `~/.claude/skills/`, `~/.agents/skills/` | `/send` + implicit | code.visualstudio.com/docs/agent-customization/agent-skills · docs.github.com/en/copilot/concepts/agents/about-agent-skills |
| **Cursor 2.4+** | `.agents/skills/`, `.cursor/skills/`, compat `.claude/skills/` | `~/.agents/skills/`, `~/.cursor/skills/` | implicit + `/` | cursor.com/docs/skills |
| **Windsurf (Cascade)** | `.windsurf/skills/`, `.agents/skills/`, compat `.claude/skills/` | `~/.codeium/windsurf/skills/` | implicit + `@skill` | docs.windsurf.com/windsurf/cascade/skills |
| **Zed** | `.agents/skills/send/` | `~/.agents/skills/send/` | autonomous + `/` + `@skill` | zed.dev/docs/ai/skills |
| **Cline** | `.cline/skills/`, `.clinerules/skills/`, `.claude/skills/` | `~/.cline/skills/` | `use_skill` + `/` | docs.cline.bot/customization/skills |
| **Roo Code** | `.roo/skills/`, `.agents/skills/` | `~/.roo/skills/`, `~/.agents/skills/` | implicit | docs.roocode.com/features/skills |
| **Codex CLI** | `.agents/skills/` (scans cwd → repo root) | `~/.agents/skills/`, `/etc/codex/skills` | `/skills`, `$send`, implicit | developers.openai.com/codex/skills |
| **Gemini CLI** | `.gemini/skills/` or `.agents/skills/` | `~/.gemini/skills/` or `~/.agents/skills/` | `/skills`, implicit | geminicli.com/docs/cli/skills |
| **Junie (JetBrains)** | `.junie/skills/send/` | `~/.junie/skills/send/` | implicit | junie.jetbrains.com/docs/agent-skills.html |
| **Kiro (AWS)** | `.kiro/skills/` | `~/.kiro/skills/` | implicit + `/` | kiro.dev/docs/skills |
| **opencode** | `.opencode/skills/`, `.claude/skills/`, `.agents/skills/` | `~/.config/opencode/skills/`, `~/.claude/skills/`, `~/.agents/skills/` | `skill` tool | opencode.ai/docs/skills |
| **Amp (Sourcegraph)** | `.agents/skills/`, compat `.claude/skills/` | `~/.config/agents/skills/`, `~/.agents/skills/`, `~/.config/amp/skills/` | implicit + user-invokable | ampcode.com/manual |
| **Warp** | cwd → repo root | `~/.agents/skills/`, `~/.warp/skills/` (+ `~/.claude/`, `~/.codex/`, … compat) | auto-discovery | docs.warp.dev/agent-platform/capabilities/skills |
| **Augment (Auggie)** | `.augment/skills/`, `.claude/skills/`, `.agents/skills/` | same | `/send`, `/skills` | docs.augmentcode.com/cli/skills |
| **Crush (Charmbracelet)** | — | `~/.config/crush/skills/`, `~/.config/agents/skills/` (`CRUSH_SKILLS_DIR`) | conversational | github.com/charmbracelet/crush |
| **Factory (droid)** | `.factory/skills/`, compat `.agent/skills/` | `~/.factory/skills/` | `/send` + implicit | docs.factory.ai/cli/configuration/skills |
| **Devin (Cognition)** | `.agents/skills/` (+ `.github/`, `.claude/`, `.cursor/`, … compat) | — | `@skills:send` + auto | docs.devin.ai/product-guides/skills |

¹ Agent SDK: load requires `settingSources` to include `user`/`project`, and the
script needs `Bash` in `allowedTools` (the skill's own `allowed-tools` frontmatter is
ignored by the SDK).

## Tier B — supported with a caveat

| Agent | Status | Caveat (verified) | Source |
|---|---|---|---|
| **Claude.ai web / Desktop** | partial | Upload as a **.zip** via Settings → Features (Pro/Max/Team/Enterprise, code-execution on). Runs in **Anthropic's sandbox VM**, not your machine — `send.sh` reaching *your* Send server may be **blocked** by sandbox network policy. Strict `name` validation (lowercase/num/hyphen, ≤64, no "claude"/"anthropic"). | platform.claude.com/docs/en/agents-and-tools/agent-skills/overview |
| **Goose (Block)** | via extension | **Not in core.** Old "Skills" extension is deprecated (v1.16–1.24); v1.25+ replaced it with the **"Summon"** extension, which still reads `…/SKILL.md`. Dirs `.agents/skills/`, global `~/.config/agents/skills/`. | block.github.io/goose/docs/mcp/skills-mcp |

## Tier C — no native skills → use the script directly (shim)

| Agent | Why | Fallback |
|---|---|---|
| **Continue** | Customization is the **rules** system (`.continue/rules/*.md`); no SKILL.md discovery. | A `.continue/rules/*.md` that instructs the agent to run `bash scripts/send.sh`. |
| **Aider** | Conventions files + `/run` only; no skills feature. (`aider-skills` on PyPI is third-party.) | A conventions/`AGENTS.md` line telling aider to `/run bash scripts/send.sh`. |
| **Qodo** | **Publishes** skills into *other* agents; its own runtime isn't documented as a SKILL.md consumer. | Same shim approach, or run the script manually. |

For any agent not listed, the script is self-sufficient — anything that can run a
shell command (or a human) can use it:

```sh
skill/send/scripts/send.sh doctor
skill/send/scripts/send.sh send <workdir> --ttl 24h --one-time
skill/send/scripts/send.sh load  "<url>#agekey=…"
```

## One-liners

```sh
# Single tool
cp -R skill/send ~/.claude/skills/send                    # Claude Code / Copilot / Cursor / opencode / Amp …
cp -R skill/send .opencode/skills/send                    # opencode (project)
mkdir -p .agents/skills && cp -R skill/send .agents/skills/send   # Codex / Gemini / Zed / Devin / Warp …

# Multi-agent: one real copy + symlinks (from repo root)
cp -R skill/send .agents/skills/send
for d in .claude .cursor .opencode .windsurf; do
  mkdir -p "$d/skills" && ln -s "../../.agents/skills/send" "$d/skills/send"
done
```

Then set the server and verify from inside the agent:

```sh
export SEND_SERVER_URL="https://send.example.com"   # or pass --server
/send --doctor      # checks age / curl / gzip + server reachability
```

## Cross-cutting rules that keep it portable

1. **`.agents/skills/` + `.claude/skills/` cover the field.** `.agents/skills/` is
   read by Codex, Gemini, Cursor, Zed, Roo, Amp, Warp, Devin, Augment, opencode,
   Copilot/VS Code. `.claude/skills/` is honored as a compat path by Copilot/VS Code,
   Cursor, Windsurf, Cline, opencode, Amp, Augment, Warp, Devin. Install into both and
   you reach nearly everyone.
2. **Keep frontmatter to `name` + `description`.** Only **opencode** ("unknown fields
   are ignored"), **Cursor**, and **Codex** *document* tolerance of extra keys. Our
   extra keys (`argument-hint`, `disable-model-invocation`) are supported by Claude
   Code, VS Code, Zed, Cursor, Factory, Devin — but are **unverified elsewhere**. They
   are harmless (ignored) in practice; don't depend on them for behavior on other tools.
3. **`name` must equal the folder name** on opencode, Augment, Roo, Cline, Zed. Ours is
   `send` in a `send/` folder — keep them in sync if you rename.
4. **The framework does not run the script — the agent does.** No tool auto-`chmod`s a
   shipped script. The `SKILL.md` body instructs the agent to invoke it; use
   `bash scripts/send.sh …` (not `./`) and keep **LF** line endings for cross-platform
   safety. The script runs with the agent's full tool permissions — Crush and Junie
   explicitly warn to install only trusted skills.
5. **Gate the side-effecting `send` behind explicit invocation.** Our
   `disable-model-invocation: true` (honored by Claude Code, VS Code, Zed, Cursor,
   Factory) keeps upload user-triggered; on tools that ignore it, the script still
   confirms before uploading unless `--yes`.

## Requirements (every target)

- [`age`](https://age-encryption.org) — `brew install age` ·
  `winget install FiloSottile.age` · distro package
- `curl` + `gzip` (preinstalled on macOS/Linux; PowerShell uses `Invoke-WebRequest`
  + built-in compression)
- A reachable Send server (`SEND_SERVER_URL` or `--server`)

Run `/send --doctor` (or `send.sh doctor`) — it lists exactly what's missing.
