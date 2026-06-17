---
title: "Monorepo layout & lightweight skill packaging"
status: accepted
---

## Context

Archcore Send has two deliverables that ship and install very differently:

- the **skill** (client) — a portable Agent Skill consumed by coding agents (Claude Code, OpenCode, Codex/Gemini), copied into an agent's skills directory;
- the **server** — a Go binary/container an operator self-hosts.

Open question: keep them in **one repository**, and if so, does installing the skill drag along server/Go files? The skill must stay maximally lightweight and **binary-free** ([[age-dependency-no-bundled-binary]]); **Go concerns the server only**.

## Decision

Use a **single repository** ("monorepo") with the skill and server as **separate, self-contained top-level subtrees**. The skill subtree contains only `SKILL.md`, shell/PowerShell scripts, references, and assets — no binaries, no Go, no build artifacts.

```text
archcore-send/                 # repo root
├── skill/
│   └── send/                  # THE installable skill — self-contained
│       ├── SKILL.md
│       ├── scripts/{send.sh,send.ps1}
│       ├── references/{format.md,security.md,backend.md}
│       └── assets/send.schema.json
├── server/                    # Go server — never installed into agents
│   ├── go.mod  main.go        # version/commit vars via ldflags
│   ├── cmd/sendd/
│   └── internal/{api,store,gc,config}/
├── install.sh / install.ps1   # SERVER binary installer (mirrors Archcore CLI)
├── .goreleaser.yaml           # SERVER release (CGO_ENABLED=0, cross-platform)
├── .archcore/                 # this context base
└── README.md  LICENSE
```

**Skill installation targets only `skill/send/`** — never the repo root. All install methods copy/publish just that subtree:

- copy `skill/send/` into the agent's skills path (`~/.claude/skills/`, `.opencode/skills/`, `.agents/skills/`, …);
- publish the `skill/send/` subtree to a skill registry (it stores only the skill);
- a sparse-checkout / `degit`-style fetch of the subpath;
- an optional thin helper that copies the subtree and writes `SEND_SERVER_URL`.

Because the skill is a self-contained subtree with **relative references only inside itself** (scripts never reach into `../server`), copying it carries **zero** server/Go files.

## Alternatives

- **Two separate repos**: cleanest isolation, but splits one product across two histories/trackers/release cadences and duplicates docs + CI. Rejected — the monorepo with isolated subtrees gives the same install isolation without the overhead.
- **Skill at repo root** (`SKILL.md` at top): "copy the repo" would install everything and pollute the skill with server files. Rejected.
- **Bundle the server binary in the skill**: reintroduces a binary, breaking [[age-dependency-no-bundled-binary]] and the audit story. Rejected.

## Consequences

- The skill stays tiny and auditable; an installed skill directory provably contains no binaries ([[skill-contract]] INV-4 + conformance).
- One repo, one README, one `.archcore/`, coordinated versioning — but **independent packaging**: skill = copy a folder; server = `go` / Docker / GoReleaser ([[go-server-conventions]], [[self-host-deploy]]).
- Exactly one `SKILL.md`, under `skill/send/`, to avoid agent skill-discovery confusion. Scripts resolve paths relative to the skill root, never the repo root.
- CI runs Go tests on `server/` and shell lint/tests on `skill/` independently ([[testing]]).
- The distribution/install step MUST package the `skill/send/` subtree only — never `git archive` the whole repo as "the skill".