# Capsule v0 — Backend Reflection & Architecture

Дата: 2026-06-11  
Статус: рабочий архитектурный документ  
Контекст: v0 продукта `Capsule` как **privacy-first transfer**, не collaboration

---

## 0. Короткая формулировка

`Capsule v0` — это не платформа коллаборации, не memory system и не MCP server.

Это **sealed transfer primitive**:

```text
AI agent A
  → собирает рабочий контекст
  → локально упаковывает
  → локально шифрует через age
  → backend хранит только ciphertext
  → AI agent B скачивает ciphertext
  → локально расшифровывает
  → импортирует compact context
```

Главный принцип backend:

```text
backend never sees plaintext
backend never sees age private key / decryption key
backend is only a ciphertext rendezvous point
```

---

## 1. Решение, зафиксированное для v0

### Client / Skill layer

```text
one skill
SKILL.md
scripts/capsule.sh
scripts/capsule.ps1
dependency: age
no bundled binary
no Python/Node requirement
```

### Backend layer

```text
simple ciphertext backend
multipart encrypted capsule parts
TTL
one-time / practical one-time redemption
rate limits
garbage collection
no plaintext processing
```

### Out of scope for v0

```text
Durable Streams
Electric Postgres Sync
MCP server
browser extension
team workspace
web collaboration
shared memory
semantic search
live editing
server-side AI agents
```

---

## 2. Product boundary: transfer vs collaboration

### v0: transfer

v0 is:

```text
send → load
```

Meaning:

```text
one sender
one or more explicit recipients
bounded context
explicit action
encrypted handoff artifact
expiration
optional one-time access
```

The user intent:

```text
"Я работал с AI agent, хочу передать текущий рабочий контекст другому человеку/агенту."
```

### Not v0: collaboration

Collaboration is:

```text
join → catch up → co-edit → append → sync → export/import
```

It needs:

```text
presence
multi-user permissions
live stream
conflict model
forks
workspace state
web UI
possibly Yjs / Durable Streams / Electric
```

This should be treated as a separate product layer.

---

## 3. Backend role

The backend is deliberately dumb.

It should do:

```text
accept encrypted bytes
store encrypted bytes
return encrypted bytes
enforce TTL
enforce redemption rules
delete expired capsules
rate limit
measure size
audit operational metadata
```

It should not do:

```text
summarization
redaction
secret scanning of plaintext
semantic search
indexing context
AI processing
payload inspection
decrypt
compress
merge
modify
```

Reason:

```text
If backend is zero-knowledge / ciphertext-only, it cannot safely process plaintext semantics.
All semantic work must happen before encryption on the sender side or after decryption on the recipient side.
```

---

## 4. Threat model

### Protect against

```text
backend operator reading capsule content
database leak exposing plaintext
object storage leak exposing plaintext
accidental server logs containing context
simple replay of expired/consumed capsules
casual unauthorized access by guessing IDs
large accidental context upload without preview
```

### Does not protect against

```text
sending AI agent seeing plaintext
receiving AI agent seeing plaintext
LLM provider processing plaintext during agent session
recipient copying decrypted content
recipient forwarding full URL
malware on local machine
compromised age binary
malicious shell/PowerShell environment
browser/terminal history storing full URL
```

### Precise privacy statement

Recommended wording:

```text
Capsule protects your context from the Capsule backend.
Encryption and decryption happen locally.
The backend stores only ciphertext and operational metadata.
The AI environments you intentionally use to create/load capsules still see plaintext.
```

---

## 5. Basic data flow

### `/capsule --send`

```text
User invokes /capsule --send
  ↓
Agent reads current conversation and workspace context
  ↓
Agent creates compact structured capsule parts
  ↓
Skill script previews size/content classes
  ↓
User confirms
  ↓
Script compresses parts
  ↓
Script encrypts parts with age
  ↓
Script uploads encrypted parts to backend
  ↓
Backend returns capsule id / public base URL
  ↓
Script appends local decryption material in URL fragment
  ↓
User receives share URL
```

