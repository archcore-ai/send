# Send format (`send.v1`) — reference

Mirrors `.archcore/specs/send-format.spec.md`. Authoritative source is the spec.

## Pipeline (per part)

```text
plaintext bytes → gzip → age -r <ephemeral recipient> → ciphertext (.age)
```

- Compress **before** encrypt; never compress ciphertext.
- Each part is encrypted independently to the same per-send ephemeral recipient.
- `ciphertext_sha256` is computed over the final ciphertext and sent on upload.

## Parts

Each part has:
- a **semantic id** (client-meaningful): `compact`, `evidence.errors`, `detail.full-diff`, …
- an **opaque transport id** the server sees: `manifest` (reserved) + `part_0001`,
  `part_0002`, … (zero-padded, assigned in creation order).
- `kind` ∈ `manifest | markdown | patch | log | json | text | binary`.
- `required` (bool), `load_by_default` (bool).

The server receives only transport ids, sizes, and sha256 — **no** semantic ids,
kinds, or flags. All semantics live inside the encrypted `manifest` part.

## Private manifest (encrypted → part `manifest`)

```json
{
  "version": "send.v1",
  "title": "Auth debugging handoff",
  "created_at": "2026-06-11T12:00:00Z",
  "source": { "agent": "archcore-send-skill" },
  "policy": {
    "raw_transcript_included": false,
    "secrets_included": false,
    "default_load": ["compact", "evidence.errors"]
  },
  "parts": [
    {"id":"compact","transport_id":"part_0001","kind":"markdown","required":true,"load_by_default":true,"plaintext_size":42000},
    {"id":"evidence.errors","transport_id":"part_0002","kind":"markdown","required":true,"load_by_default":true,"plaintext_size":18000},
    {"id":"detail.full-diff","transport_id":"part_0003","kind":"patch","required":false,"load_by_default":false,"plaintext_size":2100000}
  ]
}
```

`source.*` is optional and omittable for privacy.

## Loader behavior

- Fetch + decrypt `manifest` first, then `compact` and parts with
  `load_by_default: true`.
- **Never** auto-fetch `load_by_default: false` parts — those load only via
  `load-detail`.
- Refuse an unknown major version with `UNSUPPORTED_VERSION`.

## Caps (from size-limits.rule)

| Scope | Soft | Hard |
|---|--:|--:|
| `compact` (plaintext) | 30 KB | 50 KB |
| required evidence total | 300 KB | 800 KB |
| single `detail.*` | 10 MB | 50 MB |
| total send (encrypted) | 10 MB | 25 MB |

`compact` / evidence hard caps are **not** overridable (split into `detail.*`).
`detail.*` / total may be forced with `--include-large`.
