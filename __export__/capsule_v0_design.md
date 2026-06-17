# Capsule v0: E2EE Context Handoff Skill

**Status:** v0 design context  
**Date:** 2026-06-11  
**Primary use case:** transfer working context between AI coding-agent sessions without sending plaintext to the backend.  
**Target UX:** one skill, two operations: `/capsule --send` and `/capsule --load <url>`.

---

## 0. Executive summary

Capsule v0 is a lightweight context handoff mechanism for AI coding agents. The user working in one agent session can package the current useful context into an encrypted capsule and share a one-time link with another developer or another agent session.

The proposed v0 deliberately avoids a heavy system:

```text
one skill
SKILL.md
scripts/capsule.sh
scripts/capsule.ps1
dependency: age
no bundled binary
no Python/Node requirement
remote server sees ciphertext only
```

The core trust model is:

```text
Agent creates plaintext context locally
scripts validate / compress / encrypt locally using age
backend stores ciphertext only
recipient downloads ciphertext
scripts decrypt locally using age
recipient agent imports compact context
```

The backend should start as an encrypted object/rendezvous service, not a full collaborative sync system. Durable Streams are a good future storage primitive for chunked, live, forked, or appendable capsules, but are not required for sealed v0 capsules.

The most important product principle: **do not share raw session transcript by default**. Share a structured working context: compact summary, decisions, relevant files, diffs, errors, open questions, and optional evidence/details.

---

## 1. Product goal

### 1.1 Problem

AI coding sessions accumulate useful context:

- what the task is;
- what was already tried;
- current hypothesis;
- relevant files;
- current diff;
- error messages;
- decisions already made;
- open questions;
- constraints and next steps.

When another developer or another agent needs to continue, the common options are bad:

- manually summarize the chat;
- paste a long transcript;
- send screenshots;
- share full logs/diffs manually;
- re-run investigation from scratch.

Capsule v0 solves the handoff problem with a lightweight encrypted package.

### 1.2 Desired UX

Sender:

```text
/capsule --send
```

or:

```text
/capsule --send --ttl 24h --one-time
```

Result:

```text
Capsule created:
https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...

Included:
- compact context
- decisions
- relevant files
- diff summary
- open questions

Expires: 24h
One-time: yes
```

Receiver:

```text
/capsule --load https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...
```

Result:

```text
Loaded capsule: Auth debugging handoff

Imported:
- current goal
- current hypothesis
- relevant files
- decisions
- open questions

Available details:
- full diff
- test output
- selected logs
```

The recipient agent should then continue from this compact working context, not from a raw transcript dump.

---

## 2. Non-goals for v0

Capsule v0 should not attempt to be all of these at once:

- full collaborative workspace;
- permanent team memory;
- live shared session;
- multi-user CRDT system;
- GitHub/GitLab replacement;
- issue tracker;
- database sync engine;
- agent execution broker;
- full transcript recorder;
- automatic background daemon;
- remote MCP service that receives plaintext.

v0 should be small enough that a user can understand the entire trust boundary.

---

## 3. Finalized v0 direction

### 3.1 Packaging

Use one portable Agent Skill:

```text
capsule/
├── SKILL.md
├── scripts/
│   ├── capsule.sh
│   └── capsule.ps1
├── references/
│   ├── format.md
│   ├── security.md
│   └── backend.md
└── assets/
    └── capsule.schema.json        # optional
```

Rationale:

- one skill is easy to audit;
- no hidden binary in the skill package;
- no Python/Node runtime requirement;
- scripts are readable;
- crypto is delegated to `age`, a known external tool;
- the skill remains portable across Claude Code, OpenCode, Codex/Gemini-style agents that understand `SKILL.md`.

### 3.2 Runtime dependency

The only required runtime tool for v0 should be:

```text
age
```

Optional external tools:

```text
curl      # Unix HTTP client
gzip      # compression, usually present on Unix
tar       # optional if archive mode is used
```

On Windows, PowerShell can do HTTP and compression, but `age.exe` still needs to be installed.

### 3.3 Why not bundled binary

Bundled binaries inside skills create trust friction:

- users cannot easily inspect binaries;
- a privacy tool seeing plaintext before encryption must be especially trustworthy;
- binary-in-skill feels like a supply-chain artifact, not a simple instruction package;
- enterprise/security-conscious teams may reject hidden executables.

v0 therefore chooses visible shell/PowerShell scripts and an explicit external crypto tool.

### 3.4 Why not Python/Node for v0

Python or Node are reasonable implementations, but they introduce runtime assumptions and package/dependency questions. Python stdlib alone does not provide a convenient modern AEAD primitive such as AES-GCM/ChaCha20-Poly1305. Node/WebCrypto is plausible but creates a JS/npm perception problem. Shell + `age` keeps the skill readable and delegates crypto to a narrow, purpose-built tool.

---

## 4. Relevant ecosystem context

### 4.1 Agent Skills format

Agent Skills are a lightweight format where a skill is a folder containing `SKILL.md` plus optional scripts, references, assets, and other resources. The standard uses progressive disclosure: agents load skill metadata first, full instructions only when activated, and scripts/references only as needed.