### `/capsule --load <url>`

```text
User invokes /capsule --load <url>
  ↓
Script parses URL locally
  ↓
Script extracts decryption material from fragment
  ↓
Script requests manifest/compact parts from backend
  ↓
Backend returns ciphertext
  ↓
Script decrypts locally with age
  ↓
Script prints compact context
  ↓
Agent imports context into current session
  ↓
Optional detail parts can be lazy-loaded
```

---

## 6. URL design

### Link-key / age identity mode

Example:

```text
https://ctx.example.com/c/cap_01JABC...#agekey=AGE-SECRET-KEY-...
```

Backend sees only:

```text
GET /c/cap_01JABC...
```

Backend must never receive:

```text
#agekey=...
```

Important:

```text
URL fragment is not sent by browsers in normal HTTP requests,
but it can be leaked if the agent/tool passes the full URL to a remote API.
Therefore load must be local script logic, not remote MCP load.
```

### Alternative fragment format

```text
https://ctx.example.com/c/cap_01JABC...#k=<base64url-key>
```

But with `age`, using an ephemeral age identity can be cleaner:

```text
#agekey=AGE-SECRET-KEY-...
```

### Security implications

Anyone with the full URL including fragment can decrypt.

This is acceptable for v0 if documented:

```text
Treat the full capsule URL like a secret.
```

Future stronger mode:

```text
recipient public key mode
/capsule --send --to bob
```

Then the URL does not contain a decryption key; only Bob’s private key can decrypt.

---

## 7. Why multipart matters

A simple one-blob design is easier:

```text
capsule.json.gz.age
```

But it fails as soon as context grows.

Problems with a single blob:

```text
load blocks until entire blob is downloaded
recipient agent may ingest too much plaintext
optional logs/diffs are not lazy-loadable
one-time semantics become hostile if network breaks
large capsules are all-or-nothing
```

v0 should already use **multipart encrypted parts**.

---

## 8. Capsule part model

A capsule consists of parts:

```text
manifest
compact
evidence
details/*
```

Each part is compressed and encrypted independently:

```text
part plaintext
  → gzip
  → age encrypt
  → upload encrypted part
```

This allows:

```text
load manifest first
load compact first
lazy-load details
avoid feeding huge logs/diffs into agent context
preserve E2EE
```

### Required parts

```text
manifest
compact
```

### Optional parts

```text
evidence.errors
evidence.decisions
evidence.file-excerpts
detail.full-diff
detail.test-output
detail.logs
detail.artifact-index
```

---

## 9. Manifest design

The manifest should be encrypted too if it contains sensitive details.

But there are two possible approaches:

### Option A: encrypted manifest only

Backend cannot know part names/kinds.

Pros:

```text
stronger privacy
backend cannot infer context shape
```

Cons:

```text
backend cannot serve part-by-kind unless client knows part storage keys
load requires downloading encrypted index first
```

### Option B: minimal public manifest + encrypted private manifest

Public metadata:

```json
{
  "capsule_version": "v0",
  "part_count": 4,
  "total_encrypted_size": 381923,
  "expires_at": "2026-06-12T12:00:00Z",
  "one_time": true
}
```

Encrypted private manifest contains:

```json
{
  "title": "Auth debugging handoff",
  "parts": [
    {
      "id": "compact",
      "kind": "markdown",
      "required": true,
      "encrypted_ref": "part_01",
      "plaintext_size_estimate": 43000,
      "encrypted_size": 17120
    },
    {
      "id": "detail.full-diff",
      "kind": "patch",
      "required": false,
      "encrypted_ref": "part_02",
      "encrypted_size": 2104931
    }
  ]
}
```

Recommendation for v0:

```text
Use minimal public metadata + encrypted private manifest.
```

This keeps backend operationally useful without seeing sensitive context.

---

## 10. Recommended part structure

### `compact.md`

Should be loaded by default.

Contents:

