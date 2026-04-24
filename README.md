<div align="center">

# 🌐 Originless

**Private, decentralized file sharing for Nostr and the web**

[![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/besoeasy/Originless/pkgs/container/originless)
[![IPFS](https://img.shields.io/badge/IPFS-65C2CB?style=for-the-badge&logo=ipfs&logoColor=white)](https://ipfs.tech)
[![License: ISC](https://img.shields.io/badge/License-ISC-blue.svg?style=for-the-badge)](https://opensource.org/licenses/ISC)

**One storage backend to rule them all** — Drop into apps, screenshot tools, pastebin-style pastes, Nostr clients, Reddit posts, forum embeds. Durable, anonymous file hosting that keeps you private.

[🚀 Quick Start](#-quick-start) • [🎯 Features](#-features) • [️ Deploy Frontend](#-deploy-vuejs--react-projects) • [🛠️ API Reference](#-api-reference) • [🤖 AI Agent Integration](#-ai-agent-integration) • [🌍 Public Gateway](https://originless.besoeasy.com)

<img width="1536" src="https://github.com/user-attachments/assets/5014810c-cc51-4ad4-a1b8-6e4db510c09f" alt="Originless Banner" />

</div>

---

## 🚀 Quick Start

### Self-Hosted (Recommended)

```bash
docker run -d --restart unless-stopped --name originless \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  -v originlessd:/data \
  -e STORAGE_MAX=200GB \
  ghcr.io/besoeasy/originless
```

**Access:** Open http://localhost:3232

### Public Gateways

Don't want to self-host? Use one of the community-run public gateways:

| Gateway  | URL                              |
| -------- | -------------------------------- |
| besoeasy | https://originless.besoeasy.com/ |
| gupt.app | https://originless.gupt.app/     |
| 0xchat   | https://originless.0xchat.com/   |

Simply replace `http://localhost:3232` with any public gateway URL in API calls.

---

## 🎯 Features

<table>
<tr>
<td width="33%" valign="top">

### 🕶️ Anonymous

No accounts, no tracking, no logs. Upload files completely anonymously without leaving a trace.

</td>
<td width="33%" valign="top">

### 🌍 Decentralized

Built on IPFS. Content persists across the network even if your node goes offline.

</td>
<td width="33%" valign="top">

### 🔄 Self-Healing

Content automatically repopulates when your node comes back online. Set it and forget it.

</td>
</tr>
<tr>
<td width="33%" valign="top">

### 🔐 Privacy-First

Optional client-side encryption for sensitive content. Even the server operator can't read your data.

</td>
<td width="33%" valign="top">

### 🧰 Minimal Surface

Simple upload and hosting workflows without extra auth or admin layers.

</td>
<td width="33%" valign="top">

### 🚀 Easy Integration

Simple REST API. Drop it into any app, tool, or platform in minutes.

</td>
</tr>
</table>

---

## �📸 Screenshots

<div align="center">
<img width="900" src="https://github.com/user-attachments/assets/6ed4908c-37aa-4973-a9c0-edb7c0fe479f" alt="Originless Web Interface" />
</div>

---

## 🤝 Integrations

Originless is already powering file storage for these platforms:

<table>
<tr>
<td align="center" width="33%">

### 💬 0xchat

Private, decentralized Nostr chat

[Visit 0xchat.com →](https://0xchat.com/)

</td>
<td align="center" width="33%">

### 📝 ZeroNote

Anonymous encrypted notes sharing

[Visit zeronote.js.org →](https://zeronote.js.org/)

</td>
<td align="center" width="33%">

### 🌐 gupt.app

Private, anonymous file sharing

[Visit gupt.app →](https://gupt.app/)

</td>
</tr>
</table>

---

## 🔄 How It Works

```mermaid
graph LR
    A[📤 Upload File] --> B[🏠 Local IPFS Node]
    B --> C[🌐 IPFS Network]
    C --> D[👥 Peers Request]
    D --> E[♻️ Content Spreads]

    style A fill:#2563eb,color:#fff
    style B fill:#10b981,color:#fff
    style C fill:#8b5cf6,color:#fff
```

1. **📤 Upload** — Files stream to your local IPFS node (unpinned by default)
2. **🌐 Propagate** — Content spreads via IPFS as peers request it
3. **♻️ Self-Heal** — If garbage collected, your node repopulates content when online

---

## ⚙️ Configuration

### Environment Variables

| Variable      | Default | Description                    |
| ------------- | ------- | ------------------------------ |
| `STORAGE_MAX` | `200GB` | Maximum storage limit for IPFS |
| `PORT`        | `3232`  | API server port                |

### Advanced Setup

**Custom storage limit:**

```bash
docker run -d ... -e STORAGE_MAX=500GB ghcr.io/besoeasy/originless
```

## 🛠️ API Reference

Base URL (local): `http://localhost:3232`

### Overview

- Responses are JSON unless otherwise noted.

### POST /upload

Upload a file directly from your local system.

**Request**

```bash
curl -X POST -F "file=@yourfile.pdf" http://localhost:3232/upload
```

**Response**

```json
{
  "status": "success",
  "cid": "QmX...",
  "url": "https://dweb.link/ipfs/QmX...?filename=yourfile.pdf",
  "size": 12345,
  "type": "application/pdf",
  "filename": "yourfile.pdf"
}
```

### POST /uploadzip

Upload a `.zip` archive. Originless extracts it and stores the entire folder to IPFS as a directory. This is the endpoint to use for static site deploys.

**Request**

```bash
curl -X POST -F "file=@dist.zip" http://localhost:3232/uploadzip
```

**Response**

```json
{
  "status": "success",
  "cid": "QmX...",
  "url": "https://dweb.link/ipfs/QmX.../",
  "filename": "dist.zip",
  "fileCount": 12
}
```

Open the returned `url` in a browser to view the hosted folder. For single-page apps, append `/index.html` if needed.

---

## 🤖 AI Agent Integration

Originless is an excellent drop-in tool for your AI agents. You can teach your AI agents to use Originless for uploading files, HTML reports, images, and PDFs seamlessly.

By giving your agents access to the Originless API, they can instantly share artifacts, diagnostic reports, and generative content via IPFS—**no hosting, user accounts, or API keys required**. Just point them to your running instance to automatically give them permanent, decentralized file sharing capabilities.

### Example Prompts for your AI

Try giving these tasks to an agent or coding assistant (like Cursor or GitHub Copilot) that has terminal access:

- _"What's the current Bitcoin price and what recent news do we have on Bitcoin? Create a beautiful `index.html` report with this data and upload it to Originless (https://originless.besoeasy.com/upload) so I can share it."_
- _"Write a python script that generates a complex 3D fractal image, save it as a PNG, and upload it anonymously to my local Originless node (http://localhost:3232/upload)."_
- _"Build a small React app for a Pomodoro timer, build it to the `dist` folder, zip the output, and publish it as a live website to IPFS using `https://originless.besoeasy.com/uploadzip`."_

---

## 🌟 Use Cases

- **🌍 Decentralized Apps** — Build your frontend and upload the `dist` folder to host your live DApp
- **🖼️ Screenshot Tools** — Anonymous image hosting for screenshots
- **📝 Pastebin Alternative** — Decentralized paste sharing
- **💬 Nostr Clients** — Media attachments for decentralized social
- **🎨 Portfolio Hosting** — Permanent galleries and portfolios
- **📦 Package Distribution** — Resilient software distribution
- **🎵 Podcast Hosting** — Decentralized RSS feed media
- **💾 Backup Storage** — Self-healing backup infrastructure
- **🔗 Link Preservation** — Combat link rot with IPFS archiving