Useful references:

- Agent Skills overview: https://agentskills.io/home
- Agent Skills specification: https://agentskills.io/specification
- Claude Code skills docs: https://code.claude.com/docs/en/skills

### 4.2 Claude Code fit

Claude Code supports skills as directories with `SKILL.md`. A skill can be invoked directly with `/skill-name`. Claude Code also supports supporting files and skill arguments. For this project, a folder named `capsule` should map naturally to:

```text
/capsule --send
/capsule --load <url>
```

The skill can tell Claude to run scripts relative to the skill root.

### 4.3 OpenCode fit

OpenCode supports `SKILL.md` skills loaded on demand via its native skill tool. It discovers skills in multiple locations, including `.opencode/skills`, `.claude/skills`, and `.agents/skills` style paths.

For exact `/capsule` slash-command UX in OpenCode, an optional thin command shim may still be useful:

```text
.opencode/commands/capsule.md
```

The shim should simply instruct OpenCode to use the `capsule` skill with the provided arguments. The source of truth remains `capsule/SKILL.md`.

### 4.4 age fit

`age` is a simple modern file encryption tool and format. It has explicit keys and Unix-style composability. This matches the v0 design: scripts produce a local file/stream and call `age` to encrypt/decrypt it before/after backend transfer.

Useful references:

- age GitHub repository: https://github.com/FiloSottile/age
- age format specification: https://age-encryption.org/v1

### 4.5 Durable Streams fit

Durable Streams provide HTTP-addressable append-only byte streams with durable replay, offsets, catch-up reads, live tailing, and explicit closure. That is highly relevant for future live/fork/progressive capsule modes, but v0 can avoid the complexity.

Useful references:

- Durable Streams protocol overview: https://durable-streams-durable-streams.mintlify.app/concepts/protocol-overview
- Durable Streams protocol draft: https://github.com/durable-streams/durable-streams/blob/main/PROTOCOL.md

---

## 5. Threat model

### 5.1 Actors

- **Sender user:** creates the capsule.
- **Sender agent:** sees the plaintext context because it creates the capsule.
- **Local capsule scripts:** see plaintext before encryption and after decryption.
- **age binary:** performs encryption/decryption locally.
- **Remote backend:** stores and serves ciphertext.
- **Recipient user:** receives and loads the capsule.
- **Recipient agent:** sees plaintext after local decryption.
- **Network observer:** sees HTTPS traffic and maybe backend metadata, not plaintext capsule.

### 5.2 Assets to protect

- source code excerpts;
- current diff;
- internal architecture notes;
- error logs;
- stack traces;
- customer/internal references;
- private task context;
- secrets accidentally captured in logs or diffs;
- decryption key embedded in URL fragment;
- plaintext capsule on disk during temporary processing.

### 5.3 Explicit guarantees

Capsule v0 should honestly claim:

```text
The backend does not receive plaintext capsule content.
The backend does not receive the age private key / link key when scripts parse the fragment locally.
Encryption and decryption happen locally.
The remote service stores only ciphertext and operational metadata.
```

### 5.4 Non-guarantees

Capsule v0 should not claim:

```text
The AI provider cannot see the context.
The sender/recipient cannot copy plaintext.
One-time links prevent screenshots/copying after decryption.
The backend can redact encrypted data.
A link-key URL is safe if pasted into untrusted remote tools.
```

Important limitation: if the user pastes the full URL, including `#agekey=...`, into a remote service/tool that forwards it to the backend or a third party, the key can leak. The local scripts must parse the fragment and never send it to the backend.

### 5.5 Key privacy

The share URL can use a fragment:

```text
https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...
```

HTTP clients do not normally send URL fragments to servers. The scripts must preserve this model by extracting the key locally and making backend requests only to paths like:

```text
/v1/capsules/cap_01J...
```

### 5.6 Temporary plaintext handling

The sender workflow needs temporary plaintext files unless everything is piped. v0 should minimize risk:

- use OS temp directories;
- create files with restrictive permissions where possible;
- delete temp files on success/failure;
- do not store plaintext under project root;
- avoid printing plaintext capsule in logs;
- avoid shell tracing (`set -x`) around secrets and plaintext;
- avoid writing decryption keys to persistent disk unless necessary.

---

## 6. Data model: what is a capsule?

A capsule is not a raw transcript. It is a structured handoff package optimized for another agent/developer to continue work.

### 6.1 Logical parts

```text
capsule
├── manifest
├── compact context
├── required evidence
├── optional details
└── metadata / checksums
```

### 6.2 Compact context

This is the only part that should be loaded into the recipient agent by default.

Recommended content:

- task title;
- goal;
- current state;
- current hypothesis;
- relevant files;
- current branch/commit if in a git repo;
- decisions already made;
- what was tried;
- important errors;
- open questions;
- suggested next actions;
- explicit exclusions/redactions.

Target size:

```text
30–50 KB plaintext maximum
roughly 2k–8k tokens
```

### 6.3 Required evidence

Evidence is not full raw data. It contains small supporting facts:

- selected stack traces;
- selected log excerpts;
- important diff hunks;
- test failure summary;
- command outputs summarized or excerpted;
- line references or file paths.