```markdown
# Capsule: <title>

## Goal

## Current state

## Hypothesis

## What was tried

## Decisions made

## Open questions

## Relevant files

## Next suggested actions

## Safety / redaction notes
```

Target size:

```text
30–50 KB plaintext max
roughly 2k–8k tokens
```

### `evidence.md` or structured evidence parts

Loaded by default only if small.

Contents:

```text
important errors
short stack traces
short selected diff hunks
file excerpt references
test result summary
```

Target size:

```text
300–800 KB plaintext max
```

### Optional details

Examples:

```text
full diff
long logs
full test output
large file excerpts
artifact index
```

Not loaded by default.

---

## 11. Size and performance guidance

### Rough payload classes

| Capsule class | Plaintext | Compressed | UX |
|---|---:|---:|---|
| tiny | 20–100 KB | 5–30 KB | instant |
| normal | 200 KB–1 MB | 50–300 KB | fine |
| heavy | 2–10 MB | 500 KB–3 MB | staged recommended |
| logs/diffs | 20–100 MB | 5–40 MB | optional details only |
| binaries/screenshots | 1–100+ MB | weak compression | attachments/references |

### v0 limits

Recommended defaults:

```text
compact.md:
  max 50 KB plaintext

inline evidence:
  max 800 KB plaintext

encrypted capsule total:
  soft cap 10 MB
  hard cap 25 MB

single optional detail:
  soft cap 10 MB
  hard cap 50 MB

larger:
  require explicit --include-large or reject
```

### User preview

Before upload:

```text
Capsule preview:

Compact context: 42 KB
Inline evidence: 380 KB
Optional details:
  full diff: 2.1 MB
  test output: 4.4 MB
Skipped:
  server.log: 82 MB, too large

Remote will receive: encrypted parts only
Mode: one-time, expires in 24h

Proceed? y/N
```

---

## 12. One-time semantics

There are two possible modes.

### Strict one-time

```text
first successful GET marks capsule consumed before returning bytes
```

Pros:

```text
simple
strong one-time semantics
```

Cons:

```text
if network fails mid-download, recipient loses access
bad for multipart lazy loading
```

### Practical one-time

```text
first redeem creates short-lived download session
recipient can download required parts for N minutes
after session expires, access is gone
```

Pros:

```text
better UX
works with multipart/staged loading
allows compact first + details later
```

Cons:

```text
more state
one-time means "one redemption session", not one HTTP response
```

Recommendation:

```text
Use practical one-time for multipart capsules.
Default redemption window: 10 minutes.
```

Semantics:

```text
one-time capsule:
  may be redeemed once
  redemption opens a short-lived access session
  all part downloads must happen during that session
```

---

## 13. Backend API v0

### Create capsule

```http
POST /v1/capsules
Content-Type: application/json
```

Request:

```json
{
  "one_time": true,
  "ttl_seconds": 86400,
  "parts": [
    {
      "client_part_id": "manifest",
      "encrypted_size": 1200,
      "sha256": "..."
    },
    {
      "client_part_id": "compact",
      "encrypted_size": 17120,
      "sha256": "..."
    }
  ]
}
```

Response:

```json
{
  "id": "cap_01JABC...",
  "upload_urls": {
    "manifest": "https://ctx.example.com/v1/capsules/cap_01JABC/parts/manifest",
    "compact": "https://ctx.example.com/v1/capsules/cap_01JABC/parts/compact"
  },
  "expires_at": "2026-06-12T12:00:00Z",
  "one_time": true
}
```

### Upload part

```http
PUT /v1/capsules/:id/parts/:part_id
Content-Type: application/octet-stream

<encrypted bytes>
```

Response:

```json
{
  "ok": true,
  "part_id": "compact",
  "size": 17120,
  "sha256": "..."
}
```

### Finalize capsule

```http
POST /v1/capsules/:id/finalize
```

Response:

```json
{
  "ok": true,
  "download_url": "https://ctx.example.com/c/cap_01JABC..."
}
```

Client appends key material locally:

