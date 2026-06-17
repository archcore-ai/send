# 📦 Send

**Hand off your AI coding session — encrypted, one-time, zero-knowledge.**

You're three hours into debugging with one agent. Now you want to continue on
another machine, with another model, or with a teammate — *without* re-explaining
everything or pasting secrets into a chat box. `/send` packages the working
context, encrypts it locally, and gives you a one-time link. The other side runs
`/send --load <link>` and picks up exactly where you left off.

```text
/send                 →  🔗 https://send.example.com/s/snd_01J…#agekey=AGE-SECRET-KEY-…
/send --load <link>   →  ✅ Loaded "Auth debugging handoff" — compact-first.
```

## Why it's different

- 🔒 **Zero-knowledge server.** It only ever stores ciphertext + opaque sizes.
  No plaintext, no titles, no key — ever.
- 🗝️ **The key lives in the link.** End-to-end encryption with [`age`](https://age-encryption.org);
  the decryption key rides in the URL fragment and **never** touches the network.
- 🎯 **Compact-first.** Loads a tight, high-signal context (~8k tokens) by default;
  big diffs and logs stay as optional, lazy-loaded details.
- 🧹 **No binaries, fully auditable.** Just `SKILL.md` + a shell/PowerShell script
  orchestrating `age`, `gzip`, and `curl`. Read every line.

## Quick start

```text
/send --doctor               # check age / curl / gzip + server reachability
/send                        # package this session → encrypted one-time link
/send --inspect              # preview what WOULD be sent — uploads nothing
/send --load <url>           # load a received link into this session
/send --load-detail <url> <part-id>   # pull one optional detail on demand
```

Useful flags: `--ttl 24h` (max `7d`) · `--one-time` (default) · `--yes` ·
`--allow-secrets` · `--include-large` · `--server <url>`.

## Requirements

- [`age`](https://age-encryption.org) — `brew install age` ·
  `winget install FiloSottile.age` · or your distro package
- `curl` + `gzip` (preinstalled on macOS/Linux; PowerShell on Windows uses
  `Invoke-WebRequest` / built-in compression)
- A reachable Send server — set `SEND_SERVER_URL` (or pass `--server`)

Run `/send --doctor` and it'll tell you exactly what's missing.

## Install

The skill is a self-contained folder — copy it into your agent's skills directory:

```sh
cp -R skill/send ~/.claude/skills/send       # Claude Code
# …or .opencode/skills/, .cursor/skills/, .agents/skills/, etc.
```

`SKILL.md` is the open Agent Skills standard, so the *same folder* installs
unmodified on Claude Code, Cursor, OpenCode, Codex CLI, Gemini CLI, Kiro, Goose and
more — only the destination dir differs. See **[INSTALL.md](INSTALL.md)** for the
full per-agent matrix and a multi-agent symlink setup. No binaries, no build step,
no server files come along for the ride.

## One thing to remember

🔐 **The full link is a secret.** Anyone holding the complete URL — including the
part after `#` — can decrypt the context. Share it like a password, and prefer
one-time links. The skill will never claim more than that.

---

Part of [Archcore Send](../../README.md). Architecture & specs live in
[`.archcore/`](../../.archcore/).