Target size:

```text
300–800 KB plaintext maximum
```

### 6.4 Optional details

Optional details are lazy-loaded only when needed:

- full diff;
- larger logs;
- full command outputs;
- larger file excerpts;
- generated artifacts;
- trace files;
- screenshots, if supported later.

These should not be automatically inserted into the recipient agent context.

### 6.5 What should never be included by default

- `.env` contents;
- API keys;
- OAuth/JWT/private tokens;
- private keys;
- credentials;
- unrelated chat transcript;
- hidden chain-of-thought / private reasoning traces;
- full customer PII;
- full raw logs unless explicitly requested;
- generated/minified/lock files unless explicitly relevant.

---

## 7. Capsule size model

### 7.1 Expected sizes

Text compresses well. Diffs and logs compress well unless they contain high-entropy data. Images/binaries do not compress much.

Approximate practical ranges:

| Capsule kind | Raw plaintext | After gzip | After age | UX |
|---|---:|---:|---:|---|
| Tiny handoff | 20–100 KB | 5–30 KB | + small overhead | instant |
| Normal coding handoff | 200 KB–1 MB | 50–300 KB | similar | good |
| Heavy debugging | 2–10 MB | 500 KB–3 MB | similar | acceptable |
| Logs/diffs/artifacts | 20–100 MB | 5–40 MB | similar | staged required |
| Images/binaries | 1–100 MB | little gain | similar | optional attachment only |

### 7.2 Main bottleneck

Network upload is not the only problem. The bigger risk is loading too much plaintext into the recipient agent context.

Bad:

```text
/capsule --load <url>
→ decrypt 20 MB
→ print everything into the agent context
```

Good:

```text
/capsule --load <url>
→ load manifest + compact context + required evidence
→ list optional details
→ lazy-load details only when requested
```

### 7.3 Default limits

Recommended v0 defaults:

```text
compact.md:
  soft cap: 30 KB
  hard cap: 50 KB

required evidence:
  soft cap: 300 KB
  hard cap: 800 KB

single optional detail:
  soft cap: 10 MB
  hard cap: 50 MB

total capsule:
  soft cap: 10 MB encrypted
  hard cap: 25 MB encrypted for v0
```

If a capsule exceeds the soft cap, show a preview and require explicit confirmation. If it exceeds the hard cap, reject or require explicit flags.

### 7.4 Large file exclusions

Default exclusions:

```text
node_modules/
dist/
build/
coverage/
.git/
*.lock
package-lock.json
pnpm-lock.yaml
yarn.lock
*.min.js
*.map
*.png
*.jpg
*.jpeg
*.gif
*.pdf
*.zip
*.tar
*.gz
*.sqlite
*.db
```

These can still be referenced by path/hash, but not inlined by default.

---

## 8. Crypto model

### 8.1 v0 link-key mode

For the simplest v0, generate an age identity for each capsule.

Sender script:

```text
age-keygen → temp identity file
extract public recipient
compress part
age -r <recipient> → encrypted part
put private identity in URL fragment
upload encrypted parts
```

Recipient script:

```text
parse age private identity from URL fragment
write it to temp identity file with restrictive permissions
download encrypted part
age --decrypt -i temp identity file
remove temp identity file
```

### 8.2 URL shape

```text
https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...
```

or if URL length becomes problematic:

```text
https://ctx.example.com/c/cap_01J...#k=<base64url-wrapped-identity>
```

The fragment must never be sent to the backend.

### 8.3 Recipient-key mode later

Future mode:

```text
/capsule --send --to bob
```

Then Bob has a persistent public age recipient key, and the capsule is encrypted to Bob’s public key. The URL no longer carries the private decryption key.

Pros:

- forwarding the URL alone is insufficient;
- better for teams;
- better enterprise story.

Cons:

- requires key management;
- requires identity setup;
- harder cross-agent onboarding.

Not v0 unless the team already has public keys.

### 8.4 Compression before encryption

Always compress before encryption:

```text
plaintext → gzip → age → ciphertext
```

Do not compress ciphertext.

### 8.5 Per-part encryption

For staged loading, do not encrypt one huge archive as a single age file if the recipient needs to load only compact context first. Encrypt each part independently:

```text
manifest.json → gzip → age → manifest.age
compact.md    → gzip → age → compact.age
evidence.md   → gzip → age → evidence.age
full-diff      → gzip → age → detail.full-diff.age
```

All parts can be encrypted to the same generated age recipient.

---

## 9. Backend v0

### 9.1 Backend purpose

The backend is a ciphertext rendezvous service:

```text
POST encrypted capsule parts
GET encrypted capsule parts
TTL
one-time redemption
rate limiting
cleanup
```

It must not process plaintext.

### 9.2 Backend must not do

- summarization;
- redaction;
- semantic indexing;
- plaintext validation;
- diff parsing;
- AI processing;
- secret scanning;
- decryption;
- key storage.

All plaintext operations happen locally before encryption or after decryption.

### 9.3 Minimal backend components

For self-host/team v0:

```text
HTTP API
SQLite metadata DB
filesystem object storage
cleanup job
```

