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
- 🌸 **Blossom-compatible** – Works with Amethyst and other Blossom clients

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
curl -X PUT -F "file=@/path/to/file.jpg" http://localhost:3232/upload
```

**Response (Blossom-compatible format):**
```json
{
  "status": "success",
  "url": "https://dweb.link/ipfs/QmXXXXXX...",
  "sha256": "b1674191a88ec5cdd733e4240a81803105dc412d6c6708d53ab94fc248f4f553",
  "size": 184292,
  "type": "image/jpeg",
  "uploaded": 1725105921,
  "cid": "QmXXXXXX...",
  "filename": "file.jpg"
}
```

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/upload` | POST | Upload a file, returns Blossom-compatible response with IPFS URL |
| `/status` | GET | IPFS node stats (peers, bandwidth, storage) |
| `/health` | GET | Health check for Docker |

---

## ⚙️ Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_MAX` | `200GB` | Max IPFS storage (e.g., `50GB`, `1TB`) |

---

## 🌸 Blossom Compatibility

File Drop now implements the [Blossom API specification](https://github.com/hzrd149/blossom), making it compatible with clients like [Amethyst](https://github.com/vitorpamplona/amethyst). Simply add your File Drop server URL to your Blossom server list in Amethyst, and it will work seamlessly for uploading media.

The `/upload` endpoint returns responses in the Blossom format with SHA256 hash verification, making it interoperable with the wider Blossom ecosystem.

---

## 📝 Note

File Drop is designed for **temporary sharing**, not permanent storage. Files are cached across IPFS peers but may eventually be garbage-collected. Perfect for sharing on Nostr, forums, or any app that supports IPFS links.

---

## 📄 License

Open source. Self-host it. Own your data.