```text
https://ctx.example.com/c/cap_01JABC...#agekey=AGE-SECRET-KEY-...
```

### Redeem capsule

```http
POST /v1/capsules/:id/redeem
```

Response:

```json
{
  "redeem_token": "red_...",
  "expires_at": "2026-06-11T12:10:00Z",
  "parts": [
    {
      "part_id": "manifest",
      "encrypted_size": 1200
    },
    {
      "part_id": "compact",
      "encrypted_size": 17120
    }
  ]
}
```

### Download part

```http
GET /v1/capsules/:id/parts/:part_id
Authorization: Bearer red_...
```

Response:

```http
200 OK
Content-Type: application/octet-stream

<encrypted bytes>
```

### Delete / revoke

```http
DELETE /v1/capsules/:id
```

For unauthenticated v0 link mode, revoke is hard unless sender receives a separate management token.

Create response can include:

```json
{
  "management_url": "https://ctx.example.com/m/cap_01JABC...#mgmt=..."
}
```

But this adds complexity. For v0, revoke can be omitted or implemented with a local management token printed once.

---

## 14. Storage options

### Option A: SQLite + filesystem

Good for:

```text
prototype
self-hosted small team
single-server deployment
```

Layout:

```text
data/
  capsules.db
  parts/
    cap_01JABC/
      manifest.age
      compact.age
      detail.full-diff.age
```

Pros:

```text
very simple
cheap
easy to debug
no object storage dependency
```

Cons:

```text
single-node
backup/HA manually managed
large files may be awkward
```

### Option B: Postgres + S3/R2/GCS

Good for:

```text
hosted service
multi-node API
scale
```

Metadata in Postgres:

```text
capsules
capsule_parts
redemptions
```

Encrypted bytes in object storage.

Pros:

```text
scales better
object lifecycle policies
durability
CDN possible
```

Cons:

```text
more infrastructure
must ensure signed URLs do not bypass redemption semantics
```

### Option C: Durable Streams internal storage

Good later for:

```text
streaming parts
progressive upload
resumability
live capsule
fork capsule
append findings
```

Not needed for v0.

---

## 15. Database schema sketch

### `capsules`

