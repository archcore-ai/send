# Deploying a public Archcore Send instance

The Go server (`sendd`) behind Caddy (automatic TLS), on a DuckDNS subdomain.
Target: `docker compose up -d` and you have a public, one-time, end-to-end-encrypted
send service anyone can use and you can hand links to.

> **Hosting note (2026):** pick a host **reachable from your audience, including
> RF/CIS**. As of 2026 Roskomnadzor blocks/throttles Hetzner, DigitalOcean, OVH and
> Cloudflare(-fronted PaaS), so avoid those if RF reach matters; a CIS-friendly VPS
> (Timeweb / Selectel / VDSina) is the robust choice. See
> `.archcore/decisions/hosting-posture-rf-availability.adr.md`.

## Prerequisites

- A small VPS (1 vCPU / 1–2 GB is plenty) with a **public IP** and Docker.
- Ports **80** and **443** open.
- A free **DuckDNS** subdomain (https://www.duckdns.org).

## 1. Point DuckDNS at the VPS

In the DuckDNS dashboard set your subdomain's IP to the VPS public IP (an `A`
record). It must resolve **before** first start, or Caddy's ACME retries will spam.

```bash
dig +short send-xxxx.duckdns.org   # should print your VPS IP
```

For a dynamic IP, run the DuckDNS updater (cron) on the box; for a static-IP VPS the
one-time set above is enough.

## 2. Configure

```bash
cp deploy/.env.example deploy/.env
# edit deploy/.env: set SEND_DOMAIN and SEND_PUBLIC_URL to your subdomain
```

## 3. Run

```bash
cd deploy
docker compose up -d --build
```

Caddy obtains a Let's Encrypt cert via HTTP-01 automatically. First issuance takes a
few seconds.

## 4. Verify

```bash
curl -fsS https://send-xxxx.duckdns.org/healthz        # {"ok":true}
# from any machine with the skill + age installed:
bash skill/send/scripts/send.sh doctor --server https://send-xxxx.duckdns.org
```

Then a full round-trip: `/send` on one machine → open the link with `/send --load`
on another. Confirm logs carry no secrets:

```bash
docker compose logs sendd | grep -iE 'agekey|bearer|red_'   # expect: no matches
```

## Operations

- **Retention is automatic.** The GC worker deletes expired sends, consumed
  one-time sends past the 10-min grant, unfinished uploads >1h, and orphan blobs.
  Storage is a self-cleaning working set, not an archive.
- **Defaults are tight** for an open instance: 1h default TTL, 24h max, 25 MiB total
  per send, per-IP rate limits. Tune in `.env`.
- **Clock matters.** TTL/grant checks use the host clock — run NTP.
- **Backups.** Only `send-data` (the SQLite DB) is worth backing up; blobs are
  short-lived by design. WAL is checkpointed on shutdown.
- **Abuse.** The store is zero-knowledge, so you cannot inspect content. Mitigations
  are the tight TTL, size caps, and rate limits; you can also delete the data volume
  to purge everything.

## Without Docker

`cd server && CGO_ENABLED=0 go build -o sendd .` then run the binary with the same
`SEND_*` env vars behind any TLS-terminating reverse proxy. See
`.archcore/guides/self-host-deploy.guide.md`.
