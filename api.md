# Originless HTTP API

Base URL: **`http://localhost:3232`**

No API keys, accounts, or authentication.

**CORS** — JSON API and dashboard pages send Originless headers: `Access-Control-Allow-Origin: *`, methods `GET, HEAD, POST, OPTIONS`, headers `Content-Type`. **`/ipfs` and `/ipns` (including `{cid}.ipfs.*` hosts) do not.** Those responses are reverse-proxied from Kubo and already include CORS (`GET`/`HEAD`/`OPTIONS`, `Range`, `X-Ipfs-Path`, …). Originless must not add its own `Access-Control-*` on that path: browsers treat a second `Access-Control-Allow-Origin` as invalid and `fetch()` throws `TypeError: Failed to fetch` even on HTTP 200.

**Bodies** — JSON uses `Content-Type: application/json`. JSON and HTML are gzip-compressed when the client sends `Accept-Encoding: gzip` (`HEAD` is not wrapped). `/ipfs` and `/ipns` streams are passed through; Kubo may compress them itself.

Uploads (`POST /upload`, `/media`, `/uploadfolder`) are limited to **3 concurrent requests**. Extra uploads return `503`. Per-file size cap is `STORAGE_MAX / 100` (1 GB when `STORAGE_MAX=100GB`).

---

## Route index