For hosted v0:

```text
HTTP API
Postgres metadata DB
S3/R2/GCS object storage
cleanup worker
rate limiting / abuse controls
```

### 9.4 Metadata schema

Example SQL:

```sql
CREATE TABLE capsules (
  id TEXT PRIMARY KEY,
  version TEXT NOT NULL,
  one_time BOOLEAN NOT NULL DEFAULT true,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  finalized_at TIMESTAMPTZ,
  total_encrypted_bytes BIGINT NOT NULL DEFAULT 0,
  part_count INTEGER NOT NULL DEFAULT 0,
  uploader_ip_hash TEXT,
  user_agent_hash TEXT
);

CREATE TABLE capsule_parts (
  capsule_id TEXT NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  part_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  required BOOLEAN NOT NULL DEFAULT false,
  storage_key TEXT NOT NULL,
  encrypted_size BIGINT NOT NULL,
  ciphertext_sha256 TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (capsule_id, part_id)
);
```

### 9.5 API shape

#### Create capsule

```http
POST /v1/capsules
Content-Type: application/json
```

Request:

```json
{
  "version": "capsule.v1",
  "ttl_seconds": 86400,
  "one_time": true,
  "part_count_expected": 3
}
```

Response:

```json
{
  "id": "cap_01J...",
  "upload_base_url": "https://ctx.example.com/v1/capsules/cap_01J.../parts",
  "public_url": "https://ctx.example.com/c/cap_01J...",
  "expires_at": "2026-06-12T12:00:00Z"
}
```

#### Upload part

```http
PUT /v1/capsules/{id}/parts/{part_id}
Content-Type: application/octet-stream
X-Capsule-Part-Kind: compact
X-Capsule-Part-Required: true
X-Capsule-Ciphertext-Sha256: ...

<encrypted bytes>
```

Response:

```json
{
  "ok": true,
  "part_id": "compact",
  "encrypted_size": 28412
}
```

#### Finalize capsule

```http
POST /v1/capsules/{id}/finalize
```

Response:

```json
{
  "ok": true,
  "public_url": "https://ctx.example.com/c/cap_01J...",
  "expires_at": "2026-06-12T12:00:00Z"
}
```

The script adds the key fragment locally.

#### Get capsule manifest metadata

```http
GET /v1/capsules/{id}
```

This returns only backend metadata, not plaintext:

```json
{
  "id": "cap_01J...",
  "status": "finalized",
  "expires_at": "2026-06-12T12:00:00Z",
  "one_time": true,
  "parts": [
    { "part_id": "manifest", "kind": "manifest", "required": true, "encrypted_size": 1200 },
    { "part_id": "compact", "kind": "compact", "required": true, "encrypted_size": 28412 },
    { "part_id": "detail.full-diff", "kind": "diff", "required": false, "encrypted_size": 2104032 }
  ]
}
```

#### Redeem capsule

For staged load, strict one-time must be handled carefully. Recommended practical model:

```http
POST /v1/capsules/{id}/redeem
```

Response:

```json
{
  "redeem_token": "red_...",
  "expires_in_seconds": 600,
  "parts": [
    { "part_id": "manifest", "download_url": "/v1/capsules/cap_.../parts/manifest?redeem_token=..." },
    { "part_id": "compact", "download_url": "/v1/capsules/cap_.../parts/compact?redeem_token=..." }
  ]
}
```

The first successful redeem marks the capsule as consumed and creates a short-lived download grant. This allows loading `manifest`, `compact`, and selected details without losing staged UX.

#### Download part

```http
GET /v1/capsules/{id}/parts/{part_id}?redeem_token=red_...
```

Response:

```http
200 OK
Content-Type: application/octet-stream

<encrypted bytes>
```

### 9.6 Strict one-time vs practical one-time

Strict one-time:

```text
first GET consumes the capsule forever
```

Pros:

- easy to explain;
- strong one-time semantics.

Cons:

- bad for multi-part staged loading;
- network interruption can burn the link.

Practical one-time:

```text
first redeem consumes the public link
backend issues short-lived download grant for parts
```

Pros:

- better staged loading;
- better UX;
- one public link still only redeemable once.

Cons:

- more backend logic;
- must protect redeem tokens.

Recommended v0: **practical one-time** with short grant, e.g. 10 minutes.

---

## 10. Backend storage options

### 10.1 Filesystem + SQLite

Good for internal alpha:

```text
data/
├── capsules.db
└── blobs/
    └── cap_01J.../
        ├── manifest.age
        ├── compact.age
        └── detail.full-diff.age
```

Pros:

- very simple;
- easy to self-host;
- easy to debug.

Cons:

- no horizontal scale;
- local disk durability assumptions;
- cleanup and backup need care.

### 10.2 Postgres + S3/R2

Good for hosted beta:

```text
Postgres: metadata
S3/R2/GCS: encrypted parts
```

Pros:

- scalable;
- storage lifecycle policies;
- object-level size handling;
- easier deployment on common platforms.

Cons:

- more infra;
- object storage request costs;
- must implement one-time/rate limits in API layer.

