# Capsule v0 Hosting Research

**Date:** 2026-06-11  
**Scope:** hosting/backend/storage options for Capsule v0  
**Primary constraint:** keep the v0 security model: local encryption/decryption, backend stores ciphertext only, backend never receives plaintext or the `age` decryption key.  
**Important availability constraint:** service should be usable both globally and from Russia where possible.

---

## 1. Capsule v0 backend requirements

Capsule v0 does not need a “smart” backend. It needs a small ciphertext rendezvous service.

Required backend responsibilities:

```text
POST encrypted capsule parts
GET encrypted capsule parts
TTL / expiry
one-time redeem
short-lived download grants
cleanup
rate limiting / abuse controls
```

Backend must not do:

```text
plaintext summarization
redaction
semantic indexing
diff/log parsing
AI processing
secret scanning after upload
decryption
key storage
```

Security boundary:

```text
sender script:
  build plaintext capsule locally
  compress locally
  encrypt locally with age
  upload ciphertext only

backend:
  store encrypted .age parts
  store operational metadata only
  enforce lifecycle / one-time / TTL

receiver script:
  parse #agekey locally
  download ciphertext
  decrypt locally
```

The key property: **backend compromise should expose encrypted blobs and metadata, not plaintext context**.

---

## 2. State vs object storage

Object storage alone is enough only for encrypted bytes:

```text
capsules/cap_.../manifest.age
capsules/cap_.../compact.age
capsules/cap_.../evidence.errors.age
capsules/cap_.../detail.full-diff.age
```

A state primitive is needed for:

```text
capsule created / finalized / expired / consumed
atomic one-time redeem
short-lived redeem token
part registry
cleanup
rate limits
abuse controls
```

The state primitive can be:

```text
SQLite
Postgres
DynamoDB
Cloudflare Durable Object
Redis / Upstash
provider DB
metadata JSON in object storage, if one-time is disabled or best-effort
```

Recommended mental model:

```text
Object storage = encrypted bytes
State store     = lifecycle / access coordination
```

---

## 3. Evaluation criteria

The relevant criteria for Capsule v0:

| Criterion | Meaning |
|---|---|
| Simplicity | Few moving parts, easy deploy, easy debug |
| Transparency | Can inspect state, logs, storage objects, and lifecycle |
| Security | Private storage, no plaintext, no secrets in local scripts, easy key handling |
| One-time correctness | Can atomically redeem a link once |
| Cost predictability | Low base cost and controlled egress/operation cost |
| RF availability | Likelihood that links work from Russia |
| Global availability | Good enough latency/reliability outside Russia |
| Migration path | Easy to move to another provider later |

---

## 4. Short recommendation

### Best default for simple v0

```text
Hetzner VPS + SQLite + Hetzner Object Storage
```

Why:

```text
+ very simple
+ cheap
+ transparent
+ no Cloudflare dependency
+ EU location is reasonable for Russia + Europe + global users
+ SQLite is enough for v0 one-time redeem
+ object storage keeps encrypted .age parts outside the VPS disk
```

Approximate monthly cost:

```text
~€9–12/month before taxes/overages
```

### Best developer-friendly alternative

```text
DigitalOcean Droplet + SQLite + Spaces
```

Why:

```text
+ very easy dashboard
+ simple S3-compatible storage
+ predictable low starting cost
```

Approximate monthly cost:

```text
~$9–15/month
```

### Best global deploy DX

```text
Fly.io app + Tigris object storage
```

Why:

```text
+ good deploy workflow
+ global app deployment model
+ Tigris is S3-compatible and has no egress fees
```

Approximate monthly cost:

```text
~$5–20/month for early v0
```

### Best “RF + world” serious candidate

```text
Gcore Cloud/Object Storage
```

Why:

```text
+ stronger network story for Russia/CIS + global reach
+ one vendor can provide cloud, object storage, and CDN
```

Downside:

```text
- more expensive
- more platform-specific
```

### Best managed SQL/BaaS candidate

```text
Supabase Storage + Postgres + Edge Functions
```

Why:

```text
+ SQL transparency
+ private buckets / signed URLs
+ dashboard
+ Auth/team layer can be added later
```

Downside:

```text
- egress can dominate cost
- more expensive baseline than VPS/object storage
```

---

## 5. Key RF availability note

