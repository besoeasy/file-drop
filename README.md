# File Drop

> **Share files anonymously. No sign-ups. No tracking. No centralized servers.**

Ever wanted to send or embed a file without trusting Big Tech? File Drop lets you share images, videos, and any file type through IPFS—a decentralized, peer-to-peer network. Your data stays on your machine and propagates across the network, making it censorship-resistant and always available.

![File Drop](https://github.com/user-attachments/assets/8d427693-8ee4-4c5f-a67c-6c2991c13f27)

## ✨ Features

- 🔒 **Anonymous** – No accounts, no tracking
- 🌐 **Decentralized** – Powered by IPFS, no central servers
- 📦 **Any file type** – Images, videos, documents, anything
- 🪶 **Lightweight** – Minimal resource footprint
- 🔄 **Resilient** – Files persist across the IPFS network, accessible even if your server goes down
- 🛡️ **Zero downtime** – FILEdrop server downtime doesn't affect file access, perfect for newbies to experts

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
  -v file-drop-data:/data \
  -e STORAGE_MAX=50GB \
  -e FILE_LIMIT=5GB \
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

---

## 📖 Usage

Open `http://localhost:3232` in your browser.

### Upload via curl

```bash
curl -X PUT -F "file=@file.jpg" http://localhost:3232/upload
```

## ⚙️ Configuration

### Environment Variables

| Variable      | Default | Description                            |
| ------------- | ------- | -------------------------------------- |
| `STORAGE_MAX` | `200GB` | Max IPFS storage (e.g., `50GB`, `1TB`) |
| `FILE_LIMIT`  | `5GB`   | Max file upload size (e.g., `50MB`, `10GB`) |

### Docker Volumes

The IPFS repository is stored at `/data` inside the container. **Mounting this as a volume is critical** to persist your IPFS data across container restarts.

**Named Volume (Recommended):**
```bash
-v file-drop-data:/data
```

**Bind Mount (Alternative):**
```bash
-v /path/on/host:/data
```

Without a volume mount, all uploaded files and IPFS configuration will be lost when the container is removed or recreated.

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

**What makes FILEdrop special:** Even if your server goes down, files are cached on the IPFS network and will work fine. This means FILEdrop downtime doesn't really affect users from accessing files, which makes it a better choice for newbies to experts all together.
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

## 📝 Note

**File Persistence:** With Docker volumes configured, your IPFS repository and uploaded files will persist across container restarts. However, files are stored **unpinned** by design and will be automatically garbage-collected when your `STORAGE_MAX` limit is reached. The oldest and least-accessed files are removed first to make room for new uploads.

**Use Case:** File Drop is ideal for **temporary file sharing** - shared links on Nostr, forums, chat apps, or any platform that supports IPFS/HTTP links. Files remain available as long as storage permits and are distributed across the IPFS network for resilience.