| Method | Path | What it does |
| :----- | :--- | :----------- |
| `GET` | [`/health`](#get-health) | Liveness + IPFS peer count |
| `GET` | [`/status`](#get-status) | Node, storage, gateway, and archive snapshot |
| `POST` | [`/upload`](#post-upload) | Pin a file as-is; returns a CID |
| `POST` | [`/media`](#post-media) | Strip EXIF/GPS/XMP from an image, then pin |
| `POST` | [`/uploadfolder`](#post-uploadfolder) | Pin a directory tree as one root CID |
| `GET` | [`/history`](#get-history) | Paginated upload log |
| `GET` | [`/pins`](#get-pins) | Pinned count, bytes, and janitor threshold |
| `GET` | [`/archive`](#get-archive) | Paginated Nostr IPFS archive listing |
| `GET` | [`/metrics`](#get-metrics) | Prometheus text metrics |
| `GET`/`HEAD` | [`/ipfs/{cid}`](#get-ipfscid--ipnspath) | Serve pinned bytes from this node |
| `GET`/`HEAD` | [`/ipns/{name}`](#get-ipfscid--ipnspath) | Resolve and serve an IPNS name |
| `GET` | [`/api/examples`](#get-apiexamples) | JSON catalog of client tools |
| `GET` | [`/examples/manifest.json`](#get-apiexamples) | Same catalog as `/api/examples` |

Dashboard and Tools pages (`/`, `/examples/`, …) are HTML, not JSON. Archived Nostr media is listed at `GET /archive` and **fetched** at `GET /ipfs/{cid}` like any other pin.

---

## `GET /health`

IPFS swarm check. Use this for container healthchecks and load balancers.

```bash
curl http://localhost:3232/health
```

**200** — daemon is up and has at least one peer:

```json
{ "status": "healthy", "peers": 140 }
```

**503** — daemon down, unreachable, or zero peers:

```json
{ "status": "unhealthy", "peers": 0, "reason": "No peers connected" }
```

---

## `GET /status`

Full node snapshot for the dashboard and operators.

```bash
curl http://localhost:3232/status
```

**200** fields:

| Field | Meaning |
| :---- | :------ |
| `status` | `"success"` |
| `timestamp` | RFC 3339 UTC |
| `bandwidth` | Kubo totals and rates (`totalIn`, `totalOut`, `rateIn`, `rateOut`, `interval`) |
| `repository` | Repo `size`, `storageMax`, `numObjects`, `path`, `version` |
| `node` | Peer `id`, `publicKey`, `agentVersion`, `protocolVersion` |
| `peers.count` | Connected swarm peers |
| `storageLimit` | Configured `STORAGE_MAX` and current repo size |
| `fileLimit` | Per-upload cap (`configured` string, `bytes` integer) |
| `nostrNpubs` | Configured archive accounts |
| `nostrRelays` | Relay URLs used by the archiver |
| `appVersion` | Originless version string |
| `gateway.enabled` / `gateway.serving` | Whether this node serves `/ipfs` and `/ipns` over HTTP |
| `gateway.path` / `gateway.ipns` | `"/ipfs/"` and `"/ipns/"` |
| `gateway.url` / `gateway.ipnsUrl` | Public fetch URLs on this node (omitted when disabled) |
| `gateway.kubo` | Native Kubo gateway URL (omitted when disabled) |
| `archive` | Present when the archiver is wired up (see below) |

`archive` object:

| Field | Meaning |
| :---- | :------ |
| `enabled` | `true` when `NOSTR_NPUBS` is set |
| `count` / `size` / `sizeStr` | Archived objects and bytes |
| `dir` | Always `/archive` |
| `scanMinutes` | Scan interval (15) |
| `repinHours` | Re-pin interval (6) |
| `scanning` / `repinning` | In-progress flags |
| `lastScan` / `lastRepin` | RFC 3339 UTC, or `""` if never |
| `accounts[]` | Per-npub `npub`, `pubkey`, `cursor`, `cursorAt`, `objects`, `size`, `sizeStr`, `events` |

`gateway` when this node is serving content (the default):

```json
{
  "enabled": true,
  "serving": true,
  "path": "/ipfs/",
  "ipns": "/ipns/",
  "url": "http://localhost:3232/ipfs/{cid}",
  "ipnsUrl": "http://localhost:3232/ipns/{name}",
  "kubo": "http://127.0.0.1:8080/ipfs/{cid}"
}
```

When `ENABLE_GATEWAY=false`, `enabled` and `serving` are `false` and the URL fields are omitted.

**503** if Kubo stats cannot be read (`error`, `details`, `status: "failed"`).

---

## `POST /upload`

Pin the exact bytes of a single file. Multipart field name must be **`file`**.

```bash
curl -X POST -F "file=@photo.png" http://localhost:3232/upload
```

**200:**

```json
{
  "status": "success",
  "cid": "QmX...",
  "size": 1048576,
  "type": "image/png",
  "filename": "photo.png",
  "pinned": true
}
```

`pinned` is `false` if the file was added to IPFS but the janitor pin/db insert failed. Fetch the bytes at `GET /ipfs/{cid}`.

**Errors:** `400` no file; `413` over the per-file cap; `503` too many concurrent uploads; `500` IPFS add failed.

---

## `POST /media`

Same multipart shape as `/upload`, but only **JPEG, PNG, GIF, or WebP**. Strips EXIF, GPS, XMP, IPTC, ICC, and text comments; applies EXIF orientation so pixels are upright. WebP is transcoded to JPEG. The CID is the cleaned file, not the camera original. Use `/upload` when you need the original bytes unchanged.

```bash
curl -X POST -F "file=@photo.jpg" http://localhost:3232/media
```

**200:**

```json
{
  "status": "success",
  "cid": "QmX...",
  "filename": "photo.jpg",
  "type": "image/jpeg",
  "size": 98012,
  "originalSize": 1048576,
  "pinned": true,
  "anonymized": true,
  "stripped": ["exif", "gps", "xmp"],
  "orientation": 6,
  "transcoded": false
}
```

**Errors:** same as `/upload`, plus **415** for unsupported types.

---

## `POST /uploadfolder`

Pin a directory as one root CID. Repeat the **`file`** field; each part’s `filename` is the path inside the tree (relative paths stay intact — use this for `dist/` / static sites).

```bash
curl -X POST \
  -F "file=@dist/index.html;filename=index.html" \
  -F "file=@dist/style.css;filename=style.css" \
  http://localhost:3232/uploadfolder
```

**200:**

```json
{
  "status": "success",
  "cid": "QmFolder...",
  "files": 2,
  "size": 4096,
  "pinned": true
}
```

Open the site at `http://localhost:3232/ipfs/{cid}/`. **Errors** match `/upload`.

---

## `GET /history`

Paginated log of uploads this node has pinned (including later-unpinned rows).

| Query | Default | Range |
| :---- | :------ | :---- |
| `limit` | `50` | 1–100 |
| `offset` | `0` | ≥ 0 |

```bash
curl "http://localhost:3232/history?limit=20&offset=0"
```

**200:**

```json
{
  "status": "success",
  "limit": 20,
  "offset": 0,
  "uploads": [
    {
      "id": 1,
      "cid": "QmX...",
      "filename": "photo.png",
      "size": 1048576,
      "created_at": "2026-08-23T12:00:00Z",
      "unpinned": false,
      "unpinned_at": null
    }
  ]
}
```

Invalid `limit`/`offset` values are ignored (defaults apply). **500** if the database read fails.

---

## `GET /pins`

Current pin inventory vs the janitor threshold.

```bash
curl http://localhost:3232/pins
```

**200:**

```json
{
  "status": "success",
  "pinnedCount": 12,
  "pinnedSize": 52428800,
  "pinnedSizeStr": "50.00 MB",
  "storageLimit": "100GB",
  "threshold": 75
}
```

`threshold` is the percent of `STORAGE_MAX` at which the janitor starts evicting oldest (non-archive) pins. **500** on database error.

---

## `GET /archive`

Lists IPFS objects copied from configured Nostr accounts onto `/archive`. Does **not** stream file bytes — use `GET /ipfs/{cid}` for that.

| Query | Default | Range |
| :---- | :------ | :---- |
| `limit` | `50` | 1–200 |
| `offset` | `0` | ≥ 0 |

```bash
curl "http://localhost:3232/archive?limit=50&offset=0"
```

**200:**

```json
{
  "status": "success",
  "items": [
    {
      "cid": "bafybei...",
      "filename": "photo.jpg",
      "mime": "image/jpeg",
      "size": 2097152,
      "sha256": "abc...",
      "source_event_id": "…",
      "source_pubkey": "…",
      "source_url": "ipfs://bafybei...",
      "verified": true,
      "is_dir": false,
      "created_at": "2026-08-23T12:00:00Z"
    }
  ],
  "count": 8,
  "size": 2097152,
  "sizeStr": "2.00 MB",
  "limit": 50,
  "offset": 0,
  "npubs": ["npub1..."]
}
```

If no archiver is running, the response is `{ "status": "success", "items": [] }`. **500** on database error.

---

## `GET /ipfs/{cid}` / `/ipns/{name}`

Path-style HTTP gateway. Reverse-proxies Kubo (`IPFS_GATEWAY`, default `http://127.0.0.1:8080`). Serves **blocks already on this node** (uploads, pins, archive) — not a recursive public gateway.

```bash
curl -O "http://localhost:3232/ipfs/$CID"
curl "http://localhost:3232/ipfs/$CID?filename=photo.png"
curl -O "http://localhost:3232/ipfs/$CID/index.html"
```

Same bytes on Kubo’s native port: `http://localhost:8080/ipfs/{cid}` when `ENABLE_GATEWAY` is on.

Kubo may redirect HTML/directory CIDs to `{cid}.ipfs.localhost:3232` (origin isolation). Originless proxies that host to Kubo so you get the pinned site, not the dashboard.

**Headers** come from Kubo, not Originless. Expect a single `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Methods: GET, HEAD, OPTIONS`, `Allow-Headers` including `Range`, and `Access-Control-Expose-Headers` for `Content-Range`, `X-Ipfs-Path`, `X-Ipfs-Roots`, and stream flags. `OPTIONS` preflight is answered by Kubo.

**404 JSON** when `ENABLE_GATEWAY=false` (Originless CORS, not Kubo’s):

```json
{
  "error": "IPFS gateway is disabled",
  "status": "disabled",
  "hint": "Set ENABLE_GATEWAY=true (the default) to serve /ipfs and /ipns from this node"
}
```

**502** if the Kubo gateway process is unreachable (Originless JSON + CORS). Subresource `Content-Type` is more reliable with `?filename=`.

---

## `GET /metrics`

Prometheus text exposition (`text/plain; version=0.0.4`). Gauges refresh on each scrape.

```bash
curl http://localhost:3232/metrics
```

| Metric | Type | Meaning |
| :----- | :--- | :------ |
| `originless_build_info{version=…}` | gauge | Build version (`1`) |
| `originless_http_requests_total{path=…}` | counter | Requests by path (`/ipfs` and `/ipns` are collapsed) |
| `originless_http_errors_total` | counter | Responses with status ≥ 400 |
| `originless_uploads_total` | counter | Successful upload operations |
| `originless_upload_bytes_total` | counter | Bytes added via upload endpoints |
| `originless_pinned_count` / `_bytes` | gauge | Current janitor-tracked pins |
| `originless_storage_limit_bytes` | gauge | `STORAGE_MAX` |
| `originless_storage_used_bytes` | gauge | Kubo repo size |
| `originless_ipfs_healthy` | gauge | `1` / `0` |
| `originless_ipfs_peers` | gauge | Swarm peer count |
| `originless_gateway_enabled` | gauge | `1` if `/ipfs` is served |
| `originless_archive_count` / `_bytes` | gauge | Permanent Nostr archive |
| `originless_archive_saved_total` / `_bytes_total` | counter | Saved this process |
| `originless_archive_errors_total` | counter | Archive download failures this process |
| `originless_archive_repin_total` / `_errors_total` | counter | Re-pin results this process |

---

## `GET /api/examples`

JSON catalog of HTML client tools under [`examples/`](examples/). `GET /examples/manifest.json` returns the same payload.

```bash
curl http://localhost:3232/api/examples
```

**200:**

```json
{
  "status": "success",
  "count": 7,
  "tools": [
    {
      "file": "upload-file.html",
      "title": "Single File Uploader",
      "description": "…",
      "category": "Storage & Publishing Tools",
      "endpoint": "POST /upload",
      "badge": "pill-post",
      "order": 1
    }
  ]
}
```

Metadata is read from each page’s `<title>` and `<meta name="description|category|endpoint|order">`. **500** if the examples directory cannot be scanned.

---

## UI pages (not JSON)

| Path | Notes |
| :--- | :---- |
| `GET /` | Node dashboard (library, status) |
| `GET /examples/` | Client tools index |
| `GET /examples/{file}` | Individual tool HTML |
| `GET /examples` | `301` → `/examples/` |
| `GET /library.html` | `301` → `/` |
| `GET /archive.html` | Archive UI; `302` → `/` when `NOSTR_NPUBS` is empty |

---

## Shared upload errors

| Status | When |
| :----- | :--- |
| `400` | Missing `file` part, empty filename, or malformed multipart |
| `413` | File larger than `STORAGE_MAX / 100` (`maxSize` is included) |
| `415` | `/media` only: not JPEG/PNG/GIF/WebP |
| `503` | 3 uploads already in flight (`"Server busy"`) |
| `500` | Kubo add/pin failed |

```json
{
  "error": "File too large",
  "message": "file too large: exceeds the maximum allowed size of 1.00 GB",
  "maxSize": "1.00 GB"
}
```