Cloudflare was previously attractive for Capsule v0 because Workers + R2 + Durable Objects fit the architecture very well. However, for a product that must be reachable from Russia, Cloudflare should not be the only endpoint.

Cloudflare officially reports systematic throttling by Russian ISPs against websites and services protected by Cloudflare. Their support page says the restriction can reduce transfer to approximately 16 KB per connection, making many sites inaccessible or unusable for Russian visitors.

Implication:

```text
Cloudflare Workers + R2 + Durable Objects:
  technically excellent for v0
  cost excellent
  RF availability risk too high as sole backend
```

Use Cloudflare only if:

```text
- Russian availability is not critical, or
- you provide a non-Cloudflare fallback endpoint, or
- real tests show acceptable access for your users
```

---

## 6. Recommended v0 architecture patterns

### Pattern A: VPS + SQLite + object storage

```text
User scripts
  ↓ HTTPS
Small API service on VPS
  ↓
SQLite state DB
  ↓
S3-compatible object storage
```

Good for:

```text
maximum simplicity
full transparency
low price
easy migration
```

Backend responsibilities:

```text
POST /v1/capsules
PUT  /v1/capsules/:id/parts/:part_id
POST /v1/capsules/:id/finalize
POST /v1/capsules/:id/redeem
GET  /v1/capsules/:id/parts/:part_id?redeem_token=...
```

State tables:

```sql
capsules(id, status, expires_at, finalized_at, consumed_at, one_time, ...)
capsule_parts(capsule_id, part_id, kind, storage_key, encrypted_size, sha256, ...)
redeem_grants(token_hash, capsule_id, expires_at, ...)
```

Best provider fit:

```text
Hetzner
DigitalOcean
OVHcloud
Akamai/Linode
Scaleway
```

### Pattern B: Managed BaaS

```text
User scripts
  ↓
Edge/API functions
  ↓
Managed database
  ↓
Managed object storage
```

Good for:

```text
quick dashboard
future auth/team metadata
SQL visibility
less server maintenance
```

Provider fit:

```text
Supabase
Firebase/GCP
Convex
InstantDB
```

Main caution:

```text
Do not proxy large encrypted blobs through function runtimes unless limits are verified.
Prefer signed upload/download URLs.
```

### Pattern C: Serverless state + object storage

```text
Worker/function
  ↓
atomic state primitive
  ↓
object storage
```

Good for:

```text
no servers
low operational maintenance
atomic one-time redeem
```

Provider fit:

```text
Cloudflare Workers + R2 + Durable Objects
AWS Lambda + S3 + DynamoDB
Tigris/R2 + Upstash + Worker
```

Main caution:

```text
RF accessibility and egress pricing vary heavily.
```

---

## 7. Provider notes

## 7.1 Hetzner

Recommended architecture:

```text
Hetzner Cloud VPS
  Go/Fastify API
  SQLite
  cron cleanup

Hetzner Object Storage
  encrypted .age parts
```

Pros:

```text
+ very simple
+ cheap
+ transparent
+ EU locations near Russia
+ object storage includes meaningful storage/traffic allowance
+ no complex platform-specific runtime
```

Cons:

```text
- you maintain a VPS
- RF access must be tested
- account/payment/compliance must be checked
```

Pricing snapshot:

```text
Cloud VPS: starts around a few EUR/month
Object Storage: base price includes 1 TB storage and 1 TB egress
```

Research source:

```text
https://www.hetzner.com/cloud
https://www.hetzner.com/storage/object-storage/
```

Recommendation:

```text
Best default v0 choice if you want transparent and low-maintenance enough.
```

---

## 7.2 DigitalOcean

Recommended architecture:

```text
DigitalOcean Droplet
  API + SQLite

DigitalOcean Spaces
  encrypted .age parts
```

Pros:

```text
+ very good developer UX
+ simple dashboard
+ Spaces is S3-compatible
+ predictable entry pricing
+ Amsterdam / Frankfurt regions are useful for Europe/Russia testing
```

Cons:

```text
- US provider / sanctions/export compliance risk
- RF network path must be tested
- CDN path may be different from raw Spaces path
```

Pricing snapshot:

```text
Droplets: entry tier from low single-digit USD/month
Spaces: $5/month base, includes storage and transfer allowance
```

Research source:

```text
https://www.digitalocean.com/pricing/droplets
https://www.digitalocean.com/pricing/spaces-object-storage
https://docs.digitalocean.com/platform/regional-availability/
```

Recommendation:

```text
Best developer-friendly VPS + object storage option.
```

---

## 7.3 Fly.io + Tigris

Recommended architecture:

```text
Fly.io app
  API
  SQLite volume or small external state

Tigris
  encrypted .age parts
```

Pros:

```text
+ excellent deploy workflow
+ global app deployment model
+ Tigris is S3-compatible
+ Tigris has no egress fees
+ Tigris usage can appear on Fly bill when provisioned through Fly
```

Cons:

```text
- less transparent than plain VPS
- no Russia region
- RF path must be tested
- US company / export compliance considerations
```

Pricing snapshot:

```text
Fly: usage-based app pricing
Tigris: usage-based object storage, no egress fees
```

Research source:

```text
https://fly.io/docs/about/pricing/
https://fly.io/docs/tigris/
https://www.tigrisdata.com/pricing/
```

Recommendation:

```text
Good if deployment DX matters more than plain VPS transparency.
```

---

## 7.4 OVHcloud

Recommended architecture:

```text
OVHcloud VPS/Public Cloud
  API + SQLite/Postgres

OVHcloud Object Storage
  encrypted .age parts
```

Pros:

```text
+ European provider
+ S3-compatible object storage
+ no hidden ingress/egress/API fees on object storage according to OVH materials
+ good candidate for bandwidth-heavy encrypted downloads
```

Cons:

```text
- UX can be less polished than DO/Fly
- RF access must be tested
```

Pricing snapshot:

```text
Object storage pricing is capacity-based; OVH states no hidden ingress/egress/API fees.
```

Research source:

```text
https://www.ovhcloud.com/en/public-cloud/object-storage/
https://www.ovhcloud.com/en/public-cloud/prices/
```

Recommendation:

```text
Good EU production-ish option if you expect download volume.
```

---

## 7.5 Scaleway

Recommended architecture:

```text
Scaleway instance/container
  API + SQLite/Postgres

Scaleway Object Storage
  encrypted .age parts
```

Pros:

```text
+ European provider
+ S3-compatible object storage
+ requests included
+ transparent storage pricing
```

Cons:

```text
- egress free allowance is limited
- not as compelling as Hetzner/OVH for this v0
- RF access must be tested
```

Pricing snapshot:

```text
Standard Multi-AZ around €0.0146/GB/month
Standard One Zone around €0.00752/GB/month
75 GB egress free/month, then around €0.01/GB
requests included
```

Research source:

```text
https://www.scaleway.com/en/pricing/storage/
https://www.scaleway.com/en/docs/object-storage/faq/
```

Recommendation:

```text
Viable, but not first choice.
```

---

## 7.6 Gcore

Recommended architecture:

```text
Gcore Cloud/API
Gcore Object Storage
Optional Gcore CDN
```

Pros:

```text
+ strong global network story
+ potentially better Russia/CIS reach than many US/EU-only providers
+ S3-compatible object storage
+ single vendor can provide cloud/storage/CDN
```

Cons:

```text
- more expensive than Hetzner/DO/Fly/Tigris
- more platform-specific
- current Russia reach must be verified by testing
```

Pricing snapshot:

```text
S3 Standard:
  storage about $0.044/GB/month
  egress about $0.022/GB
  requests about $0.033/10K
```

Research source:

```text
https://gcore.com/storage
https://gcore.com/docs/storage
https://gcore.com/docs/storage/how-storage-is-billed
```

Recommendation:

```text
Best candidate if Russia + global reach becomes the main blocker.
```

---

## 7.7 Cloudflare Workers + R2 + Durable Objects

Recommended architecture:

```text
Cloudflare Worker
  API

Durable Object per capsule
  one-time state
  short-lived grants
  cleanup alarm

R2 bucket
  encrypted .age parts
```

Pros:

```text
+ technically excellent fit for v0
+ R2 has no egress fees
+ Durable Objects give per-capsule atomic state
+ no server maintenance
+ very low cost
```

Cons:

```text
- high RF availability risk
- Cloudflare officially reports Russian ISP throttling
- platform-specific architecture
```

Pricing snapshot:

```text
Workers Paid minimum: around $5/month
R2 storage: usage-based with free tier; no egress fees
Durable Objects: compute/storage usage; inactive objects do not run
```