```sql
CREATE TABLE capsules (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL CHECK (status IN ('creating', 'finalized', 'expired', 'deleted')),
  one_time BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finalized_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  total_encrypted_size BIGINT NOT NULL DEFAULT 0,
  part_count INTEGER NOT NULL DEFAULT 0,
  public_metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

### `capsule_parts`

```sql
CREATE TABLE capsule_parts (
  capsule_id TEXT NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  part_id TEXT NOT NULL,
  storage_key TEXT NOT NULL,
  encrypted_size BIGINT NOT NULL,
  sha256 TEXT NOT NULL,
  uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (capsule_id, part_id)
);
```

### `redemptions`

```sql
CREATE TABLE redemptions (
  id TEXT PRIMARY KEY,
  capsule_id TEXT NOT NULL REFERENCES capsules(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  client_ip_hash TEXT,
  user_agent_hash TEXT
);
```

### Atomic redemption

```sql
UPDATE capsules
SET consumed_at = now()
WHERE id = $1
  AND one_time = true
  AND consumed_at IS NULL
  AND expires_at > now()
  AND status = 'finalized'
RETURNING id;
```

For practical one-time:

```text
after this update, create redemptions row with 10-minute expiry
```

---

## 16. Operational metadata

Backend may store:

```text
capsule id
part count
encrypted sizes
hashes of encrypted parts
created_at
expires_at
consumed_at
rate-limit metadata
IP hash
user-agent hash
```

Backend should avoid storing:

```text
full source IP forever
full user agent forever
full referrer
full URL
URL fragment
raw request body logs
plaintext title
plaintext file names
plaintext part semantic names if sensitive
```

If public part names are used, keep them generic:

```text
part_0001
part_0002
```

Encrypted manifest maps them to semantic names.

---

## 17. Logging policy

Never log:

```text
request body
full URL including fragment
Authorization bearer tokens
redeem token
management token
```

Log only:

```text
request id
capsule id
status code
size
duration
coarse error code
```

Potential log line:

```json
{
  "request_id": "req_...",
  "route": "GET /v1/capsules/:id/parts/:part_id",
  "capsule_id": "cap_...",
  "part_id": "part_0002",
  "status": 200,
  "encrypted_size": 17120,
  "duration_ms": 42
}
```

---

## 18. Error model

### Upload errors

```text
413 payload too large
400 invalid capsule metadata
409 capsule already finalized
422 part hash mismatch
429 rate limited
500 storage failure
```

### Load errors

```text
404 not found
410 expired
410 already consumed
403 invalid/expired redemption token
422 corrupted ciphertext or hash mismatch
```

### Local decrypt errors

These happen client-side:

```text
missing age
invalid age key
wrong URL fragment
ciphertext corrupted
decompression failed
unsupported capsule version
```

The script should print actionable errors:

```text
Capsule exists but decryption failed.
Possible causes:
- wrong or truncated URL fragment
- copied URL without #agekey=...
- corrupted download
- incompatible capsule version
```

---

## 19. Abuse controls

Even with ciphertext-only backend, it can be abused as file hosting.

v0 should include:

```text
max capsule size
max part size
TTL cap
rate limiting by IP
upload count limits
content-type enforcement
no public directory listing
no search
no permanent hosting mode
background deletion
optional CAPTCHA only for public unauthenticated web flows
```

Recommended defaults:

```text
max ttl: 7 days
default ttl: 24 hours
max total encrypted size: 25 MB
max part encrypted size: 50 MB only with explicit allow
anonymous upload rate: strict
team/self-host mode: configurable
```

---

## 20. Multipart + one-time subtlety

If one-time redemption is too strict, lazy loading breaks.

Example:

```text
/capsule --load <url>
downloads compact only
later user wants detail.full-diff
but capsule already consumed
```

Therefore v0 should define:

```text
one-time = one redemption session
not one HTTP part download
```

Within redemption session:

```text
manifest + compact + selected details can be downloaded
window expires after 10 minutes
```

This is user-comprehensible:

```text
The link can be opened once. After opening, details remain available for 10 minutes.
```

---

## 21. Client/backend contract

### Backend does not enforce semantics of parts

It does not know:

```text
which part is compact
which part is full diff
which part is logs
```

The encrypted manifest tells the client.

### Backend enforces bytes and lifecycle

```text
part exists
size is within limits
hash matches
capsule is finalized
capsule is not expired
redemption is valid
```

This keeps backend simple and private.

---

## 22. Why not remote MCP backend in v0

Remote MCP would create a privacy trap:

```text
/capsule --load https://ctx.example.com/c/cap#agekey=...
```

If the agent sends that full URL to a remote MCP server:

```text
remote MCP receives age key
zero-knowledge broken
```

Therefore:

```text
load/decrypt must happen locally
```

MCP can be added later only as:

```text
local MCP wrapper
```

or remote MCP that never receives fragment/key and only talks in ciphertext metadata.

For v0, keep it simpler:

```text
skill script calls backend directly
```

---

## 23. Why not Durable Streams in v0

Durable Streams solve:

```text
append-only stream
offset reads
catch-up
live tail
fork
multi-reader stream
resumable sessions
```

v0 needs:

```text
upload encrypted parts
download encrypted parts
expire
redeem once
```

Durable Streams would add complexity before its strengths are needed.

Recommendation:

```text
do not use Durable Streams in v0 backend
```

But design storage interface so migration is possible.

---

## 24. Storage interface abstraction

Internal backend interface:

```ts
interface CapsulePartStore {
  putPart(capsuleId: string, partId: string, bytes: ReadableStream): Promise<StoredPart>
  getPart(capsuleId: string, partId: string): Promise<ReadableStream>
  deleteCapsule(capsuleId: string): Promise<void>
}
```

Implementations:

```text
FilesystemPartStore
S3PartStore
DurableStreamsPartStore
```

Public API remains stable while storage changes.

---

## 25. Future Durable Streams mapping

When collaboration or progressive streaming becomes important:

```text
capsule = Durable Stream
part = encrypted stream event
large detail = multiple encrypted chunk events
sealed = final event
```

Example stream events:

```json
{ "type": "part", "part_ref": "part_0001", "bytes": "<encrypted>" }
{ "type": "part", "part_ref": "part_0002", "bytes": "<encrypted>" }
{ "type": "sealed" }
```

Or for chunked large parts:

```json
{ "type": "part.chunk", "part_ref": "detail.full-diff", "index": 0, "bytes": "..." }
{ "type": "part.chunk", "part_ref": "detail.full-diff", "index": 1, "bytes": "..." }
{ "type": "part.complete", "part_ref": "detail.full-diff" }
```

E2EE remains:

```text
Durable Streams stores encrypted events only
clients decrypt locally
```

Backend still should not see plaintext.

---

## 26. Future Electric ecosystem boundary

Electric Postgres Sync / Shapes becomes useful only when there is a UI:

```text
my capsules
shared with me
team workspaces
audit log
permissions
workspace list
live dashboard
```

For v0 transfer:

```text
Electric Sync is unnecessary
```

For future live context workspace:

```text
Postgres:
  users, teams, ACL, workspace metadata

Electric Shapes:
  sync workspace lists and metadata to web app

Durable Streams:
  live workspace event log

Yjs:
  collaborative rich text / CRDT editing if needed
```

---

## 27. Backend deployment options

### Local/self-host dev

```text
single binary or simple web service
SQLite
filesystem storage
reverse proxy with HTTPS
```

### Team internal

```text
containerized API
Postgres
filesystem or S3-compatible storage
OIDC optional
internal network
```

### Public hosted

```text
API service
Postgres
S3/R2/GCS
global CDN only for encrypted objects
rate limits
abuse protection
observability
deletion worker
signed release / security docs
```

---

## 28. Auth model v0

### Anonymous link mode

Simplest:

```text
no accounts
unguessable capsule ids
one-time/TTL
fragment key controls decryption
```

Pros:

```text
low friction
best for MVP
```

Cons:

```text
hard revoke
hard ownership
abuse risk
no dashboard
```

### Sender auth mode

Sender logs in or uses API token.

Pros:

```text
revoke
list sent capsules
quota
better abuse handling
```

Cons:

```text
more product surface
privacy questions
accounts before value
```

Recommendation:

```text
Start with anonymous/self-host mode or simple team token.
Avoid full account system in v0 unless necessary.
```

---

## 29. Backend does not need to know recipients

In link-key v0:

```text
recipient is anyone with full URL
```

This is simpler and avoids identity.

Future recipient-key mode:

```text
recipient public key
team public keys
key directory
```

This belongs in v1/v2.

---

## 30. Management/revocation token

Optional v0 feature:

On create, return:

```text
share_url:
  https://ctx.example.com/c/cap_123#agekey=...

management_url:
  https://ctx.example.com/m/cap_123#mgmt=...
```

Management token allows:

```text
check status
delete before redemption
```

But it must be stored locally or printed once.

Risk:

```text
another secret URL
more UX complexity
```

Recommendation:

```text
omit from earliest v0 unless revoke is required
```

---

## 31. Deletion and retention

Default policy:

```text
delete expired capsules quickly
delete consumed one-time capsules after redemption window
delete unfinished uploads after 1 hour
```

Background jobs:

```text
expire capsules
delete orphan parts
delete creating capsules not finalized
compact metadata
```

Retention statements should be explicit:

```text
Encrypted capsule parts are deleted after expiry or successful one-time redemption window.
Operational logs may remain for N days and do not contain plaintext or URL fragments.
```

---

## 32. Observability

Metrics:

```text
capsules_created
capsules_finalized
capsules_redeemed
capsules_expired
parts_uploaded
part_downloads
upload_bytes
download_bytes
decrypt_failures? 
```

Note:

```text
decrypt failures happen client-side and should not be reported automatically unless user opts in.
```

Logs should not include sensitive data.

---

## 33. Client-side preview and guardrails

Backend cannot inspect plaintext, so client must protect users before upload.

Required local checks:

```text
show included sections
show size
warn on large parts
detect obvious secrets by regex
skip lock files and generated outputs by default
confirm before upload
```

Secret patterns:

```text
AWS keys
GitHub tokens
JWT-looking strings
private key blocks
.env-like lines
Slack tokens
OpenAI/Anthropic API key patterns
```

This is not perfect, but better than silent upload.

---

## 34. File/diff inclusion policy

Default excludes:

```text
.env
*.pem
*.key
id_rsa
node_modules/
dist/
build/
coverage/
.git/
*.png
*.jpg
*.zip
*.tar
*.pdf
package-lock.json
pnpm-lock.yaml
yarn.lock
*.min.js
*.map
```

Default includes:

```text
summary of git status
summary of diff
selected important hunks
file paths
short excerpts only
test/error summaries
```

Full diff/logs only as optional details.

---

## 35. Suggested backend implementation for first prototype

### Minimal

```text
language: any
storage: SQLite + filesystem
deployment: one container / one VPS / internal machine
HTTPS: reverse proxy
```

API:

```text
POST /v1/capsules
PUT  /v1/capsules/:id/parts/:part_id
POST /v1/capsules/:id/finalize
POST /v1/capsules/:id/redeem
GET  /v1/capsules/:id/parts/:part_id
```

No auth at first, but with:

```text
unguessable IDs
TTL
size caps
rate limits
no directory listing
```

---

## 36. Suggested backend implementation for hosted MVP

```text
API service
Postgres
S3/R2
background worker
rate limiter
structured logs
```

Important:

```text
avoid object-storage public URLs unless they preserve redemption semantics
```

If using pre-signed URLs for download, generate them only after redeem and with very short TTL.

---

## 37. Public API philosophy

Keep public API storage-agnostic:

```text
/v1/capsules
/v1/capsules/:id/parts/:part_id
```

Do not expose:

```text
S3 keys
Durable Stream internals
database IDs
storage implementation
```

This allows migration to Durable Streams later.

---

## 38. Future roadmap from backend perspective

### v0

```text
sealed encrypted transfer
multipart parts
simple backend
TTL / one-time redemption
```

### v0.1

```text
load-detail
management/revoke token
better preview
secret scanner improvements
larger part support
```

### v1

```text
recipient public key mode
team key directory
sender auth optional
audit dashboard optional
```

### v2

```text
Durable Streams storage
progressive streaming
append findings
capsule forks
```

### v3

```text
web context workspace
multi-user editing
agents as participants
Electric Shapes for metadata
Yjs or StreamDB for live structured state
```

---

## 39. Decision summary

### Use for v0

```text
simple ciphertext backend
multipart encrypted parts
practical one-time redemption
age-based local E2EE
compact-first loading
storage abstraction
```

### Do not use for v0

```text
Durable Streams
Electric Sync
remote MCP
plaintext backend processing
browser extension
accounts unless necessary
full collaboration layer
```

### Keep open for future

```text
Durable Streams as encrypted event/part log
Electric Shapes for workspace metadata
Yjs for collaborative editing
recipient public keys
workspace import/export
```

---

## 40. Final architectural statement

Capsule v0 backend is a **ciphertext rendezvous service**.

It should be boring, minimal, and privacy-preserving:

```text
store encrypted parts
enforce lifecycle
serve encrypted parts once or within a short redemption window
delete aggressively
never process plaintext
```

Do not make the backend intelligent in v0.

The intelligence belongs to:

```text
the sending agent
the local preview/packaging script
the receiving agent
```

The backend belongs to:

```text
transport
storage
expiry
redemption
abuse control
```

This is the right architecture for a privacy-first transfer primitive.

