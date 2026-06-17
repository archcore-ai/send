---
title: "age as the only crypto dependency; no bundled binary"
status: accepted
---

## Context

A privacy tool that sees plaintext **before** encryption must be especially trustworthy. Bundled binaries and runtime stacks create supply-chain friction and reduce auditability — exactly what security-conscious teams reject.

## Decision

The client depends on exactly one external crypto tool — **`age`** (user-installed) — plus standard `curl`/`gzip` where available. Hard constraints:

- **no bundled binary** in the skill package;
- **no Python/Node runtime** requirement;
- no `curl | sh`, no auto-update, no dynamic remote code loading, no automatic dependency install;
- scripts are short, readable `sh` / `ps1`; they do not read arbitrary project files on their own — all content is assembled explicitly by the agent.

## Alternatives

- **Bundle `age`**: an opaque blob inside the skill; fails the "user can audit everything" goal. Rejected.
- **Implement crypto in-script or via a language runtime**: larger surface and runtime assumptions. Rejected.

## Consequences

- `/send --doctor` must detect and guide `age` installation per-OS ([[skill-contract]], [[error-catalog]]).
- Users/enterprises can pin the skill version, audit scripts, and install `age` via internal channels.
- Portability cost: Windows needs `age.exe`, handled in `send.ps1` ([[skill-implementation]]).
- The skill contains **no compiled code at all** — Go is confined to the server ([[repo-layout-and-skill-packaging]]).
- Reinforces the auditable trust story of [[security-privacy]].