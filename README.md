<div align="center">

# 🌐 Originless

**Private, decentralized file sharing for Nostr and the web**

[![Docker](https://img.shields.io/badge/docker-%230db7ed.svg?style=for-the-badge&logo=docker&logoColor=white)](https://github.com/besoeasy/Originless/pkgs/container/originless)
[![IPFS](https://img.shields.io/badge/IPFS-65C2CB?style=for-the-badge&logo=ipfs&logoColor=white)](https://ipfs.tech)
[![License: ISC](https://img.shields.io/badge/License-ISC-blue.svg?style=for-the-badge)](https://opensource.org/licenses/ISC)

**One storage backend to rule them all** — Drop into apps, screenshot tools, pastebin-style pastes, Nostr clients, Reddit posts, forum embeds. Durable, anonymous file hosting that keeps you private.

<img width="1536" src="https://github.com/user-attachments/assets/5014810c-cc51-4ad4-a1b8-6e4db510c09f" alt="Originless Banner" />

</div>

---

## 🚀 Quick Start

```bash
docker run -d --restart unless-stopped --name originless \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  -v originlessd:/data \
  -e STORAGE_MAX=200GB \
  ghcr.io/besoeasy/originless
```

Open http://localhost:3232

### Public Gateways

| Gateway  | URL                              |
| -------- | -------------------------------- |
| besoeasy | https://originless.besoeasy.com/ |
| gupt.app | https://originless.gupt.app/     |
| 0xchat   | https://originless.0xchat.com/   |

---

## 🌟 Use Cases

- **🌍 Decentralized Apps** — Build your frontend and upload the `dist` folder to host your live DApp on IPFS
- **🖼️ Screenshot Tools** — Anonymous image hosting for screenshots and screen recordings
- **📝 Pastebin Alternative** — Decentralized paste and snippet sharing
- **💬 Nostr Clients** — Media attachments for decentralized social apps
- **🎨 Portfolio Hosting** — Permanent galleries and portfolios that survive link rot
- **📦 Package Distribution** — Resilient software and asset distribution
- **🎵 Podcast Hosting** — Decentralized RSS feed media hosting
- **💾 Backup Storage** — Self-healing backup infrastructure

---

## 🔄 How It Works

1. **Upload** — Files stream to your local IPFS node (unpinned by default)
2. **Propagate** — Content spreads via IPFS as peers request it
3. **Self-Heal** — If garbage collected, your node repopulates content when online

---

## 🤝 Integrations

| Platform | Description |
| -------- | ----------- |
| [0xchat](https://0xchat.com/) | Private, decentralized Nostr chat |
| [ZeroNote](https://zeronote.js.org/) | Anonymous encrypted notes sharing |
| [gupt.app](https://gupt.app/) | Private, anonymous file sharing |

---

## ⚙️ Configuration

| Variable      | Default | Description                    |
| ------------- | ------- | ------------------------------ |
| `STORAGE_MAX` | `200GB` | Maximum storage limit for IPFS |
| `PORT`        | `3232`  | API server port                |

---

## 🛠️ API Reference

Base URL: `http://localhost:3232`

### POST /upload

Upload a single file.

```bash
curl -X POST -F "file=@yourfile.pdf" http://localhost:3232/upload
```

```json
{
  "status": "success",
  "cid": "QmX...",
  "size": 12345,
  "type": "application/pdf",
  "filename": "yourfile.pdf"
}
```

---

## 🤖 AI Agent Integration

Teach your agents (Cursor, GitHub Copilot, Claude, etc.) to use Originless — **no API keys, no accounts, no configuration required**. Just point them at a running instance.

### Example Prompts

- _"What's the current Bitcoin price? Create a beautiful `index.html` report and upload it to `https://originless.besoeasy.com/upload` so I can share it."_
- _"Generate a complex 3D fractal image, save it as a PNG, and upload it to my local Originless node at `http://localhost:3232/upload`."_
- _"Build a React app, and publish the built output to IPFS via `https://originless.besoeasy.com/upload`."_
