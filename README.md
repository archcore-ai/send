# Archcore Send

**Hand off an AI coding session — encrypted, one-time, zero-knowledge.**

Package the working context of one session — goal, state, errors, diffs — encrypt
it locally with [`age`](https://age-encryption.org), upload only ciphertext, and get
a one-time link. The other side picks up exactly where you left off — on another
machine, another model, or with a teammate. The server only ever sees ciphertext.

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-11100e?style=flat-square)](LICENSE)
[![Agent Skill](https://img.shields.io/badge/Agent_Skill-open_standard-2E6B45?style=flat-square)](skill/send/INSTALL.md)
[![Go](https://img.shields.io/badge/server-Go-00ADD8?style=flat-square&logo=go&logoColor=white)](server/)
[![Zero-knowledge](https://img.shields.io/badge/server-zero--knowledge-11100e?style=flat-square)](#why-its-different)

You're three hours into debugging with one agent. Now you want to continue on
another machine, with another model, or with a teammate — *without* re-explaining
everything or pasting secrets into a chat box. That's what `/send` is for.

## How it works

One round trip. You run `/send`, hand over the link, the other side loads it.

```text
# you — package the current session into an encrypted one-time link
/send                 →  🔗 https://send.archcore.ai/s/snd_01J…#agekey=AGE-SECRET-KEY-…

# them — load the link, resume with full context
/send --load <link>   →  ✅ Loaded "Auth debugging handoff" — compact-first.
```

The decryption key lives in the link's `#agekey=…` fragment and is parsed **locally**
by the client — it never touches the network. Load is **compact-first**: a tight,
high-signal context (~8k tokens) by default; big diffs and logs stay as optional,
lazy-loaded details.

🔐 **The full link is a secret.** Anyone holding the complete URL — including the part
after `#` — can decrypt the context. Share it like a password, prefer one-time links,
and don't paste it into untrusted remote tools.

## Install the skill

The thing you install is the **skill** — one self-contained folder. No build step, no
binaries, fully auditable. Two steps:

**1. Copy the folder into your agent's skills directory.**

```sh
cp -R skill/send ~/.claude/skills/send     # Claude Code
cp -R skill/send .agents/skills/send       # Cursor · Codex · Gemini · Zed · opencode · …
```

`SKILL.md` is the open **Agent Skills** standard, so the *same folder* installs
unmodified across Claude Code, Cursor, GitHub Copilot, Codex CLI, Gemini CLI, Zed,
opencode, Windsurf, Cline, and more — only the destination dir differs. The full
per-agent matrix is in **[skill/send/INSTALL.md](skill/send/INSTALL.md)**.

**2. Check it works.**

By default the skill talks to the free public instance `https://send.archcore.ai` —
zero config. It's zero-knowledge (the server only ever stores ciphertext, never your
context or keys), so it's safe to use as-is:

```sh
/send --doctor   # checks age / curl / gzip + server reachability
```

That's it. Run `/send`. Prefer your own server? See [Self-hosted](#self-hosted).

No skill support in your tool? Run `skill/send/scripts/send.sh` directly — the script
is the whole product.

## Why it's different

- 🔒 **Zero-knowledge server.** It stores only ciphertext + operational metadata —
  no plaintext, no titles, no key. (It can't redact what it can't read; the trust
  boundary is small and auditable on purpose.)
- 🗝️ **The key lives in the link.** End-to-end encryption with [`age`](https://age-encryption.org);
  the decryption key rides in the URL fragment and **never** reaches the server.
- 🎯 **Compact-first load.** A tight, high-signal context by default; large diffs and
  logs stay as optional, lazy-loaded details.
- 🧹 **No binaries, fully auditable.** Just `SKILL.md` + a shell/PowerShell script
  orchestrating `age`, `gzip`, and `curl`. Read every line before you run it.

## Quick start

```text
/send                 # package current context → encrypted one-time link
/send --load <url>    # load a received link into this session
/send --inspect       # preview what WOULD be sent — uploads nothing
/send --doctor        # check age/curl/gzip + server reachability
```

Useful flags: `--ttl 24h` (max `7d`) · `--one-time` (default) · `--yes` ·
`--allow-secrets` · `--include-large` · `--server <url>`.

See **[skill/send/README.md](skill/send/README.md)** for the full client guide.

## Self-hosted

The public `send.archcore.ai` is the zero-config default, but self-hosting is a
first-class path. Two parts: run the server, then point the client at it.

**1. Run the server.** Any VPS with a public IP and Docker — the design is provider-
and storage-agnostic. From the repo:

```sh
cp deploy/.env.example deploy/.env    # set SEND_DOMAIN + SEND_PUBLIC_URL
cd deploy && docker compose up -d --build
```

Caddy fronts `sendd` with automatic TLS. Full guide: **[deploy/README.md](deploy/README.md)**
and `.archcore/guides/self-host-deploy.guide.md`.

**2. Point the client at it.** Override the built-in default with `SEND_SERVER_URL`
(persistent) or `--server` (per call):

```sh
export SEND_SERVER_URL="https://send.example.com"   # or: /send --server https://send.example.com
/send --doctor                                       # confirm it's reachable
```

## Repository layout

This is a **monorepo** with two independently-packaged deliverables:

| Part | What | Install |
|---|---|---|
| [`skill/send/`](skill/send/) | The **`/send` Agent Skill** (client) — `SKILL.md` + shell/PowerShell scripts. No binaries. | copy the folder into your agent's skills dir |
| [`server/`](server/) | The Send server (Go, zero-knowledge ciphertext rendezvous) — single static binary / container. | self-host via [`deploy/`](deploy/) |

The skill is self-contained: copying `skill/send/` carries no server or Go files.

## Development

```sh
make lint          # shellcheck the skill scripts
make test          # bats unit + e2e (e2e uses a python mock server)
make server-test   # go test ./... -race
make test-all      # skill + server + e2e against the real Go server
```

## Architecture & decisions

All architecture, decisions, specs and rules live in [`.archcore/`](.archcore/)
(Git-native context). Start with `architecture-overview.doc.md` and the specs under
`.archcore/specs/`.

## License

[Apache 2.0](LICENSE).

---

Source: **[github.com/archcore-ai/send](https://github.com/archcore-ai/send)** ·
Built by **[@ivklgn](https://github.com/ivklgn)**.
