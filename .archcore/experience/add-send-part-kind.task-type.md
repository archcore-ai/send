---
title: "Add a new send part kind"
status: draft
---

## What

Add a new part kind (e.g. `evidence.coverage`, `detail.trace`) end-to-end across manifest, caps, skill packaging, and load behavior — without weakening zero-knowledge or compact-first loading.

## When

- A recurring evidence/detail category emerges (coverage reports, profiling traces, screenshots-as-attachments later).
- You need different default-load or size behavior for a content class.

## Steps

1. Classify: required + `load_by_default` (evidence) vs optional/lazy (detail). Default large/raw → `detail.*` ([[content-policy]], [[size-limits]]).
2. Extend the `kind` enum + private-manifest handling in [[send-format]] (transport id stays opaque `part_NNNN`).
3. Update the skill's send-mode instructions to populate the new part, and the preview to disclose it ([[skill-implementation]], [[cli-contract]] UX-1).
4. Set caps/behavior in [[size-limits]] if they differ.
5. Ensure load-mode honors `load_by_default`; lazy parts load only via `--load-detail`.
6. The **server needs no change** — it stores opaque bytes ([[zero-knowledge-backend]]). If you feel tempted to special-case the kind server-side, stop: that violates zero-knowledge.
7. Add tests: manifest round-trip; default-load excludes the new lazy kind; preview lists it; the secret scan still runs.

## Example

Add `detail.trace` (CPU profile): kind `binary`, optional, not load-by-default, single-detail cap from [[size-limits]]; the skill writes `details/trace.pb` and the manifest maps it to `part_0004`; load lists it under `available_details`, never auto-injected.

## Pitfalls

- Inlining a large kind into `compact` (context-window blowout) — keep it a `detail.*`.
- Adding a semantic header/route server-side — breaks [[zero-knowledge-backend]].
- Skipping the preview disclosure — violates [[content-policy]] R5.
- New binary kinds bypassing the secret scan — the scan still applies pre-encryption ([[content-policy]]).