Research source:

```text
https://developers.cloudflare.com/workers/platform/pricing/
https://developers.cloudflare.com/durable-objects/platform/pricing/
https://developers.cloudflare.com/r2/pricing/
https://developers.cloudflare.com/support/troubleshooting/general-troubleshooting/service-disruption/
https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/
```

Recommendation:

```text
Do not use as sole endpoint if Russia access is required.
```

---

## 7.8 Supabase

Recommended architecture:

```text
Supabase Edge Functions
  create/finalize/redeem

Supabase Postgres
  capsules
  capsule_parts
  redeem_grants

Supabase Storage
  encrypted .age parts
```

Pros:

```text
+ excellent SQL transparency
+ dashboard
+ private buckets and signed URLs
+ Auth/team features later
+ easy to build product UI later
```

Cons:

```text
- egress can dominate cost
- Pro/project baseline
- Edge Functions have runtime limits; avoid proxying large blobs
```

Pricing snapshot:

```text
Pro starts around $25/month
Pro includes egress/file storage quotas
Storage egress overage can be material
Edge Functions have quota then per-million pricing
```

Research source:

```text
https://supabase.com/pricing
https://supabase.com/docs/guides/functions/pricing
https://supabase.com/docs/guides/platform/manage-your-usage/egress
https://supabase.com/docs/reference/javascript/storage-from-createsignedurls
```

Recommendation:

```text
Best if SQL dashboard and future app/auth features matter more than raw egress economics.
```

---

## 7.9 AWS S3 + DynamoDB + Lambda

Recommended architecture:

```text
Lambda / API Gateway or Function URLs
DynamoDB
S3 private bucket
```

Pros:

```text
+ mature infrastructure
+ S3 presigned URLs are standard
+ DynamoDB conditional writes solve atomic one-time redeem
+ lifecycle/TTL primitives are mature
```

Cons:

```text
- IAM/API Gateway/Lambda/DynamoDB complexity
- egress can dominate
- less simple than VPS/SQLite
```

Research source:

```text
https://aws.amazon.com/s3/pricing/
https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html
https://aws.amazon.com/dynamodb/pricing/
https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/WorkingWithItems.html
```

Recommendation:

```text
Enterprise/serverless option, not simplest v0.
```

---

## 7.10 Tigris + Upstash + Worker/API

Recommended architecture:

```text
Cloudflare Worker / Fly app / small API
Upstash Redis
Tigris object storage
```

Pros:

```text
+ Tigris has no egress fees
+ Upstash Redis is easy for short-lived tokens/grants
+ composable
+ low cost
```

Cons:

```text
- more moving parts than Cloudflare-only or VPS-only
- Redis state is less self-documenting than SQL
- RF access depends on selected API host and Tigris path
```

Research source:

```text
https://www.tigrisdata.com/pricing/
https://www.tigrisdata.com/docs/overview/
https://upstash.com/pricing/redis
https://upstash.com/docs/redis/features/restapi
```

Recommendation:

```text
Good modular fallback if you dislike Durable Objects but want low egress cost.
```

---

## 7.11 Bunny.net

Best use:

```text
storage/CDN layer, not primary backend state machine
```

Pros:

```text
+ cheap CDN bandwidth
+ storage pricing is simple
+ potential Russia PoPs/signals, but must be verified
+ useful future download acceleration layer
```

Cons:

```text
- not a full backend/state system
- one-time semantics become more complex with CDN/cache
```

Research source:

```text
https://bunny.net/pricing/
https://bunny.net/pricing/storage/
```

Recommendation:

```text
Consider later as encrypted download acceleration, not as first backend.
```

---

## 7.12 Backblaze B2

Best use:

```text
object storage paired with API/state hosted elsewhere
```

Pros:

```text
+ good object storage pricing
+ free egress up to 3x average monthly storage
+ unlimited free egress through some CDN/compute partners
+ S3-compatible
```

Cons:

```text
- no built-in state/backend
- pairing with another provider increases moving parts
- RF access must be tested
```

Research source:

```text
https://www.backblaze.com/cloud-storage/pricing
https://www.backblaze.com/cloud-storage
```

Recommendation:

```text
Good storage component, not complete v0 stack by itself.
```

---

## 7.13 Akamai / Linode