### 10.3 Durable Streams

Good later for progressive/live/fork:

```text
capsule stream:
  encrypted manifest event
  encrypted compact event
  encrypted evidence event
  encrypted detail chunk events
  sealed event
```

Pros:

- append-only structure;
- chunked upload/download;
- offsets;
- catch-up reads;
- live tailing later;
- fork/live handoff later.

Cons:

- overkill for sealed v0;
- one-time semantics must still be implemented outside the stream;
- more moving parts;
- deletion/retention model needs attention.

Recommended: do not expose Durable Streams in the public v0 API. Hide storage behind an internal `CapsuleStore` interface.

---

## 11. Internal storage abstraction

Define the backend around an interface like:

```ts
interface CapsuleStore {
  createCapsule(input: CreateCapsuleInput): Promise<CapsuleRecord>
  putPart(capsuleId: string, partId: string, encryptedBytes: Readable, metadata: PartMetadata): Promise<void>
  finalizeCapsule(capsuleId: string): Promise<void>
  redeemCapsule(capsuleId: string): Promise<RedeemGrant>
  getPart(capsuleId: string, partId: string, redeemToken: string): Promise<Readable>
  deleteExpired(now: Date): Promise<number>
}
```

Initial implementations:

```text
FilesystemCapsuleStore
S3CapsuleStore
```

Future implementation:

```text
DurableStreamsCapsuleStore
```

This keeps `/capsule --send` and `/capsule --load` stable even if backend internals change.

---

## 12. Script behavior

### 12.1 `capsule.sh`

Responsibilities:

- parse `--send`, `--load`, `--doctor`, `--inspect`;
- locate `age`, `curl`, `gzip`;
- create temporary files securely;
- run local secret scans;
- compress parts;
- generate per-capsule age keypair;
- encrypt parts;
- upload/download parts;
- decrypt selected parts;
- delete temp files;
- print structured output for the agent.

Non-responsibilities:

- semantic summarization;
- reading arbitrary repo files independently;
- generating capsule content itself;
- AI processing;
- complex JSON mutation if avoidable.

### 12.2 `capsule.ps1`

Same responsibilities as `capsule.sh`, adapted for Windows:

- detect `age.exe`;
- use `Invoke-WebRequest` or `curl.exe` if available;
- handle paths and quoting carefully;
- avoid persistent plaintext files;
- avoid execution policy surprises where possible;
- provide `doctor` diagnostics.

### 12.3 Command modes

```text
capsule.sh doctor
capsule.sh send <capsule-dir-or-json> --ttl 24h --one-time
capsule.sh load <url>
capsule.sh load-detail <url> <part-id>
capsule.sh inspect <capsule-dir-or-json>
```

PowerShell equivalent:

```powershell
./capsule.ps1 doctor
./capsule.ps1 send ./capsule-workdir -Ttl 24h -OneTime
./capsule.ps1 load "https://ctx.example.com/c/cap#agekey=..."
./capsule.ps1 load-detail "https://ctx.example.com/c/cap#agekey=..." "detail.full-diff"
```

### 12.4 Structured output

The scripts should output JSON for machine readability:

`send` output:

```json
{
  "ok": true,
  "url": "https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...",
  "expires_at": "2026-06-12T12:00:00Z",
  "one_time": true,
  "included": ["manifest", "compact", "evidence.errors"],
  "optional_parts": ["detail.full-diff"]
}
```

`load` output:

```json
{
  "ok": true,
  "title": "Auth debugging handoff",
  "compact_context": "...markdown...",
  "required_evidence": [
    { "part_id": "evidence.errors", "content": "..." }
  ],
  "available_details": [
    { "part_id": "detail.full-diff", "kind": "diff", "encrypted_size": 2104032 }
  ]
}
```

The skill then tells the agent how to import this output.

---

## 13. Skill behavior

### 13.1 `SKILL.md` frontmatter draft

```yaml
---
name: capsule
description: Send or load encrypted context capsules for AI coding sessions. Use when the user asks to share, package, hand off, send, import, or load current working context.
argument-hint: "--send | --load <url> | --load-detail <url> <part-id> | --doctor"
disable-model-invocation: true
---
```

`disable-model-invocation: true` is recommended where supported because sending/loading context is a side-effect workflow that should be user-invoked.

### 13.2 Send mode instructions

When invoked as:

```text
/capsule --send
```

The agent should:

1. Build a compact working context.
2. Inspect git state only when useful and permitted.
3. Avoid secrets and unrelated transcript.
4. Prepare a capsule work directory:

```text
/tmp/capsule-XXXX/
├── manifest.json
├── compact.md
├── evidence.errors.md
└── details/
    └── full-diff.patch
```

5. Show a preview to the user unless a `--yes` flag is explicitly provided.
6. Run the local script.
7. Return the URL and brief included-content summary.

### 13.3 Load mode instructions

When invoked as:

```text
/capsule --load <url>
```

The agent should:

1. Run local script with the URL.
2. Import only compact context and required evidence by default.
3. Summarize what was loaded.
4. List optional details.
5. Ask before loading large optional details.
6. Not modify project files until the user confirms.

