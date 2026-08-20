<div align="center">

# 🌐 Originless

**Private, decentralized, origin-independent file & app hosting powered by IPFS**

[![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)](https://ghcr.io/besoeasy/originless)
[![IPFS](https://img.shields.io/badge/IPFS-65C2CB?style=for-the-badge&logo=ipfs&logoColor=white)](https://ipfs.tech)
[![License: ISC](https://img.shields.io/badge/License-ISC-blue.svg?style=for-the-badge)](https://opensource.org/licenses/ISC)

**The frictionless storage backend for the modern web** — Drop into DApps, Nostr clients, AI agents, screenshot tools, and pastebins. Durable, cryptographic file hosting with zero API keys or user accounts required.

</div>

---

## ⚡ Quick Start (Run in 5 Seconds)

Run a complete Originless node with IPFS integrated using a single command:

```bash
docker run -d \
  --name originless \
  --restart unless-stopped \
  -p 3232:3232 \
  -e STORAGE_MAX=100GB \
  -v originless-data:/data \
  -v originless-archive:/archive \
  ghcr.io/besoeasy/originless:latest
```

> **Podman User?** Simply replace `docker` with `podman` in the command above!

- Access **Originless Web UI & API** at **[http://localhost:3232](http://localhost:3232)**

To also archive IPFS media from Nostr accounts onto `/archive`, pass `NOSTR_NPUBS`:

```bash
docker run -d \
  --name originless \
  --restart unless-stopped \
  -p 3232:3232 \
  -e STORAGE_MAX=100GB \
  -e NOSTR_NPUBS=npub1...,npub1... \
  -v originless-data:/data \
  -v originless-archive:/archive \
  ghcr.io/besoeasy/originless:latest
```

## Testing

```bash
docker build -t originless:local . && docker run --name originless -p 3232:3232 originless:local
```

## ⚡ Quick Showcase — Upload in One Command

Uploading to Originless is a single `POST /upload` request — **no API keys, no accounts, no auth**:

```bash
curl -X POST -F "file=@my-image.png" http://localhost:3232/upload
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

The file is pinned to IPFS and instantly available anywhere on the swarm:

- **Native IPFS:** `ipfs://QmX...`
- **Public gateway:** `https://ipfs.io/ipfs/QmX...`
- **Alt gateway:** `https://dweb.link/ipfs/QmX...`

Your `cid` is the file's cryptographic hash — anyone can verify the bytes match the address with zero trust in this server. Upload an entire folder (static site, `dist/`, asset bundle) as one root CID with [`/uploadfolder`](#upload-folder--dapp). Or run the ready-made script at the repo root to upload the whole [`examples/`](examples/) folder in one go:

```bash
./upload-examples.sh
```

### Examples & Client Tools

Client examples and decoupled tools are located in the [`examples/`](examples/) folder and served directly by Originless at `http://localhost:3232/examples/` (accessible via the **Client Tools** button in the dashboard navigation). You can also open them directly in your browser or deploy them independently:

- **[Single File Uploader](examples/upload-file.html)**: Drag & drop images, video, audio, or binaries to upload and pin on IPFS with instant public gateway links, embed HTML codes, and SHA-256 verification.
- **[Folder & DApp Uploader](examples/upload-folder.html)**: Upload full static websites and React/Vite `dist/` directories to IPFS with intact relative paths under a single root CID.
- **[Share Snippets & Pastebin](examples/snippet.html)**: Upload and pin code snippets, logs, and text pastes to IPFS with syntax highlighting, SHA-256 hashes, and instant public gateway links.
- **[Kind 20 Picture Post Generator](examples/picture.html)** (Instagram-style photo dump): Batch upload photos to IPFS, preview an interactive carousel feed, generate NIP-68 Kind 20 JSON with NIP-92 `imeta` tags (URL, MIME, SHA-256, dimensions, blurhash, alt), and publish directly or via NoStrudel.
- **[Kind 1 Short Note & Image Post Generator](examples/post.html)**: Compose Nostr text notes with IPFS image attachments and NIP-92 `imeta` tags, preview live note rendering, and publish directly or via NoStrudel.

---

## 🔥 Key Features

- **🌐 Origin-Independent Storage**: Files are pinned to IPFS. Once propagated to peers, content stays online even if your origin node goes offline.
- **🛡️ Legal & Host Protection**: P2P multihash routing shields node operators—you participate in the decentralized IPFS swarm rather than acting as a direct HTTP web host responsible for serving content under a centralized domain origin.
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
| **Hosting Liability & Risk**      | ❌ Server acts as direct HTTP web host (`https://server.com/<hash>`), making operator directly liable. | ✅ **Decentralized Swarm Buffer**. Content is content-addressed (`ipfs://CID`) and distributed over global P2P nodes. |
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
4. **Nostr Archive** (optional): When `NOSTR_NPUBS` is set, Originless scans those accounts, downloads discovered IPFS objects through Kubo (gateway fallback + `x` sha256 when needed), and writes them to `/archive`. It pins each CID immediately and re-pins the whole archive every `ARCHIVE_REPIN_HOURS` (default 6). The janitor skips archive CIDs (SQLite skip list — no Kubo config changes).

---

## 📡 Fetching Files & Content Verification

### 🛡️ Recommended for Automated Fetching: [`@helia/verified-fetch`](https://github.com/ipfs/helia-verified-fetch)

We strongly recommend using [**`@helia/verified-fetch`**](https://github.com/ipfs/helia-verified-fetch) for programmatic and automated retrieval of files pinned via Originless.

Unlike traditional HTTP requests to central gateways, `helia-verified-fetch` provides **trustless, content-addressed verification**. It downloads raw blocks directly over IPFS / libp2p, verifies the cryptographic hash against the CID in real-time, and protects your applications from tampered or stale data.

#### Installation

```bash
npm install @helia/verified-fetch
```

#### Example Usage

```javascript
import { verifiedFetch } from "@helia/verified-fetch";

// Fetch and verify content directly by CID
const cid = "QmX...";
const response = await verifiedFetch(`ipfs://${cid}`);
const fileBlob = await response.blob();

// Or parse JSON automatically
// const data = await verifiedFetch(`ipfs://${cid}`).then(res => res.json());
```

### 🌐 Standard Public Gateways

For simple browser links or non-verified web views, standard IPFS gateway URLs can also be used:

```text
https://dweb.link/ipfs/QmX...?filename=document.pdf
```

---

## 🛠️ API Quick Reference

Base URL: `http://localhost:3232`

### Upload Single File

```bash
curl -X POST -F "file=@photo.png" http://localhost:3232/upload
```

_Response:_

```json
{
  "status": "success",
  "cid": "QmX...",
  "size": 1048576,
  "type": "image/png",
  "filename": "photo.png"
}
```

### Upload Folder / DApp

```bash
curl -X POST \
  -F "file=@dist/index.html;filename=index.html" \
  -F "file=@dist/style.css;filename=style.css" \
  http://localhost:3232/uploadfolder
```

### Check Storage & Pins

```bash
curl http://localhost:3232/pins
curl http://localhost:3232/history
curl http://localhost:3232/status
```

### Nostr IPFS Archive

When `NOSTR_NPUBS` is set, Originless walks those accounts every `ARCHIVE_INTERVAL` minutes (first scan at startup), extracts IPFS CIDs, and copies the bytes to `/archive`.

**Sources scanned**

| Source                    | Example                                                                                                        |
| :------------------------ | :------------------------------------------------------------------------------------------------------------- |
| `ipfs://` in note content | `ipfs://bafybei...`                                                                                            |
| Public gateway URLs       | `https://ipfs.io/ipfs/bafybei...`, `https://dweb.link/ipfs/...`, Pinata, Cloudflare, `<cid>.ipfs.*` subdomains |
| NIP-94 kind `1063`        | `["url", "ipfs://..."]`, `["m", "image/png"]`, `["x", "<sha256>"]`                                             |
| NIP-92 `imeta` tags       | `["imeta", "url https://.../ipfs/<cid>", "m image/jpeg", "x <sha256>"]`                                        |

**Kinds:** `1` notes, `6` reposts, `20` pictures, `1063` files, `30023`/`30024` long-form, `9802` highlights.

Kubo `cat`/`get` is preferred (CID-verified). If the swarm miss, HTTP gateways are tried and hashed against the NIP-94 `x` tag when present. Failed CIDs are retried up to 5 times.

After each save, Originless pins the CID via the Kubo HTTP API (and re-adds from `/archive` if GC already dropped the blocks). Every `ARCHIVE_REPIN_HOURS` it walks the archive table and pins again so DHT provider records stay fresh. The janitor never unpins these CIDs: they are excluded in SQLite (`cid NOT IN archive`), not by changing Kubo config.

```bash
# List archived objects
curl http://localhost:3232/archive

# Fetch a stored file (or directory tree)
curl -O http://localhost:3232/archive/<cid>
```

### Health Check

```bash
curl http://localhost:3232/health
```

_Response:_

```json
{
  "status": "healthy",
  "peers": 140
}
```

### Prometheus Metrics

Exposes Prometheus text-format metrics (request counts, uploads, pinned storage, IPFS health, Nostr archive size) for scraping by Prometheus, Grafana, or any metrics collector:

```bash
curl http://localhost:3232/metrics
```

_Sample output:_

```
originless_http_requests_total{path="/upload"} 42
originless_uploads_total 42
originless_upload_bytes_total 1048576
originless_pinned_count 12
originless_pinned_bytes 52428800
originless_ipfs_healthy 1
originless_ipfs_peers 140
originless_archive_count 8
originless_archive_bytes 2097152
```

---

## ⚙️ Environment Configuration

| Variable              | Default       | Description                                                                                                                                                                                                                                       |
| :-------------------- | :------------ | :------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `STORAGE_MAX`         | `100GB`       | Maximum storage limit allocated to IPFS data.                                                                                                                                                                                                     |
| `PIN_EXPIRY_DAYS`     | `30`          | Days a file stays pinned before the janitor may evict it.                                                                                                                                                                                         |
| `NOSTR_NPUBS`         | `""`          | Comma-separated list or JSON array of Nostr `npub` public keys. Enables the IPFS media archiver.                                                                                                                                                  |
| `NOSTR_RELAYS`        | famous relays | WebSocket relay URLs (comma-separated or JSON). Defaults: `wss://relay.damus.io`, `wss://nos.lol`, `wss://relay.nostr.band`, `wss://relay.primal.net`, `wss://nostr.mom`, `wss://purplerelay.com`, `wss://offchain.pub`, `wss://eden.nostr.land`. |
| `ARCHIVE_DIR`         | `/archive`    | Directory (Docker volume) for permanent Nostr IPFS media. Never garbage-collected.                                                                                                                                                                |
| `ARCHIVE_INTERVAL`    | `15`          | Minutes between Nostr archive scans. First scan runs at startup.                                                                                                                                                                                  |
| `ARCHIVE_REPIN_HOURS` | `6`           | Hours between re-pinning every archived CID into Kubo (DHT provide + restore blocks from `/archive` if needed).                                                                                                                                   |

**Volumes**

| Path       | Role                                                                                |
| :--------- | :---------------------------------------------------------------------------------- |
| `/data`    | SQLite database and Kubo IPFS repository. Janitor may unpin content here.           |
| `/archive` | Permanent copies of IPFS media discovered on configured Nostr accounts. Never GC'd. |

---

## 📄 License

Distributed under the **ISC License**. Free for personal and commercial use.