Recommended architecture:

```text
Akamai/Linode VPS
  API + SQLite

Akamai Object Storage
  encrypted parts
```

Pros:

```text
+ simple VPS + object storage model
+ object storage has flat-rate entry tier
+ first 1 TB egress/month free on Akamai cloud pricing page
+ mature network
```

Cons:

```text
- RF access must be tested
- not as obviously cheap/simple as Hetzner/DO
```

Pricing snapshot:

```text
Object Storage: $5/month includes 250 GB according to Akamai tech docs
Cloud pricing page: $0.02/GB-month and first 1 TB egress free, then $0.005/GB
```

Research source:

```text
https://techdocs.akamai.com/cloud-computing/docs/object-storage-pricing
https://www.akamai.com/cloud/pricing
```

Recommendation:

```text
Viable DO-like alternative.
```

---

## 7.14 Vultr

Recommended architecture:

```text
Vultr VPS
Vultr Object Storage
```

Pros:

```text
+ straightforward cloud/VPS provider
+ S3-compatible object storage
```

Cons:

```text
- object storage pricing is less compelling
- no obvious advantage for Capsule v0
- RF access must be tested
```

Pricing snapshot:

```text
Standard Object Storage around $18/month tier in current docs/product page
```

Research source:

```text
https://www.vultr.com/products/object-storage/
https://docs.vultr.com/support/platform/billing/how-is-object-storage-billed
```

Recommendation:

```text
Use only if already on Vultr.
```

---

## 7.15 Yandex Cloud

Recommended architecture:

```text
Yandex Cloud API/Compute/Serverless
Yandex Object Storage
```

Pros:

```text
+ RF-first access and payments
+ S3-compatible object storage
+ local network path for Russia
```

Cons:

```text
- weaker global trust story for some international users
- compliance/contract/product-positioning issues
- not ideal as global-primary for privacy/security developer tool
```

Research source:

```text
https://yandex.cloud/en/docs/storage/pricing
https://yandex.cloud/en/prices
```

Recommendation:

```text
Use as RF-first deployment or fallback, not necessarily global primary.
```

---

## 7.16 Selectel

Recommended architecture:

```text
Selectel Cloud/API
Selectel S3
```

Pros:

```text
+ RF infrastructure
+ S3-compatible storage
+ likely better Russia network path
```

Cons:

```text
- global availability/trust positioning weaker than EU/global providers
- pricing details require provider calculator/account context
```

Research source:

```text
https://docs.selectel.ru/en/s3/
https://docs.selectel.ru/en/s3/about/payment/
```

Recommendation:

```text
Potential RF fallback or RF-primary if most users are in Russia.
```

---

## 7.17 Firebase / GCP

Recommended architecture:

```text
Cloud Functions / Cloud Run
Firestore
Cloud Storage
```

Pros:

```text
+ mature BaaS/serverless ecosystem
+ Firestore TTL policies
+ Cloud Storage rules
+ good for web/mobile app layer
```

Cons:

```text
- not CLI-first
- Firebase-specific model
- egress/pricing complexity
- RF access must be tested
```

Research source:

```text
https://firebase.google.com/docs/functions
https://firebase.google.com/docs/firestore/ttl
https://firebase.google.com/docs/storage/security
```

Recommendation:

```text
Not first choice for Capsule v0, unless product already lives in Firebase.
```

---

## 7.18 Convex / InstantDB

Best use:

```text
future product app layer:
  dashboard
  teams
  shared-with-me
  realtime UI
  auth
```

Pros:

```text
+ very fast product development
+ database + storage + functions/app model
+ great for UI state and collaboration-like features
```

Cons:

```text
- app-platform lock-in
- not a dumb object storage
- admin/API token handling must be server-side
- one-time redeem semantics still need careful modeling
```

Research source:

```text
https://www.instantdb.com/product/storage
https://www.instantdb.com/docs/storage
https://docs.convex.dev/file-storage/overview
https://docs.convex.dev/scheduling/scheduled-functions
```

Recommendation:

```text
Do not start here for CLI-first v0.
Consider later if Capsule becomes a broader product with UI/team workflows.
```

---

## 7.19 Vercel Blob / Render / Railway

Use case:

```text
web app frontend/dashboard, not primary blob exchange path
```

Notes:

```text
Vercel Blob can handle large object workflows, but function request limits mean direct upload flows must be designed carefully.
Render/Railway are fine app platforms, but bandwidth/egress pricing can make them poor choices for download-heavy encrypted capsules.
```

Recommendation:

```text
Use for UI if already on these platforms; avoid as first backend for capsule blob path.
```

---

## 8. Cost scenarios

Assumptions:

```text
1 capsule = 4 encrypted parts
1 sender upload
1 receiver download
TTL = 24h
average retained storage = monthly uploaded GB / 30
```

Example scenarios:

| Scenario | Capsules/month | Avg encrypted size | Download/month | Avg retained storage |
|---|---:|---:|---:|---:|
| Alpha | 1,000 | 1 MB | ~1 GB | ~0.03 GB-month |
| Small beta | 50,000 | 2 MB | ~98 GB | ~3.3 GB-month |
| Active beta | 500,000 | 5 MB | ~2.4 TB | ~81 GB-month |
| Scale | 1,000,000 | 5 MB | ~4.9 TB | ~163 GB-month |

Interpretation:

```text
TTL 24h means storage cost is usually small.
Download egress and per-operation cost dominate at scale.
```

Rough monthly cost tendencies:

| Stack | Alpha | Small beta | Active beta | Scale |
|---|---:|---:|---:|---:|
| Hetzner VPS + Object Storage | ~€9–12 | ~€9–15 | depends on egress over 1 TB | depends on egress |
| DigitalOcean Droplet + Spaces | ~$9–15 | ~$9–20 | egress over included quota matters | egress matters |
| Fly + Tigris | ~$5–20 | ~$5–25 | likely moderate | likely moderate |
| Cloudflare R2 + Workers + DO | ~$5 | ~$5–10 | low | low, but RF risk |
| Supabase | ~$25–30 | ~$25–30 | can reach hundreds due egress | can reach hundreds |
| AWS S3 + DynamoDB + Lambda | low on alpha | low on beta | egress dominates | egress dominates |
| Gcore | higher | higher | egress + requests | egress + requests |

---

## 9. Availability testing plan

Before picking a final provider, deploy the same tiny test service to each candidate.

Test endpoints:

```text
GET  /healthz
GET  /test-1mb.bin
GET  /test-10mb.bin
POST /echo-1mb
POST /echo-10mb
```

Test locations:

```text
Russia:
  Moscow
  Saint Petersburg
  mobile: MTS / Beeline / MegaFon / Tele2
  broadband: Rostelecom + at least one local ISP

Outside Russia:
  Amsterdam / Frankfurt
  London
  US East
  US West
  Asia, if relevant
```

Metrics:

```text
DNS resolution success
TLS handshake success
TTFB
download throughput
upload throughput
failure rate
packet loss / timeout
repeatability across days
```

Decision rule:

```text
If a provider is slow but stable, it can work for Capsule v0 because blobs are small.
If TLS/connectivity intermittently fails, reject it.
If download is capped/throttled in Russia, reject it as primary.
```

---

## 10. Final recommendation

### Start with this

```text
Hetzner VPS + SQLite + Hetzner Object Storage
```

Implementation:

```text
Go API or TypeScript/Fastify API
SQLite on local disk
object storage for encrypted .age parts
cron cleanup
single EU region first
```

Why:

```text
maximum transparency
low price
few moving parts
good enough global/EU path
not Cloudflare-dependent
easy migration later
```

### Keep this as backup

```text
DigitalOcean Droplet + Spaces
```

Use if:

```text
DO dashboard/DX is preferable
account/payment is easier
network tests from Russia pass
```

### Use this if deployment DX matters

```text
Fly.io + Tigris
```

Use if:

```text
global app deployment and GitOps-like deploy are more valuable than VPS transparency
```

### Use this if RF reach is the blocker

```text
Gcore
```

Use if:

```text
Hetzner/DO/Fly fail Russia availability tests
```

### Avoid as sole primary for RF

```text
Cloudflare Workers + R2 + Durable Objects
```

Reason:

```text
excellent architecture and pricing, but official RF throttling risk.
```

---

## 11. Migration path

Keep a storage abstraction from day one:

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

Initial implementation:

```text
SQLiteCapsuleState + S3CompatibleBlobStore
```

This lets you move between:

```text
Hetzner Object Storage
DigitalOcean Spaces
OVH Object Storage
Tigris
Backblaze B2
AWS S3
Yandex/Selectel S3
```