### 13.4 Load-detail mode

When invoked as:

```text
/capsule --load-detail <url> detail.full-diff
```

The agent should:

1. Load only the requested optional part.
2. Summarize the part if large.
3. Avoid dumping huge logs/diffs into context unless directly needed.

---

## 14. Capsule file format v0

### 14.1 Local plaintext workdir before encryption

```text
capsule-workdir/
├── manifest.json
├── compact.md
├── evidence/
│   ├── errors.md
│   ├── decisions.md
│   └── files.md
└── details/
    ├── full-diff.patch
    └── test-output.log
```

### 14.2 Manifest example

```json
{
  "version": "capsule.v1",
  "title": "Auth debugging handoff",
  "created_at": "2026-06-11T12:00:00Z",
  "source": {
    "agent": "claude-code",
    "repo": "api-service",
    "branch": "fix-auth-cache",
    "commit": "abc123"
  },
  "policy": {
    "raw_transcript_included": false,
    "secrets_included": false,
    "default_load": ["compact", "evidence.errors", "evidence.decisions"]
  },
  "parts": [
    {
      "id": "compact",
      "path": "compact.md",
      "kind": "markdown",
      "required": true,
      "load_by_default": true,
      "plaintext_size": 42000
    },
    {
      "id": "evidence.errors",
      "path": "evidence/errors.md",
      "kind": "markdown",
      "required": true,
      "load_by_default": true,
      "plaintext_size": 18000
    },
    {
      "id": "detail.full-diff",
      "path": "details/full-diff.patch",
      "kind": "patch",
      "required": false,
      "load_by_default": false,
      "plaintext_size": 2100000
    }
  ]
}
```

### 14.3 Compact context template

```md
# Context Capsule: <title>

## Goal

...

## Current state

...

## Current hypothesis

...

## What was already tried

...

## Decisions made

- ...

## Relevant files

- `path/to/file.ts` — why relevant

## Current diff summary

...

## Important errors

...

## Open questions

- ...

## Suggested next steps

1. ...
2. ...

## Exclusions / redactions

- Raw transcript not included.
- Secrets/env vars not included.
- Full logs available only as optional details, if present.
```

---

## 15. Preview and confirmation

Before sending, show a preview:

```text
Capsule preview

Title: Auth debugging handoff
TTL: 24h
One-time: yes

Default load:
- compact.md: 42 KB
- evidence/errors.md: 18 KB
- evidence/decisions.md: 5 KB

Optional details:
- detail.full-diff: 2.1 MB
- detail.test-output: 480 KB

Skipped:
- pnpm-lock.yaml: excluded by default
- coverage/: excluded by default
- .env: excluded by policy

Estimated encrypted upload: ~700 KB
Remote backend receives ciphertext only.

Proceed? y/N
```

The agent should not silently share a capsule unless the user gave an explicit command and any preview policy has been satisfied.

---

## 16. Secret scanning v0

Local scripts can run a lightweight regex scan. This is not a complete DLP system, but useful.

Patterns to detect/block or warn:

- `-----BEGIN PRIVATE KEY-----`;
- `AWS_ACCESS_KEY_ID` / `AKIA...`;
- GitHub tokens `ghp_`, `github_pat_`;
- Slack tokens `xoxb-`, `xoxp-`;
- JWT-like strings `eyJ...`;
- `.env`-style lines with `SECRET=`, `TOKEN=`, `API_KEY=`;
- PEM blocks;
- OAuth refresh tokens;
- database connection strings.

Policy:

```text
block by default if high-confidence secret found
allow override only with explicit --allow-secrets flag
log only counts/types, not secret values
```

Important: secret scanning must happen before encryption because the backend cannot scan ciphertext.

---

## 17. Error handling

### 17.1 Missing age

```json
{
  "ok": false,
  "error_code": "AGE_NOT_FOUND",
  "message": "age is required. Install age and re-run /capsule --doctor.",
  "install_hints": {
    "macos": "brew install age",
    "windows": "winget install FiloSottile.age",
    "linux": "Use your distro package manager or download from the age project."
  }
}
```

### 17.2 Expired capsule

```json
{
  "ok": false,
  "error_code": "CAPSULE_EXPIRED",
  "message": "This capsule has expired. Ask the sender to create a new one."
}
```

### 17.3 Already redeemed

```json
{
  "ok": false,
  "error_code": "CAPSULE_ALREADY_REDEEMED",
  "message": "This one-time capsule has already been redeemed."
}
```

### 17.4 Decryption failed

```json
{
  "ok": false,
  "error_code": "DECRYPTION_FAILED",
  "message": "Could not decrypt the capsule. The URL fragment key may be missing or incorrect."
}
```

### 17.5 Large capsule blocked

```json
{
  "ok": false,
  "error_code": "CAPSULE_TOO_LARGE",
  "message": "Capsule exceeds the v0 size limit. Exclude large details or use explicit --include-large."
}
```

---

## 18. Security and trust posture

### 18.1 Trust story

The user-facing trust story:

