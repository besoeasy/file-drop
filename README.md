# File Drop

> **Share files anonymously. No sign-ups. No tracking. No centralized servers.**

Ever wanted to send or embed a file without trusting Big Tech? File Drop lets you share images, videos, and any file type through IPFS—a decentralized, peer-to-peer network. Your data stays on your machine and propagates across the network, making it censorship-resistant and always available.

![File Drop](https://github.com/user-attachments/assets/8d427693-8ee4-4c5f-a67c-6c2991c13f27)

## ✨ Features

- 🔒 **Anonymous** – No accounts, no tracking
- 🌐 **Decentralized** – Powered by IPFS, no central servers
- 📦 **Any file type** – Images, videos, documents, anything
- 🪶 **Lightweight** – Minimal resource footprint
- 🔄 **Resilient** – Files persist across the IPFS network

![File Drop](https://github.com/user-attachments/assets/ff683fd8-d7c0-4378-81d4-a6342890cb86)
![File Drop](https://github.com/user-attachments/assets/0d7c6291-0194-470c-a07c-ef748b39337f)

---

## 🚀 Quick Install

### Umbrel OS (Recommended)

One-click install on Umbrel:

[![Install on Umbrel](https://img.shields.io/badge/Umbrel-Install%20Now-5351FB?style=for-the-badge&logo=umbrel&logoColor=white)](https://apps.umbrel.com/app/file-drop)

### Docker

```bash
docker run -d --restart unless-stopped \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  --name file-drop \
  -e STORAGE_MAX=50GB \
  ghcr.io/besoeasy/file-drop:main
```

### Docker Compose

```yaml
services:
  file-drop:
    image: ghcr.io/besoeasy/file-drop:main
    container_name: file-drop
    restart: unless-stopped
    ports:
      - "3232:3232"
      - "4001:4001/tcp"
      - "4001:4001/udp"
    environment:
      - STORAGE_MAX=50GB
```

---

## 📖 Usage

Open `http://localhost:3232` in your browser.

### Upload via curl

```bash
curl -X POST -F "file=@/path/to/file.jpg" http://localhost:3232/upload
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/upload` | POST | Upload a file, returns IPFS CID |
| `/status` | GET | IPFS node stats (peers, bandwidth, storage) |
| `/health` | GET | Health check for Docker |

---

## ⚙️ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_MAX` | `200GB` | Max IPFS storage (e.g., `50GB`, `1TB`) |

---

## 📝 Note

File Drop is designed for **temporary sharing**, not permanent storage. Files are cached across IPFS peers but may eventually be garbage-collected. Perfect for sharing on Nostr, forums, or any app that supports IPFS links.

---

## 📄 License

Open source. Self-host it. Own your data.
