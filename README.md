# Archcore Send

End-to-end-encrypted handoff of AI coding-session context. Package the working
context of one session — goal, state, errors, diffs — encrypt it locally, and
hand a one-time link to the next session or teammate. The server only ever sees
ciphertext.

This is a **monorepo** with two independently-packaged deliverables:

| Part | What | Install |
|---|---|---|
| [`skill/send/`](skill/send/) | The **`/send` Agent Skill** (client) — `SKILL.md` + shell/PowerShell scripts. No binaries. | copy the folder into your agent's skills dir |
| `server/` | The Send server (Go, zero-knowledge ciphertext rendezvous). | *not yet in this repo — separate task* |

The skill is self-contained: copying `skill/send/` carries no server or Go files.

## Quick start (skill)

```text
/send                 # package current context → encrypted one-time link
/send --load <url>    # load a received link into this session
/send --doctor        # check age/curl/gzip + server reachability
```

See **[skill/send/README.md](skill/send/README.md)** for the full client guide.

## Development

```sh
make lint        # shellcheck the skill scripts
make test        # bats unit + e2e (e2e uses a python mock server)
```

## Architecture & decisions

All architecture, decisions, specs and rules live in [`.archcore/`](.archcore/)
(Git-native context). Start with `architecture-overview.doc.md` and the specs
under `.archcore/specs/`.

## License

[Apache 2.0](LICENSE).
