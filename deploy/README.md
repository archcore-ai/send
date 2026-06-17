# Deploying an Archcore Send instance

The Go server (`sendd`) behind Caddy (automatic TLS). Target: `docker compose up -d` and you
have a self-hosted, one-time, end-to-end-encrypted send service you can hand links to.

> **Hosting:** the design is provider- and storage-agnostic — any VPS with a public IP and
> Docker works. Pick a provider and region that are **reachable and performant for your
> audience**, and validate empirically with the availability test plan in
> `.archcore/guides/self-host-deploy.guide.md`. Don't rely on a single managed edge/CDN as the
> only endpoint where broad reach matters; pair it with a direct fallback. See
> `.archcore/decisions/hosting-posture-availability.adr.md`.

## Prerequisites

- A small VPS (1 vCPU / 1–2 GB is plenty) with a **public IP** and Docker + Docker Compose.
- Ports **80** and **443** open.
- A domain or hostname that resolves to the VPS — a subdomain you own, or a free dynamic-DNS
  name (e.g. DuckDNS). Many VPS providers also assign a usable host that already resolves to
  your IP out of the box.

## 1. Point DNS at the VPS

Create an `A` record for your hostname → the VPS public IP. It must resolve **before** first
start, or Caddy's ACME (Let's Encrypt) challenge will retry/fail.

```bash
dig +short your-domain.example   # should print your VPS IP
```

For a dynamic IP, run a dynamic-DNS updater on the box; for a static-IP VPS the one-time set
above is enough.

## 2. Configure

```bash
cp deploy/.env.example deploy/.env
# edit deploy/.env: set SEND_DOMAIN and SEND_PUBLIC_URL to your hostname
```

`.env` is git-ignored — keep it that way; never commit it (it may hold a `SEND_TEAM_TOKEN`).

## 3. Run

```bash
cd deploy
docker compose up -d --build
```

Caddy obtains a Let's Encrypt certificate via the ACME challenge automatically. First issuance
takes a few seconds.

## 4. Verify

```bash
curl -fsS https://your-domain.example/healthz        # {"ok":true}
# from any machine with the skill + age installed:
bash skill/send/scripts/send.sh doctor --server https://your-domain.example
```

Then a full round-trip: `/send` on one machine → open the link with `/send --load` on another.
Confirm logs carry no secrets:

```bash
docker compose logs sendd | grep -iE 'agekey|bearer|red_'   # expect: no matches
```

## Operations

- **Retention is automatic.** The GC worker deletes expired sends, consumed one-time sends past
  the 10-min grant, unfinished uploads >15m, and orphan blobs. Storage is a self-cleaning working
  set, not an archive.
- **Open vs private.** Leaving `SEND_TEAM_TOKEN` empty makes an **open** instance: anyone with a
  link can read/redeem, and writes are anonymous (bounded only by TTL, size caps, and per-IP rate
  limits). Set `SEND_TEAM_TOKEN` to require a bearer token on writes.
- **Defaults are tight** for an open instance: 1h default TTL, 24h max, 25 MiB total per send,
  per-IP rate limits. Tune in `.env`.
- **Clock matters.** TTL/grant checks use the host clock — run NTP.
- **Backups.** Only `send-data` (the SQLite DB) is worth backing up; blobs are short-lived by
  design. WAL is checkpointed on shutdown.
- **Firewall note.** Docker publishes container ports directly via iptables, which **bypasses
  host `ufw` rules** — only the ports declared in `docker-compose.yml` are reachable, regardless
  of ufw. Add or remove exposure by editing the Compose `ports`, not ufw.

## Without Docker

`cd server && CGO_ENABLED=0 go build -o sendd .` then run the binary with the same `SEND_*` env
vars behind any TLS-terminating reverse proxy. See
`.archcore/guides/self-host-deploy.guide.md`.
