---
title: "Self-Hosting, Deploying & Operating the Send Server"
status: draft
---

## Prerequisites

- A small VPS (1 vCPU / 1–2 GB is plenty for v0), a domain, and TLS. Default provider Hetzner ([[hosting-posture-rf-availability]]).
- The `sendd` binary or container ([[server-implementation]]).
- Storage choice: filesystem (default) or S3-compatible object storage ([[storage-abstraction]]).

## Steps

### 1. Run the server (container)
```bash
docker run -d --name sendd -p 8080:8080 \
  -e SEND_PUBLIC_URL=https://send.example.com \
  -e SEND_DB_PATH=/data/sends.db -e SEND_BLOB_DIR=/data/blobs \
  -v send-data:/data  ghcr.io/<org>/sendd:latest
```

### 2. TLS / reverse proxy (Caddy)
```caddy
send.example.com {
  reverse_proxy 127.0.0.1:8080
  request_body { max_size 26MB }   # align with SEND_MAX_TOTAL_BYTES
}
```

### 3. Hetzner reference deployment
- Cloud VPS (EU) + Hetzner Object Storage for encrypted parts; SQLite on the VPS disk for state.
- Point `SEND_S3_*` at the Hetzner Object Storage endpoint to keep blobs off local disk.
- EU location gives reasonable global latency; verify RF reach (step 6).

### 4. Object-storage option (any S3-compatible)
Set `SEND_S3_ENDPOINT/BUCKET/REGION/ACCESS_KEY/SECRET_KEY`. The bucket MUST be **private** — no public listing, no long-lived public object URLs ([[security-privacy]]). Swappable across Hetzner OS / DO Spaces / OVH / Tigris / B2 / S3 without code changes ([[storage-abstraction]]).

### 5. Operations
- **GC / retention** — the built-in worker deletes expired + consumed sends and unfinished uploads (>1h). Publish the policy: *"Encrypted parts are deleted after expiry or the one-time redemption window; operational logs (no plaintext, no fragments) persist N days."*
- **Observability** — counters `sends_created/finalized/redeemed/expired`, `parts_uploaded/downloaded`, `upload/download_bytes`. Decrypt failures are client-side; do not auto-collect ([[security-privacy]]).
- **Abuse controls** — per-IP rate limits, max sizes/TTL ([[size-limits]]), no directory listing, unguessable ids, background deletion.
- **Backups** — back up the state DB; encrypted blobs are short-lived by design.

### 6. RF availability test plan (gates any hosted default)
Deploy a tiny test service; measure from RF (Moscow, SPb; MTS/Beeline/MegaFon/Tele2; Rostelecom) and outside (AMS/FRA, LON, US-East/West).
```text
endpoints: GET /healthz · GET /test-1mb.bin · GET /test-10mb.bin · POST /echo-1mb · POST /echo-10mb
metrics:   DNS · TLS handshake · TTFB · up/down throughput · failure rate · repeatability
decision:  slow-but-stable = OK (blobs are small)
           intermittent TLS/conn failures = reject
           throttled-in-RF = reject as primary (escalate to Gcore / Yandex / Selectel)
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
- From an external host: `/send --doctor --server https://send.example.com` → reachable.
- Full round-trip across two networks.
- `docker logs sendd` contains no bodies, tokens, or fragments ([[security-privacy]] R5).
- RF test metrics satisfy the decision rule.

## Common Issues
- **413 on upload** → raise the proxy body limit to match `SEND_MAX_TOTAL_BYTES`.
- **Cloudflare in front** → RF throttling risk; provide a non-CF endpoint ([[hosting-posture-rf-availability]]).
- **Public object URLs** → lock the bucket; mediate via API or short-TTL post-redeem presign.
- **Clock skew** → TTL/grant checks need correct server time (run NTP).