```text
Capsule skill contains no hidden binary by default.
Scripts are readable.
The only crypto dependency is age.
Encryption happens locally.
The backend stores ciphertext only.
The decryption key is in the URL fragment and is parsed locally.
No remote MCP service receives plaintext.
No background daemon is required.
```

### 18.2 Supply-chain constraints

- no bundled binary in v0;
- no auto-update;
- no `curl | sh`;
- no dynamic remote code loading;
- no automatic dependency install;
- no reading arbitrary project files inside scripts;
- all context collection is explicit through the agent prompt;
- scripts should be short and auditable.

### 18.3 What to publish

Open-source repository should include:

```text
capsule/SKILL.md
scripts/capsule.sh
scripts/capsule.ps1
references/security.md
references/format.md
backend reference implementation
threat model
examples
```

### 18.4 Enterprise/self-host story

Enterprises can:

- self-host the backend;
- install `age` through internal package management;
- pin the skill version in repo;
- audit scripts;
- configure maximum TTL and size;
- disable public hosted backend;
- enforce recipient-key mode later.

---

## 19. Backend privacy metadata

Even with ciphertext-only storage, metadata can leak information:

- capsule size;
- creation time;
- expiration time;
- download time;
- IP addresses;
- user agents;
- frequency of sharing.

Mitigations:

- minimize logging;
- hash or truncate IPs where possible;
- short retention;
- delete expired capsules and metadata;
- no plaintext title on backend unless encrypted;
- do not store recipient names in v0 link-key mode;
- allow self-hosting.

---

## 20. Durable Streams future path

### 20.1 Why not required for v0

A sealed capsule is naturally an encrypted object or a small set of encrypted parts. Blob/object storage is simpler.

### 20.2 When Durable Streams become useful

Use Durable Streams when adding:

- large chunked capsules;
- resumable upload/download;
- progressive loading with offsets;
- live capsules;
- fork capsules;
- append findings;
- agent session event log;
- context replay.

### 20.3 Possible v1/v2 model

```text
capsule stream:
  encrypted manifest event
  encrypted compact event
  encrypted required evidence events
  encrypted optional detail chunk events
  sealed event
```

The public share URL should still go through the capsule API, not raw stream URLs. The API enforces TTL, one-time redemption, revoke, rate limits, and audit. Durable Streams remain an internal storage primitive.

### 20.4 Migration strategy

Keep the public API stable:

```text
POST /v1/capsules
PUT  /v1/capsules/:id/parts/:part_id
POST /v1/capsules/:id/finalize
POST /v1/capsules/:id/redeem
GET  /v1/capsules/:id/parts/:part_id
```

Under the hood:

```text
v0: filesystem/S3 parts
v1: Durable Streams parts/chunks
v2: live/fork session streams
```

---

## 21. ElectricSQL future path

ElectricSQL/Postgres Sync is not necessary for v0.

It may become useful if the product adds:

- web dashboard;
- “my capsules” list;
- team audit log;
- shared-with-me view;
- capsule metadata sync;
- admin policies;
- revoke UI;
- live UI around sessions/capsules.

Then the split could be:

```text
Postgres:
  users, teams, grants, capsule metadata, audit rows

Electric Shapes:
  sync metadata subsets to UI

Durable Streams:
  encrypted capsule/session event streams

Object storage:
  large encrypted artifacts
```

Not needed for v0 command-line-only handoff.

---

## 22. MVP implementation plan

### Phase 0: repo skeleton

```text
capsule-skill/
├── capsule/
│   ├── SKILL.md
│   ├── scripts/
│   │   ├── capsule.sh
│   │   └── capsule.ps1
│   ├── references/
│   │   ├── format.md
│   │   ├── security.md
│   │   └── backend.md
│   └── assets/
│       └── capsule.schema.json
└── backend/
    └── reference-server/
```

### Phase 1: local scripts only

Implement:

```text
/capsule --doctor
/capsule --send --dry-run
```

No backend yet. Verify:

- age installed;
- gzip/curl available where expected;
- can generate age keypair;
- can encrypt/decrypt a test file;
- can scan for obvious secrets;
- can create preview.

### Phase 2: simple backend

Implement:

```text
POST /v1/capsules
PUT /v1/capsules/:id/parts/:part_id
POST /v1/capsules/:id/finalize
POST /v1/capsules/:id/redeem
GET /v1/capsules/:id/parts/:part_id
```

Use SQLite + filesystem first.

### Phase 3: end-to-end send/load

Implement:

```text
/capsule --send
/capsule --load <url>
/capsule --load-detail <url> <part-id>
```

### Phase 4: polish and hardening

- size caps;
- preview confirmation;
- better secret scan;
- cleanup job;
- rate limits;
- error JSON;
- Windows path/quoting tests;
- docs;
- examples.

### Phase 5: hosted/self-host variants

- local/self-host docs;
- Dockerfile for backend;
- S3/R2 backend option;
- reverse proxy docs;
- metrics without plaintext;
- retention policy.

---

## 23. Acceptance criteria for v0

### Functional

- User can run `/capsule --send` from Claude Code-style skill UX.
- User can run equivalent flow in OpenCode with a small command shim if needed.
- Sender receives a URL with a decryption key in the fragment.
- Receiver can run `/capsule --load <url>` and import compact context.
- Optional details are not loaded by default.
- Backend never receives plaintext.
- Backend never receives URL fragment key from the scripts.