without changing the `/capsule --send` and `/capsule --load` UX.

---

## 12. Security checklist for any provider

Required:

```text
private bucket
no public list
no long-lived public object URLs
signed upload/download URLs or API-mediated access
short TTL redeem grants
hashed redeem tokens in state DB
no plaintext titles unless encrypted
no URL fragments sent to backend
no admin storage keys in local scripts
server-side rate limits
strict max object size
delete expired objects and metadata
minimal logging
IP/user-agent hashing or truncation
```

Local script must still do:

```text
secret scan before encryption
compress before encryption
age encrypt locally
parse #agekey locally
never send #agekey to backend
delete temp plaintext files
```

---

## 13. Source index

Primary Capsule context:

```text
capsule_v0_design(1).md
```

Official/product sources checked:

```text
Cloudflare Workers pricing:
https://developers.cloudflare.com/workers/platform/pricing/

Cloudflare Durable Objects pricing:
https://developers.cloudflare.com/durable-objects/platform/pricing/

Cloudflare Russia service disruption:
https://developers.cloudflare.com/support/troubleshooting/general-troubleshooting/service-disruption/

Cloudflare blog on Russia throttling:
https://blog.cloudflare.com/russian-internet-users-are-unable-to-access-the-open-internet/

Supabase pricing:
https://supabase.com/pricing

Supabase Edge Functions pricing:
https://supabase.com/docs/guides/functions/pricing

Supabase egress:
https://supabase.com/docs/guides/platform/manage-your-usage/egress

DigitalOcean Droplets pricing:
https://www.digitalocean.com/pricing/droplets

DigitalOcean Spaces pricing:
https://www.digitalocean.com/pricing/spaces-object-storage

DigitalOcean regional availability:
https://docs.digitalocean.com/platform/regional-availability/

Hetzner Cloud:
https://www.hetzner.com/cloud

Hetzner Object Storage:
https://www.hetzner.com/storage/object-storage/

Fly pricing:
https://fly.io/docs/about/pricing/

Fly Tigris:
https://fly.io/docs/tigris/

Tigris pricing:
https://www.tigrisdata.com/pricing/

Tigris overview:
https://www.tigrisdata.com/docs/overview/

OVHcloud Object Storage:
https://www.ovhcloud.com/en/public-cloud/object-storage/

OVHcloud public cloud prices:
https://www.ovhcloud.com/en/public-cloud/prices/

Scaleway storage pricing:
https://www.scaleway.com/en/pricing/storage/

Scaleway object storage FAQ:
https://www.scaleway.com/en/docs/object-storage/faq/

Gcore storage:
https://gcore.com/storage

Gcore storage docs:
https://gcore.com/docs/storage

Gcore storage billing:
https://gcore.com/docs/storage/how-storage-is-billed

Bunny pricing:
https://bunny.net/pricing/

Bunny storage pricing:
https://bunny.net/pricing/storage/

Backblaze B2 pricing:
https://www.backblaze.com/cloud-storage/pricing

Backblaze B2:
https://www.backblaze.com/cloud-storage

Akamai cloud pricing:
https://www.akamai.com/cloud/pricing

Akamai object storage pricing:
https://techdocs.akamai.com/cloud-computing/docs/object-storage-pricing

Vultr object storage:
https://www.vultr.com/products/object-storage/

Vultr object storage billing:
https://docs.vultr.com/support/platform/billing/how-is-object-storage-billed

Yandex Cloud Object Storage pricing:
https://yandex.cloud/en/docs/storage/pricing

Yandex Cloud prices:
https://yandex.cloud/en/prices

Selectel S3:
https://docs.selectel.ru/en/s3/

Selectel S3 payment:
https://docs.selectel.ru/en/s3/about/payment/

InstantDB Storage:
https://www.instantdb.com/product/storage

InstantDB storage docs:
https://www.instantdb.com/docs/storage

Convex file storage:
https://docs.convex.dev/file-storage/overview

Convex scheduled functions:
https://docs.convex.dev/scheduling/scheduled-functions

Firebase Functions:
https://firebase.google.com/docs/functions

Firestore TTL:
https://firebase.google.com/docs/firestore/ttl

Firebase Storage security:
https://firebase.google.com/docs/storage/security
```
