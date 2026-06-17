# Archcore Send

End-to-end-encrypted handoff of AI coding-session context. Package the working
context of one session — goal, state, errors, diffs — encrypt it locally, and
hand a one-time link to the next session or teammate. The server only ever sees
ciphertext.

This is a **monorepo** with two independently-packaged deliverables:

| Part | What | Install |
|---|---|---|
| [`skill/send/`](skill/send/) | The **`/send` Agent Skill** (client) — `SKILL.md` + shell/PowerShell scripts. No binaries. | copy the folder into your agent's skills dir |
| [`server/`](server/) | The Send server (Go, zero-knowledge ciphertext rendezvous) — single static binary / container. | self-host via [`deploy/`](deploy/) |

The skill is self-contained: copying `skill/send/` carries no server or Go files.

## Install the skill

It's one self-contained folder — no build step, no binaries. Two steps:

**1. Copy the folder into your agent's skills directory.**

```sh
cp -R skill/send ~/.claude/skills/send     # Claude Code
cp -R skill/send .agents/skills/send       # Cursor · Codex · Gemini · Zed · opencode · …
```

**2. Point it at a server and check it's ready.**

Don't want to deploy anything? Use the free public instance. It's
zero-knowledge — the server only ever stores ciphertext, never your context or
keys — so it's safe to use as-is:

```sh
export SEND_SERVER_URL="https://v864364.hosted-by-vdsina.com"   # or pass --server
/send --doctor                                                  # tells you anything that's missing
```

That's it. Run `/send`.

Prefer your own? [Self-host the server](#self-host-the-server) and point
`SEND_SERVER_URL` at it instead.

Using a different agent? Every tool's exact directory is in
**[skill/send/INSTALL.md](skill/send/INSTALL.md)**. No skill support at all? Run
`skill/send/scripts/send.sh` directly — the script is the whole product.

## Quick start (skill)

```text
/send                 # package current context → encrypted one-time link
/send --load <url>    # load a received link into this session
/send --doctor        # check age/curl/gzip + server reachability
```

See **[skill/send/README.md](skill/send/README.md)** for the full client guide.

## Self-host the server

Any VPS with a public IP and Docker — the design is provider- and storage-agnostic.
From the repo:

```sh
cp deploy/.env.example deploy/.env    # set SEND_DOMAIN + SEND_PUBLIC_URL
cd deploy && docker compose up -d --build
```

Caddy fronts `sendd` with automatic TLS. Full guide: **[deploy/README.md](deploy/README.md)**
and `.archcore/guides/self-host-deploy.guide.md`.

## Development

```sh
make lint          # shellcheck the skill scripts
make test          # bats unit + e2e (e2e uses a python mock server)
make server-test   # go test ./... -race
make test-all      # skill + server + e2e against the real Go server
```

## Architecture & decisions

All architecture, decisions, specs and rules live in [`.archcore/`](.archcore/)
(Git-native context). Start with `architecture-overview.doc.md` and the specs
under `.archcore/specs/`.

## License

[Apache 2.0](LICENSE).
