<div align="center">

# 🌐 Originless

**Private, decentralized, origin-independent file & app hosting powered by IPFS**

[![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)](https://ghcr.io/besoeasy/originless)
[![IPFS](https://img.shields.io/badge/IPFS-65C2CB?style=for-the-badge&logo=ipfs&logoColor=white)](https://ipfs.tech)
[![API](https://img.shields.io/badge/HTTP%20API-api.md-1f6feb?style=for-the-badge)](api.md)
[![License: ISC](https://img.shields.io/badge/License-ISC-blue.svg?style=for-the-badge)](https://opensource.org/licenses/ISC)

**The frictionless storage backend for the modern web** — Drop into DApps, Nostr clients, AI agents, screenshot tools, and pastebins. Durable, cryptographic file hosting with zero API keys or user accounts required.

**[HTTP API →](api.md)** — every route, request, and response (`/upload`, `/media`, `/ipfs/{cid}`, `/status`, `/archive`, `/metrics`, …). No auth.

</div>

---

## ⚡ Quick Start (Docker Compose)

Run a complete Originless node with IPFS integrated:

```bash
docker compose up -d --build
# or pull a published image:
# IMAGE=ghcr.io/besoeasy/originless:latest docker compose up -d
```

`docker-compose.yml` publishes **3232** (API/UI), **8080** (Kubo gateway), and **4001** TCP+UDP (libp2p swarm), and persists **`/data`** (pins + SQLite) plus **`/archive`**.

Equivalent one-liner:

```bash
docker run -d \
  --name originless \
  --restart unless-stopped \
  -p 3232:3232 \
  -p 8080:8080 \
  -p 4001:4001 \
  -p 4001:4001/udp \
  -e STORAGE_MAX=100GB \
  -v originless-data:/data \
  -v originless-archive:/archive \
  ghcr.io/besoeasy/originless:latest
```

> **Podman User?** Replace `docker` with `podman` / `podman compose`.

- Open the **node dashboard** at **[http://localhost:3232](http://localhost:3232)** (library, archive, status)
- Pin files from **[Client Tools](http://localhost:3232/examples/)** (`/upload`, `/media`, `/uploadfolder`) or the **[HTTP API](api.md)**
- Fetch by CID at `http://localhost:3232/ipfs/<cid>` — this node **retrieves from the IPFS swarm** when the blocks are not local yet
- Publish **port 4001** (TCP + UDP) so other Kubo peers can Bitswap **your** pins; without it, only this node's HTTP gateway can serve what you uploaded
- Disable the HTTP gateway with `-e ENABLE_GATEWAY=false` if you do not want this node to serve bytes over HTTP
- On a public shared host that should only serve its own pins, set `-e GATEWAY_NO_FETCH=true`

To also archive IPFS media from Nostr accounts onto `/archive`, pass `NOSTR_NPUBS`:

```bash
NOSTR_NPUBS=npub1...,npub1... docker compose up -d
```

### Cross-node check (two containers)

```bash
docker compose -f docker-compose.dual.yml up -d --build
CID=$(curl -s -X POST -F "file=@README.md" http://localhost:3232/upload | jq -r .cid)
# node-b dials node-a over the compose network, then serves the same CID:
curl -fsS "http://localhost:3233/ipfs/$CID" -o /tmp/from-b.bin
```

## Testing

```bash
docker compose up -d --build
```

The local gateway is **on by default** and **fetches from the swarm** when needed. After an upload:

```bash
CID=$(curl -s -X POST -F "file=@README.md" http://localhost:3232/upload | jq -r .cid)
curl -O http://localhost:3232/ipfs/$CID
# Kubo-native path gateway (same content):
curl -O http://localhost:8080/ipfs/$CID
```

To run upload-and-pin-only (no HTTP content serving):

```bash
ENABLE_GATEWAY=false docker compose up -d
```

## ⚡ Quick Showcase — Upload in One Command

Uploading to Originless is a single `POST /upload` request — **no API keys, no accounts, no auth**:

```bash
curl -X POST -F "file=@my-image.png" http://localhost:3232/upload
```

For photos, `POST /media` strips EXIF/GPS/XMP before pinning so the CID is not the original camera file:

```bash
curl -X POST -F "file=@my-image.jpg" http://localhost:3232/media
```

_Response:_

```json
{
  "status": "success",
  "cid": "QmX...",
  "size": 1048576,
  "type": "image/png",
  "filename": "my-image.png"
}
```

The file is pinned to IPFS and instantly available on the swarm. Fetch it from **this node** (path-style, the [self-hosted gateway swap](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/#pick-your-path)):

- **This node:** `http://localhost:3232/ipfs/QmX...` (disable with `ENABLE_GATEWAY=false`)
- **Native IPFS:** `ipfs://QmX...`
- **Temporary public fallback:** `https://ipfs.io/ipfs/QmX...` — shared, best-effort, no SLA. Do not build production on it.

Your `cid` is the file's cryptographic hash — anyone can verify the bytes match the address with zero trust in this server. Upload an entire folder (static site, `dist/`, asset bundle) as one root CID with [`POST /uploadfolder`](api.md#post-uploadfolder).

### Examples & Client Tools

The dashboard stays a library for this node. Uploads live in [`examples/`](examples/), served at `http://localhost:3232/examples/` (the **Tools** tab). You can also open those pages on their own or deploy them independently:

- **[Single File Uploader](examples/upload-file.html)**: Drag & drop images, video, audio, or binaries to `POST /upload`. Pins the exact bytes, then share `ipfs://` or this node's `/ipfs/{cid}` (public gateways are a fallback).
- **[Anonymized Image Uploader](examples/upload-media.html)**: JPEG, PNG, GIF, or WebP only via `POST /media`. Strips EXIF/GPS/XMP before pinning; the CID is the cleaned file, not the original camera bytes.
- **[Folder & DApp Uploader](examples/upload-folder.html)**: Upload full static websites and React/Vite `dist/` directories to IPFS with intact relative paths under a single root CID.
- **[Share Snippets & Pastebin](examples/snippet.html)**: Upload and pin code snippets, logs, and text pastes to IPFS with syntax highlighting, SHA-256 hashes, and this-node `/ipfs` links.
- **[Kind 20 Picture Post Generator](examples/picture.html)** (Instagram-style photo dump): Batch upload photos to IPFS, preview an interactive carousel feed, generate NIP-68 Kind 20 JSON with NIP-92 `imeta` tags (URL, MIME, SHA-256, dimensions, blurhash, alt), and publish directly or via NoStrudel.
- **[Kind 1 Short Note & Image Post Generator](examples/post.html)**: Compose Nostr text notes with IPFS image attachments and NIP-92 `imeta` tags, preview live note rendering, and publish directly or via NoStrudel.
- **[Kind 30023 Long-Form Article Writer](examples/article.html)**: Write NIP-23 Markdown articles with a cover image and inline IPFS media, preview the rendered post, copy Kind `30023` / draft `30024` JSON, and publish via NIP-07 or NoStrudel.

---

## 🔥 Key Features

- **🌐 Origin-Independent Storage**: Files are pinned to IPFS. Once propagated to peers, content stays online even if your origin node goes offline.
- **🔄 Self-Hosted Gateway**: This node is the [Kubo “retrieve and publish” path](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/#pick-your-path). Swap `https://ipfs.io/ipfs/{CID}` for `http://localhost:3232/ipfs/{CID}` (or `ipfs://`). Public gateways are a temporary fallback, not production infrastructure.
- **🛡️ Legal & Host Protection**: P2P multihash routing is the default sharing model—content lives as `ipfs://CID` on the swarm. An optional HTTP gateway (`/ipfs/<cid>`, on by default) lets this node serve files directly; set `ENABLE_GATEWAY=false` if you do not want to be an HTTP origin.
- **🏠 Zero Domain, HTTPS, or Port Exposure Required**: Runs seamlessly behind NATs, firewalls, home servers, or local environments using IPFS `libp2p` hole-punching. No domain name, public IP, or SSL certificate setup needed!
- **🔒 Zero-Friction & Accountless**: No API keys, passwords, or authentication overhead needed for uploads. Perfect for local dev, public APIs, or AI agent integration.
- **📁 Full Folder & DApp Uploads**: Upload complete static websites, React/Vite build directories (`dist/`), or asset folders with intact relative paths (unlike single-file media servers).
- **🛡️ Tamper-Proof Cryptographic Verification**: Content-addressed by unique IPFS CIDs (`ipfs://QmX...`). Downloads can be cryptographically verified against the hash.
- **🧹 Automated Smart Janitor**: Intelligent disk space management. Keeps uploads pinned for 30 days by default and automatically evicts oldest files at 75% capacity to prevent disk overflow.
- **📡 Nostr IPFS Archive**: Walks configured `npub`s, finds `ipfs://` links, gateway URLs, NIP-92 `imeta` tags, and NIP-94 (kind 1063) file events, and copies verified media onto a separate Docker volume. The janitor never unpins those CIDs. Every 6 hours Originless re-pins them from `/archive` so they stay on the IPFS swarm.
- **🤖 Built for Autonomous AI Agents**: Native endpoint design allows LLMs (Claude, Copilot, Cursor, Custom Agents) to host generated media, code snippets, or sites instantly.

---

## ⚡ Why Originless vs. Nostr Blossom & Centralized Media Servers

| Feature                           | Nostr Blossom / Centralized HTTP Servers                                                               | 🌐 **Originless**                                                                                                     |
| :-------------------------------- | :----------------------------------------------------------------------------------------------------- | :-------------------------------------------------------------------------------------------------------------------- |
| **Domain & Network Setup**        | ❌ **Requires** public domain, SSL certificates (HTTPS), and open public web ports.                    | ✅ **Zero Domain / NAT Ready**. Runs behind home routers, firewalls, or local Docker containers via `libp2p`.         |
| **Hosting Liability & Risk**      | ❌ Server acts as direct HTTP web host (`https://server.com/<hash>`), making operator directly liable. | ✅ **Decentralized Swarm Buffer**. Content is content-addressed (`ipfs://CID`) and distributed over global P2P nodes. Optional local `/ipfs` gateway (on by default; `ENABLE_GATEWAY=false` turns it off). |
| **Link Resilience**               | ❌ Single point of failure. If server operator shuts down, all file links break instantly.             | ✅ **Origin-Independent**. Content propagates across global IPFS swarm and stays alive even if your node shuts down.  |
| **Multi-File & DApp Support**     | ❌ Single flat files/blobs only (photos/videos).                                                       | ✅ **Full Directory & DApp Hosting**. Upload entire Vite/React `dist/` folders or static apps.                        |
| **Trustless Client Verification** | ❌ Relies on server trust.                                                                             | ✅ **Content-Addressed Cryptography**. Verifiable via `@helia/verified-fetch` down to raw data blocks.                |

---

## 🌟 Primary Use Cases

| Use Case                                    | Description                                                                                                                                                               |
| :------------------------------------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **🚀 Decentralized DApp Hosting**           | Drag-and-drop or upload your frontend `dist/` folder to host permanent, censorship-resistant web applications on IPFS.                                                    |
| **💬 Nostr Media Attachments**              | Instant, zero-auth media uploader for decentralized social apps (e.g., [0xchat](https://0xchat.com/)).                                                                    |
| **📡 Personal Nostr IPFS Archive**          | Mirror IPFS media from your (or followed) `npub`s onto a local volume so kind 1063 files, `ipfs://` notes, and gateway links survive even if the original pin disappears. |
| **🤖 Autonomous AI Agent Outputs**          | Give AI assistants the ability to publish reports, interactive charts, and generated images with zero setup or API keys.                                                  |
| **🖼️ Anonymous Image & Screenshot Sharing** | Instant backend for screenshot capture tools, screen recordings, and temporary image uploads.                                                                             |
| **📝 Pastebin & Code Snippet Sharing**      | Share text, log files, or code snippets without fear of link rot or centralized deletion.                                                                                 |
| **📦 Resilient Asset Distribution**         | Distribute software releases, binaries, audio/podcasts, or heavy media over a self-healing P2P network.                                                                   |

---

## 🔄 How It Works

```
 ┌──────────────┐      Upload File / Folder      ┌──────────────────┐
 │ User / Client│ ─────────────────────────────> │  Originless Node │
 └──────────────┘                                └─────────┬────────┘
                                                           │
                                             Pins Content & Generates CID
                                                           │
                                                           ▼
 ┌──────────────┐      Cryptographic CID         ┌──────────────────┐
 │ Public Peers │ <───────────────────────────── │  IPFS P2P Swarm  │
 └──────────────┘    Helia / Gateway Access      └──────────────────┘

 ┌──────────────┐      Notes / kind 1063         ┌──────────────────┐
 │ Nostr Relays │ ─────────────────────────────> │  Originless Node │
 └──────────────┘                                └─────────┬────────┘
                                                           │
                                      Extract CIDs → Kubo fetch → verify
                                                           │
                                                           ▼
                                                 ┌──────────────────┐
                                                 │  /archive volume │
                                                 │  (never GC'd)    │
                                                 └──────────────────┘
```

1. **Upload**: Send single files or directory trees via the Web UI or simple HTTP POST endpoint.
2. **Pin & Distribute**: Originless pins the content locally on IPFS and broadcasts it to global P2P peers.
3. **Automated Lifecycle**: Uploads stay pinned for `PIN_EXPIRY_DAYS` (default 30). An automated janitor reconciles storage and evicts oldest pins at 75% of `STORAGE_MAX`.
4. **Nostr Archive** (optional): When `NOSTR_NPUBS` is set, Originless scans those accounts, downloads discovered IPFS objects through Kubo (gateway fallback + `x` sha256 when needed), and writes them to `/archive`. It pins each CID immediately and re-pins the whole archive every 6 hours. The janitor skips archive CIDs (SQLite skip list — no Kubo config changes).

---

## 📡 Replace Public Gateways With This Node

Public gateways at `ipfs.io` and `dweb.link` are a shared public good: best-effort, no SLA, shared rate limits that can throttle without notice. [IPFS recommends](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/#pick-your-path) fetching by CID from infrastructure you control, and moving off those hosts incrementally.

**Originless is the Kubo row of that guide** — retrieve *and* publish. You add content with `POST /upload` / `/media` / `/uploadfolder`, keep it pinned, and serve it from a path-style gateway on this node.

| Where you use IPFS | What you do | Official path | Originless |
| :----------------- | :---------- | :------------ | :--------- |
| Browser (web page) | Retrieve by CID (`fetch`, Service Worker, `<img>` / `<video>`) | [`@helia/verified-fetch`](https://github.com/ipfs/helia-verified-fetch) | Same. Pass `ipfs://{CID}`; optionally set `gateways` to this node |
| Backend (server, script, CLI, mobile) | Retrieve only | [Rainbow](https://github.com/ipfs/rainbow) | Not this product. Use Rainbow if you only fetch and never pin |
| Backend (server, script, CLI, mobile) | Retrieve **and** publish | [Kubo](https://docs.ipfs.tech/install/command-line/) | **This node.** Pin here, then `GET /ipfs/{CID}` |

Every non-browser path ends with the same drop-in change: swap the public URL for a **path-style** URL on a gateway you run. Subdomain URLs (`{CID}.ipfs.dweb.link`) cost an extra redirect for origin isolation that `curl` and servers do not use.

```text
https://ipfs.io/ipfs/{CID}/path
https://{CID}.ipfs.dweb.link/path
        ↓
http://localhost:3232/ipfs/{CID}/path
http://127.0.0.1:8080/ipfs/{CID}/path
```

```bash
CID=$(curl -s -X POST -F "file=@README.md" http://localhost:3232/upload | jq -r .cid)

# Same bytes, from this node instead of ipfs.io
curl -sO "http://localhost:3232/ipfs/$CID"
curl -s "http://localhost:3232/ipfs/$CID" | sha256sum
```

Keep a public gateway as a **temporary fallback** while you cut over one call site at a time. Production traffic should hit this node (or `ipfs://`), not `ipfs.io` / `dweb.link`.

### 🌐 This Node's HTTP Gateway (default on)

Originless reverse-proxies Kubo's path gateway so pinned files and folder sites are served from the same origin as the UI:

```text
http://localhost:3232/ipfs/QmX...?filename=document.pdf
http://localhost:3232/ipfs/QmFolderCid/
http://localhost:8080/ipfs/QmX...
```

Kubo may redirect directory and HTML CIDs to `{cid}.ipfs.localhost:3232` (origin isolation). Originless proxies that host to Kubo so you get the pinned site, not the dashboard. `*.localhost` resolves to loopback in the browser; no extra DNS is required.

The gateway retrieves missing blocks from the IPFS swarm (Bitswap/DHT) and then serves them — the Kubo “retrieve and publish” path. That is how a self-hosted node can open media pinned on another Originless/Kubo peer.

Set `GATEWAY_NO_FETCH=true` only if this host must **only** serve blocks it already holds (uploads, pins, archive) and must not act as a recursive public gateway.

Swarm reachability is separate from the HTTP gateway: other Kubo nodes pull your pins over **libp2p port 4001**. Publish TCP+UDP `4001` in Docker, and set `SWARM_ANNOUNCE` to your public multiaddrs when you are behind NAT. Optional sticky dials: `IPFS_PEER_NODES=http://other-node:3232` (reads `/status` for the peer id) or `IPFS_SWARM_CONNECT=/ip4/…/tcp/4001/p2p/…`.

Set `ENABLE_GATEWAY=false` to return `404` on `/ipfs` and `/ipns` and bind Kubo’s gateway to localhost only.

Internal backends should treat this like Redis, not a CDN: call `http://127.0.0.1:3232/ipfs/{CID}` (or `:8080`) over localhost. Putting the gateway on a public hostname is a different threat model (deserialized responses, abuse, rate limits) — see [serve only verifiable responses](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/#serve-only-verifiable-responses-on-a-public-domain) if you expose it.

### 🛡️ Browser apps: [`@helia/verified-fetch`](https://github.com/ipfs/helia-verified-fetch)

If JavaScript in a web page fetches IPFS content, do not `fetch('https://ipfs.io/ipfs/...')`. Use [**`@helia/verified-fetch`**](https://github.com/ipfs/helia-verified-fetch): it is a drop-in for `fetch` that retrieves over libp2p (and optional HTTP trustless gateways) and **verifies every byte against the CID**.

```bash
npm install @helia/verified-fetch
```

```javascript
import { verifiedFetch, createVerifiedFetch } from "@helia/verified-fetch";

// Public gateway URL → content address
//   https://ipfs.io/ipfs/{CID}/path  →  ipfs://{CID}/path
const cid = "QmX...";
const response = await verifiedFetch(`ipfs://${cid}`);
const fileBlob = await response.blob();

// Production: point the HTTP fallback at this Originless node
// (browsers on HTTPS pages need an HTTPS gateway origin).
const verified = await createVerifiedFetch({
  gateways: ["http://127.0.0.1:3232"],
});
const local = await verified(`ipfs://${cid}`);
```

`verifiedFetch` accepts `ipfs://`, `ipns://`, `/ipfs/`, `/ipns/`, and `dnslink://` — not `http(s)` gateway URLs. For HTML/JS/CSS/SVG subresources, pass `?filename=` so the response gets a usable `Content-Type`.

Out of the box, verified-fetch still uses public **delegated routing** (`delegated-ipfs.dev`) and a public **trustless gateway** (`trustless-gateway.link`). A URL swap alone is not enough for the browser path. For production, point `gateways` at this node (or Rainbow) and `routers` at your own [`/routing/v1`](https://docs.ipfs.tech/how-to/replace-public-gateways-with-self-hosted-ipfs/#run-someguy-your-routing-endpoint) host (Someguy, or Kubo's gateway `/routing/v1`). Confirm in the network panel that nothing still hits `ipfs.io`, `dweb.link`, `delegated-ipfs.dev`, or `trustless-gateway.link`.

If a CID inspector or Service Worker says **“None of these providers can be reached from a web browser”**, that is expected. Originless is Kubo: it announces the pin over libp2p **TCP/QUIC** (swarm port `4001`). A web page cannot open raw TCP or UDP. Browser IPFS clients (Helia, inbrowser.link, Service Workers) only dial **Secure WebSockets, WebTransport, WebRTC, or an HTTPS trustless gateway**. They find this node as a provider, cannot Bitswap to it, and fall back to HTTP.

That fallback is Originless. Open the file on **this node’s HTTP gateway**, not via Helia’s default public peers:

```text
http://localhost:3232/ipfs/{CID}
http://localhost:3232/ipfs/{CID}?filename=hello.txt
```

`ipfs.io` / `dweb.link` top-level clicks often redirect into [inbrowser.link](https://inbrowser.link), which is a Helia Service Worker — that page will print the provider warning even when the CID is pinned here. Use **This node** in the dashboard gateway picker, or `createVerifiedFetch({ gateways: [window.location.origin] })` so the HTTP path is Originless.

Originless does not currently expose `/wss` or WebRTC-Direct. Native IPFS peers (other Kubo nodes, IPFS Desktop) still retrieve over `4001`. Browsers use HTTP `/ipfs`.

### 🌐 Temporary public fallback

While you migrate, public path URLs still resolve the same CID:

```text
https://ipfs.io/ipfs/QmX...?filename=document.pdf
https://dweb.link/ipfs/QmX...?filename=document.pdf
```

Treat these as borrowed time, not the default share link. Originless already prepends **This node** in the dashboard gateway picker.

---

## 📡 HTTP API

**Full reference: [api.md](api.md)** — methods, query params, JSON shapes, and error codes for every route.

Base URL: `http://localhost:3232` · no API keys · CORS `*`.

| Method | Path | What it does |
| :----- | :--- | :----------- |
| `POST` | `/upload` | Pin a file as-is |
| `POST` | `/media` | Strip EXIF/GPS/XMP, then pin (JPEG/PNG/GIF/WebP) |
| `POST` | `/uploadfolder` | Pin a directory as one root CID |
| `GET` | `/ipfs/{cid}` | Fetch pinned bytes from this node |
| `GET` | `/health` | Liveness + peer count |
| `GET` | `/status` | Node, storage, gateway, archive |
| `GET` | `/pins` | Pin count and janitor threshold |
| `GET` | `/history` | Paginated upload log |
| `GET` | `/archive` | List Nostr-archived CIDs (fetch bytes via `/ipfs/{cid}`) |
| `GET` | `/metrics` | Prometheus text metrics |
| `GET` | `/api/examples` | JSON catalog of client tools |

```bash
curl -X POST -F "file=@photo.png" http://localhost:3232/upload
curl -O "http://localhost:3232/ipfs/$CID"
curl http://localhost:3232/health
```

### Nostr IPFS Archive

When `NOSTR_NPUBS` is set, Originless walks those accounts every 15 minutes (first scan at startup), extracts IPFS CIDs, and copies the bytes to `/archive`. List them with `GET /archive`; serve them with `GET /ipfs/{cid}`. See [api.md](api.md#get-archive).

**Sources scanned**

| Source                    | Example                                                                                                        |
| :------------------------ | :------------------------------------------------------------------------------------------------------------- |
| `ipfs://` in note content | `ipfs://bafybei...`                                                                                            |
| Public gateway URLs       | `https://ipfs.io/ipfs/bafybei...`, `https://dweb.link/ipfs/...`, Pinata, Cloudflare, `<cid>.ipfs.*` subdomains |
| NIP-94 kind `1063`        | `["url", "ipfs://..."]`, `["m", "image/png"]`, `["x", "<sha256>"]`                                             |
| NIP-92 `imeta` tags       | `["imeta", "url https://.../ipfs/<cid>", "m image/jpeg", "x <sha256>"]`                                        |

**Kinds:** `1` notes, `6` reposts, `20` pictures, `1063` files, `30023`/`30024` long-form, `9802` highlights.

Kubo `cat`/`get` is preferred (CID-verified). If the swarm miss, HTTP gateways are tried and hashed against the NIP-94 `x` tag when present. Failed CIDs are retried up to 5 times.

After each save, Originless pins the CID via the Kubo HTTP API (and re-adds from `/archive` if GC already dropped the blocks). Every 6 hours it walks the archive table and pins again so DHT provider records stay fresh. The janitor never unpins these CIDs: they are excluded in SQLite (`cid NOT IN archive`), not by changing Kubo config.

---

## ⚙️ Environment Configuration

| Variable              | Default       | Description                                                                                                                                                                                                                                       |
| :-------------------- | :------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `STORAGE_MAX`         | `100GB`       | Maximum storage limit allocated to IPFS data.                                                                                                                                                                                                     |
| `PIN_EXPIRY_DAYS`     | `30`          | Days a file stays pinned before the janitor may evict it.                                                                                                                                                                                         |
| `NOSTR_NPUBS`         | `""`          | Comma-separated list or JSON array of Nostr `npub` public keys. Enables the IPFS media archiver.                                                                                                                                                  |
| `NOSTR_RELAYS`        | famous relays | WebSocket relay URLs (comma-separated or JSON). Defaults: `wss://relay.damus.io`, `wss://nos.lol`, `wss://relay.nostr.band`, `wss://relay.primal.net`, `wss://nostr.mom`, `wss://purplerelay.com`, `wss://offchain.pub`, `wss://eden.nostr.land`. |
| `ENABLE_GATEWAY`      | `true`        | Serve content at `/ipfs/<cid>` and `/ipns/<name>` on port `3232`, and bind Kubo’s gateway on `8080`. Set `false` / `0` / `off` to disable HTTP content serving.                                                                              |
| `GATEWAY_NO_FETCH`    | *(off)*       | Optional. Set `true` to serve only local pins/uploads/archive (no swarm fetch). Swarm retrieve is the built-in default.                                                                                                                    |
| `IPFS_PROFILE`        | `lowpower`    | Kubo init profile for new `/data` repos. Default is Umbrel/home-friendly (`lowpower`). Also accepts `default`, `server`, etc. Does not rewrite an existing repo.                                                                           |
| `IPFS_ROUTING`        | `dhtclient`   | Kubo routing mode: `dhtclient`, `dht`, `dhtserver`, `auto`, or `none`.                                                                                                                                                                      |
| `IPFS_GATEWAY`        | `http://127.0.0.1:8080` | Backend URL of the Kubo HTTP gateway that Originless reverse-proxies.                                                                                                                                                                       |
| `SWARM_ANNOUNCE`      | `""`          | Comma-separated public multiaddrs announced to the DHT (use when Docker publishes `4001` behind NAT), e.g. `/ip4/203.0.113.10/tcp/4001,/ip4/203.0.113.10/udp/4001/quic-v1`.                                                                   |
| `IPFS_PEER_NODES`     | `""`          | Comma-separated Originless base URLs. On boot, read each `/status` peer id and `ipfs swarm connect` over DNS/TCP/QUIC `4001` (Compose-friendly).                                                                                            |
| `IPFS_SWARM_CONNECT`  | `""`          | Comma-separated multiaddrs to dial once at startup.                                                                                                                                                                                         |
| `IPFS_PEERING`        | `""`          | Raw Kubo `Peering.Peers` JSON array for sticky peers.                                                                                                                                                                                       |

**Volumes**

| Path       | Role                                                                                |
| :--------- | :---------------------------------------------------------------------------------- |
| `/data`    | Kubo IPFS repository + SQLite pin/upload log. **Persist this** or pins vanish when the container is recreated. |
| `/archive` | Permanent copies of IPFS media discovered on configured Nostr accounts. Never GC'd. |

**Ports:** `3232` (Originless), `8080` (Kubo gateway), `4001/tcp` + `4001/udp` (libp2p swarm).

---

## 📄 License

Distributed under the **ISC License**. Free for personal and commercial use.
