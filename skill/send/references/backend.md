# Backend HTTP API (v1) — reference

Mirrors `.archcore/specs/backend-http-api.spec.md`. The skill scripts talk to
these endpoints; the server is a zero-knowledge ciphertext rendezvous.

Base path `/v1`. JSON for metadata; `application/octet-stream` for part bytes.
The URL fragment is **never** part of any request.

## Endpoints

**Health** — `GET /healthz` → `200 {"ok":true}`

**Create** — `POST /v1/sends`
```json
{ "version":"send.v1", "one_time":true, "ttl_seconds":86400,
  "parts":[ {"part_id":"manifest","encrypted_size":1200,"sha256":"…"},
            {"part_id":"part_0001","encrypted_size":28412,"sha256":"…"} ] }
```
→ `201 { "id":"snd_…", "upload_urls":{…}, "public_url":"…/s/snd_…",
         "expires_at":"…", "one_time":true }`

**Upload part** — `PUT /v1/sends/{id}/parts/{part_id}`
body = ciphertext; headers `Content-Type: application/octet-stream`,
`X-Send-Ciphertext-Sha256: <hex>`. Server verifies byte count + sha256
(`422 INTEGRITY_FAILED` on mismatch). Idempotent on identical re-PUT.

**Finalize** — `POST /v1/sends/{id}/finalize` → `200`. Requires all parts present.
The client then appends `#agekey=…` to the public URL **locally**.

**Get public metadata** — `GET /v1/sends/{id}` → `200` (sizes + sha256 only, no
semantics). `404` if unknown/deleted.

**Redeem (one-time)** — `POST /v1/sends/{id}/redeem`
→ `200 { "redeem_token":"red_…", "expires_at":"…(+10m)", "parts":[…] }`.
First redeem atomically consumes the link; subsequent → `410
SEND_ALREADY_REDEEMED`. Expired → `410 SEND_EXPIRED`.

**Download part** — `GET /v1/sends/{id}/parts/{part_id}` with
`Authorization: Bearer red_…` → `200` ciphertext. Any part may be fetched
repeatedly within the grant window (supports lazy details).

## Errors

Client maps server statuses to its own error objects + exit codes; see
`.archcore/specs/error-catalog.spec.md` for the full table.
