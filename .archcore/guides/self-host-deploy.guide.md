---
title: "Self-Hosting, Deploying & Operating the Send Server"
status: accepted
---

## Prerequisites

- A small VPS (1 vCPU / 1–2 GB is plenty for v0) with a public IP, a domain (or any hostname that resolves to the box), and TLS. Any mainstream VPS provider works — pick one reachable and performant for your audience ([[hosting-posture-availability]]).
- Docker + Docker Compose on the box. The reference deploy builds `sendd` from source ([[server-implementation]]); no prebuilt image is required.
- Storage choice: filesystem (default) or S3-compatible object storage ([[storage-abstraction]]).

## Steps

### 1. Point DNS at the box
Create an `A` record for your domain → the VPS public IP. It must resolve **before** first start so the ACME (Let's Encrypt) challenge can complete.

```bash
dig +short your-domain.example   # should print the VPS IP
```

### 2. Configure
Copy `deploy/.env.example` to `deploy/.env` and set `SEND_DOMAIN` + `SEND_PUBLIC_URL` to your domain. Tune lifecycle/size/rate knobs ([[size-limits]]) for your audience — an open public instance should stay tight (short TTL, strict caps, per-IP limits); a private/team instance can set `SEND_TEAM_TOKEN` to gate writes.

### 3. Run (Docker Compose — reference deployment)
```bash
cd deploy
docker compose up -d --build
```
Compose builds the static `sendd` binary, runs it on an internal port, and puts Caddy in front for automatic TLS (Let's Encrypt). First certificate issuance takes a few seconds. The same binary can also run behind any other TLS-terminating reverse proxy, or fully managed object storage instead of local disk.

### 4. Object-storage option (any S3-compatible)
Set `SEND_S3_ENDPOINT/BUCKET/REGION/ACCESS_KEY/SECRET_KEY` to keep encrypted parts off local disk. The bucket MUST be **private** — no public listing, no long-lived public object URLs ([[security-privacy]]). Swappable across S3-compatible providers without code changes ([[storage-abstraction]]).

### 5. Operations
- **GC / retention** — the built-in worker deletes expired + consumed sends and unfinished uploads (>15m). Publish the policy: *"Encrypted parts are deleted after expiry or the one-time redemption window; operational logs (no plaintext, no fragments) persist N days."*
- **Observability** — counters `sends_created/finalized/redeemed/expired`, `parts_uploaded/downloaded`, `upload/download_bytes`. Decrypt failures are client-side; do not auto-collect ([[security-privacy]]).
- **Abuse controls** — per-IP rate limits, max sizes/TTL ([[size-limits]]), no directory listing, unguessable ids, background deletion.
- **Backups** — back up the state DB; encrypted blobs are short-lived by design.

### 6. Availability & latency test plan (gates any hosted default)
Before committing to a provider or region, deploy a tiny test service and measure reachability and speed **from the regions and networks your audience actually uses**, plus a few global vantage points.
```text
endpoints: GET /healthz · GET /test-1mb.bin · GET /test-10mb.bin · POST /echo-1mb · POST /echo-10mb
metrics:   DNS · TLS handshake · TTFB · up/down throughput · failure rate · repeatability
decision:  slow-but-stable                       = OK (blobs are small)
           intermittent TLS/conn errors          = reject
           unreachable from a key audience network = reject as primary (try another provider/region)
```

### 7. Security checklist (every provider)
```text
private bucket · no public list · no long-lived public object URLs
signed/API-mediated access only · short-TTL redeem grants · hashed redeem tokens
no plaintext titles · no URL fragments reach backend · no admin storage keys in client scripts
server-side rate limits · strict max object size · delete expired objects+metadata
minimal logging · IP/UA hashing or truncation
```

## Verification
- `curl -fsS https://<your-domain>/healthz` → `{"ok":true}`.
- From an external host: `/send --doctor --server https://<your-domain>` → reachable.
- Full round-trip across two networks.
- `docker compose logs sendd` contains no bodies, tokens, or fragments ([[security-privacy]] R5).
- Test-plan metrics satisfy the decision rule.

## Common Issues
- **413 on upload** → raise the proxy body limit to match `SEND_MAX_TOTAL_BYTES`.
- **CDN / managed edge in front** → can change per-connection limits or reachability for some networks; offer a direct (non-edge) endpoint as a fallback ([[hosting-posture-availability]]).
- **Public object URLs** → lock the bucket; mediate via API or short-TTL post-redeem presign.
- **Clock skew** → TTL/grant checks need correct server time (run NTP).