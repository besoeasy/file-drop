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
curl -X PUT -F "file=@file.jpg" http://localhost:3232/upload
```

## ⚙️ Configuration

| Variable      | Default | Description                            |
| ------------- | ------- | -------------------------------------- |
| `STORAGE_MAX` | `200GB` | Max IPFS storage (e.g., `50GB`, `1TB`) |

---

## 🌟 Apps Using FILEdrop

### [0xchat](https://0xchat.com/)

0xchat is a secure chat app built on the Nostr protocol. It prioritizes security, featuring private key login, encrypted private chats and contacts, encrypted group chats, and lightning payments. Additionally, it also offers an open communication platform through public channels.

**Use Case:** Uses FILEdrop as a way to send files in chat conversations.

---

## �‍💻 For Developers
### Using FILEdrop as a Backend

FILEdrop can be integrated as a backend service in your applications. Since it accepts all file types without restrictions, it's perfect for:
- Chat applications needing file sharing
- Social media platforms
- Content management systems
- Any app requiring decentralized file storage

Simply point your app's file upload functionality to your FILEdrop instance endpoint.
### Health Endpoint

FILEdrop includes a health endpoint that reports the number of connected IPFS peers:

```bash
curl http://localhost:3232/health
```

Response:
```json
{
  "status": "healthy",
  "peers": 42
}
```

### Multiple Server Setup

If you're running multiple FILEdrop servers, you should **always prefer the server with the highest peer count**. A peer count above **10 is golden and good to go** for reliable file propagation.

#### Pseudo Logic for Server Selection

```javascript
// For each filedrop server
const servers = [
  'http://filedrop1.example.com',
  'http://filedrop2.example.com',
  'http://filedrop3.example.com'
];

async function selectBestServer(servers) {
  let bestServer = null;
  let highestPeers = 0;

  for (const server of servers) {
    try {
      const response = await fetch(`${server}/health`);
      const health = await response.json();
      
      // Prefer servers with peers > 10 (golden threshold)
      if (health.peers > 10 && health.peers > highestPeers) {
        bestServer = server;
        highestPeers = health.peers;
      }
    } catch (error) {
      console.log(`Server ${server} unavailable`);
    }
  }

  // Fallback: use server with highest peers even if < 10
  if (!bestServer && highestPeers > 0) {
    bestServer = servers[0]; // or implement fallback logic
  }

  return bestServer;
}

// Upload to the best available server
const bestServer = await selectBestServer(servers);
if (bestServer) {
  await uploadFile(bestServer, yourFile);
}
```

---

## �📝 Note

File Drop is designed for **temporary sharing**, not permanent storage. Files are cached across IPFS peers but may eventually be garbage-collected. Perfect for sharing on Nostr, forums, or any app that supports IPFS links.


