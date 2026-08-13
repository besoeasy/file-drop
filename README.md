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
  ghcr.io/besoeasy/originless:latest
```

> **Podman User?** Simply replace `docker` with `podman` in the command above!

- Access **Originless Web UI & API** at **[http://localhost:3232](http://localhost:3232)**

---

## 🔥 Key Features

- **🌐 Origin-Independent Storage**: Files are pinned to IPFS. Once propagated to peers, content stays online even if your origin node goes offline.
- **🛡️ Legal & Host Protection**: P2P multihash routing shields node operators—you participate in the decentralized IPFS swarm rather than acting as a direct HTTP web host responsible for serving content under a centralized domain origin.
- **🏠 Zero Domain, HTTPS, or Port Exposure Required**: Runs seamlessly behind NATs, firewalls, home servers, or local environments using IPFS `libp2p` hole-punching. No domain name, public IP, or SSL certificate setup needed!
- **🔒 Zero-Friction & Accountless**: No API keys, passwords, or authentication overhead needed for uploads. Perfect for local dev, public APIs, or AI agent integration.
- **📁 Full Folder & DApp Uploads**: Upload complete static websites, React/Vite build directories (`dist/`), or asset folders with intact relative paths (unlike single-file media servers).
- **🛡️ Tamper-Proof Cryptographic Verification**: Content-addressed by unique IPFS CIDs (`ipfs://QmX...`). Downloads can be cryptographically verified against the hash.
- **🧹 Automated Smart Janitor**: Intelligent disk space management. Keeps uploads pinned for 7 days minimum and automatically evicts oldest files at 75% capacity to prevent disk overflow.
- **🤖 Built for Autonomous AI Agents**: Native endpoint design allows LLMs (Claude, Copilot, Cursor, Custom Agents) to host generated media, code snippets, or sites instantly.

---

## ⚡ Why Originless vs. Nostr Blossom & Centralized Media Servers

| Feature | Nostr Blossom / Centralized HTTP Servers | 🌐 **Originless** |
| :--- | :--- | :--- |
| **Domain & Network Setup** | ❌ **Requires** public domain, SSL certificates (HTTPS), and open public web ports. | ✅ **Zero Domain / NAT Ready**. Runs behind home routers, firewalls, or local Docker containers via `libp2p`. |
| **Hosting Liability & Risk** | ❌ Server acts as direct HTTP web host (`https://server.com/<hash>`), making operator directly liable. | ✅ **Decentralized Swarm Buffer**. Content is content-addressed (`ipfs://CID`) and distributed over global P2P nodes. |
| **Link Resilience** | ❌ Single point of failure. If server operator shuts down, all file links break instantly. | ✅ **Origin-Independent**. Content propagates across global IPFS swarm and stays alive even if your node shuts down. |
| **Multi-File & DApp Support** | ❌ Single flat files/blobs only (photos/videos). | ✅ **Full Directory & DApp Hosting**. Upload entire Vite/React `dist/` folders or static apps. |
| **Trustless Client Verification**| ❌ Relies on server trust. | ✅ **Content-Addressed Cryptography**. Verifiable via `@helia/verified-fetch` down to raw data blocks. |

---

## 🌟 Primary Use Cases

| Use Case | Description |
| :--- | :--- |
| **🚀 Decentralized DApp Hosting** | Drag-and-drop or upload your frontend `dist/` folder to host permanent, censorship-resistant web applications on IPFS. |
| **💬 Nostr Media Attachments** | Instant, zero-auth media uploader for decentralized social apps (e.g., [0xchat](https://0xchat.com/)). |
| **🤖 Autonomous AI Agent Outputs** | Give AI assistants the ability to publish reports, interactive charts, and generated images with zero setup or API keys. |
| **🖼️ Anonymous Image & Screenshot Sharing** | Instant backend for screenshot capture tools, screen recordings, and temporary image uploads. |
| **📝 Pastebin & Code Snippet Sharing** | Share text, log files, or code snippets without fear of link rot or centralized deletion. |
| **📦 Resilient Asset Distribution** | Distribute software releases, binaries, audio/podcasts, or heavy media over a self-healing P2P network. |

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
```

1. **Upload**: Send single files or directory trees via the Web UI or simple HTTP POST endpoint.
2. **Pin & Distribute**: Originless pins the content locally on IPFS and broadcasts it to global P2P peers.
3. **Automated Lifecycle**: Files are guaranteed pinned for 7 days. An automated janitor routine reconciles storage to keep usage below limits.

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
*Response:*
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
```

---

## ⚙️ Environment Configuration

| Variable | Default | Description |
| :--- | :--- | :--- |
| `STORAGE_MAX` | `100GB` | Maximum storage limit allocated to IPFS data. |
| `PIN_EXPIRY_DAYS` | `30` | Days a file stays pinned before the janitor may evict it. |

> All other paths are fixed: the SQLite database and IPFS node repository live at `/data` (the container's persistent volume).

---

## 📄 License

Distributed under the **ISC License**. Free for personal and commercial use.
