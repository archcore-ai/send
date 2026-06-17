# Security & privacy — reference

Mirrors `.archcore/rules/security-privacy.rule.md` and `content-policy.rule.md`.

## Non-negotiable rules

- **Local crypto only.** Encryption/decryption happen on the sender/recipient
  machine. The server receives only ciphertext + operational metadata.
- **The fragment never leaves the client.** `#agekey=…` (or `#k=…`) is parsed
  locally and must **not** appear in any HTTP request, redirect, log, telemetry,
  or argv passed to a remote tool. This forbids a remote-MCP "load" path — load
  is always local script logic.
- **Temp plaintext hygiene.** Plaintext/identity temp files live in the OS temp
  dir (never under the project root), are created `0600`, and are deleted on
  success **and** failure. Never printed, never produced under `set -x`.
- **Key not persisted.** The ephemeral `age` identity is never written to
  persistent disk beyond the short-lived temp file needed to decrypt.
- **No secret values in logs.** The secret scanner logs counts/types only.

## Secret scan (runs before encryption)

A local regex scan blocks upload by default on high-confidence matches
(private keys, AWS keys, `*_API_KEY=…`, GitHub/Slack tokens, JWTs, DB URIs).
Override only with explicit `--allow-secrets` (which logs counts, never values).

Default exclusions (don't package these): `node_modules/ dist/ build/ .git/`,
lockfiles, minified/map files, images/archives/binaries, `.env`, `*.pem`,
`*.key`, `id_rsa`.

## What you MAY claim

- The server does not receive plaintext content.
- The server does not receive the `age` key / URL fragment.
- Encryption/decryption are local; the server stores ciphertext + operational
  metadata only.

## What you MUST NOT claim

- That the AI provider can't see the context (agents see plaintext by design).
- That one-time prevents copying or screenshots.
- That the server can redact encrypted data.
- That a full link (with fragment) is safe to paste into untrusted remote tools.

## The one real residual risk

**Anyone with the full URL including the fragment can decrypt.** Treat the link
like a secret. One-time redemption means *one redemption session* (a ~10-minute
grant window after first open), not one HTTP response.