### Security

- age is required and checked.
- Secret scanner blocks high-confidence secrets by default.
- Temporary plaintext files are deleted.
- No bundled binary.
- No auto-install of dependencies.
- No dynamic remote code loading.
- One-time redemption works.
- Expired capsules cannot be loaded.

### UX

- `/capsule --doctor` explains setup problems clearly.
- `/capsule --send` shows preview before upload.
- `/capsule --load` summarizes imported context.
- Large details are listed, not injected automatically.
- Errors are actionable.

### Portability

- macOS/Linux via `capsule.sh`.
- Windows via `capsule.ps1`.
- Works with `age` installed.
- Does not require Python/Node.

---

## 24. Open questions

### Product

- Should v0 require confirmation every time, or allow `--yes`?
- Should the default TTL be 24h or shorter?
- Should one-time be default for all capsules?
- Should browser fallback show ciphertext-only metadata or allow download?
- Should the product expose human-readable HTML after local decryption later?

### Security

- Is link-key mode acceptable for the target team?
- How should the key fragment be encoded to avoid shell quoting problems?
- Should secret scan be warn-only or block-by-default?
- Should private key in URL be named `agekey`, `identity`, or wrapped as `k=`?
- Should the backend keep metadata after capsule expiry?

### Backend

- SQLite/filesystem or Postgres/S3 for first implementation?
- Practical one-time grant duration: 5, 10, or 15 minutes?
- Maximum part size for v0?
- Should upload be multipart or sequential part uploads?
- How to handle interrupted downloads after redeem?

### Agent UX

- Should `/capsule --send` automatically inspect `git diff`?
- Should it include command history only if present in context?
- Should it ask before including full diff?
- How to adapt exact slash command UX for OpenCode/Codex/Gemini?

---

## 25. Recommended defaults

```text
TTL:
  24h

One-time:
  true

Redeem grant:
  10 minutes

Compact cap:
  50 KB plaintext hard cap

Required evidence cap:
  800 KB plaintext hard cap

Total encrypted capsule cap:
  25 MB hard cap

Secret scan:
  block high-confidence secrets

Raw transcript:
  false

Full diff:
  optional detail, not default inline

Logs:
  excerpts inline, full logs optional detail

Backend:
  ciphertext only

Storage:
  SQLite + filesystem for internal alpha
  Postgres + object storage for hosted beta

Durable Streams:
  not v0 dependency
  design storage abstraction for future replacement
```

---

## 26. Example end-to-end transcript

Sender:

```text
User: /capsule --send --ttl 24h

Agent:
I will create a compact encrypted context capsule. I will include the task goal,
current state, relevant files, decisions, open questions, and selected evidence.
I will not include secrets, env vars, raw transcript, or full logs by default.
```

Preview:

```text
Capsule preview

Title: Staging auth failure handoff
Compact context: 38 KB
Required evidence: 64 KB
Optional details:
- full diff: 1.9 MB
- test output: 320 KB
Skipped:
- pnpm-lock.yaml
- coverage/
- .env

Remote receives ciphertext only.
Proceed? y/N
```

After confirmation:

```text
Capsule created:
https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...

Expires: 2026-06-12 12:00 UTC
One-time: yes
```

Receiver:

```text
User: /capsule --load https://ctx.example.com/c/cap_01J...#agekey=AGE-SECRET-KEY-...

Agent:
Loaded capsule: Staging auth failure handoff

Current task:
Fix intermittent staging auth failures caused by suspected stale JWT cache.

Relevant files:
- src/auth/session.ts
- src/middleware/jwt.ts

Open questions:
- why refresh token passes locally but fails in staging
- whether cache invalidation is skipped on deploy

Available details:
- detail.full-diff
- detail.test-output

I will not modify files until you confirm the next action.
```

---

## 27. Final recommendation

Build v0 as:

```text
one portable skill
readable SKILL.md
readable sh/ps1 scripts
age as explicit crypto dependency
ciphertext-only backend
multipart capsule format
compact-first loading
optional lazy details
no MCP requirement
no Python/Node requirement
no bundled binary
no Durable Streams requirement yet
```

Use Durable Streams later when the product grows from sealed one-time context handoff into live/forkable agent session sharing.

This keeps the first version small, auditable, privacy-preserving, and useful immediately for a team using different coding agents.

---

## 28. Source links used for this design context

- Agent Skills overview: https://agentskills.io/home
- Agent Skills specification: https://agentskills.io/specification
- Claude Code skills documentation: https://code.claude.com/docs/en/skills
- OpenCode skills documentation: https://opencode.ai/docs/skills/
- OpenCode commands documentation: https://opencode.ai/docs/commands/
- age repository: https://github.com/FiloSottile/age
- age format specification: https://age-encryption.org/v1
- Durable Streams protocol overview: https://durable-streams-durable-streams.mintlify.app/concepts/protocol-overview
- Durable Streams protocol draft: https://github.com/durable-streams/durable-streams/blob/main/PROTOCOL.md
