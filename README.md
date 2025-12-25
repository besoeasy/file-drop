# File Drop

> **Anonymous file sharing via IPFS. No accounts. No tracking. Fully decentralized.**

![File Drop](https://github.com/user-attachments/assets/8d427693-8ee4-4c5f-a67c-6c2991c13f27)

## Features

- **Anonymous** – No sign-ups, no tracking
- **Decentralized** – Powered by IPFS peer-to-peer network
- **Any file type** – Images, videos, documents, anything
- **Resilient** – Files propagate across IPFS; accessible even when your server is offline
- **Self-cleaning** – Automatic garbage collection keeps storage sustainable

---

## Quick Start

### Umbrel (One-Click)

[![Install on Umbrel](https://img.shields.io/badge/Umbrel-Install%20Now-5351FB?style=for-the-badge&logo=umbrel&logoColor=white)](https://apps.umbrel.com/app/file-drop)

### Docker

```bash
docker run -d --restart unless-stopped \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  -v file-drop-data:/data \
  --name file-drop \
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
    volumes:
      - file-drop-data:/data
    environment:
      - STORAGE_MAX=50GB
      - FILE_LIMIT=5GB

volumes:
  file-drop-data:
```

Open **http://localhost:3232** after starting.

---

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_MAX` | `200GB` | Maximum IPFS storage before garbage collection |
| `FILE_LIMIT` | `5GB` | Maximum size per file upload |

**Volume Mount:** The `/data` volume persists your IPFS repository. Without it, all files are lost on container restart.

---

## API Quick Reference

### Upload a File

```bash
curl -X PUT -F "file=@photo.jpg" http://localhost:3232/upload
```

**Returns IPFS CID and gateway URL.**

### Check Server Health

```bash
curl http://localhost:3232/health
```

**📚 Complete API Documentation:** See [API.md](API.md) for full specification, error handling, chunked uploads, and client implementation examples.

---

## How It Works

1. **Upload** – Files are added to your local IPFS node (unpinned)
2. **Propagate** – Content spreads across the IPFS network as peers request it
3. **Access** – Files accessible via any IPFS gateway, even if your server goes offline
4. **Cleanup** – When storage fills, oldest/least-accessed files are garbage collected

> **Note:** Files are temporary by design. Popular files persist longer; old unused files are cleaned up automatically.

---

## For Developers

**📚 Full API Documentation:** [API.md](API.md)

File Drop provides a simple HTTP API for file uploads to IPFS:
- **Single uploads** for small files (< 5MB)
- **Chunked uploads** for large files with automatic retry
- Client libraries: JavaScript, Python, Go, Dart/Flutter
- No authentication required

**Use cases:**
- Chat apps (file attachments)
- Social platforms (media uploads)
- Forums (embedded content)
- CDN replacement for static assets

---

## Built With File Drop

**[0xchat](https://0xchat.com/)** – Secure Nostr chat app using File Drop for file sharing in conversations.

---