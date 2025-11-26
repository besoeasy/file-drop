# File Drop

_A open-source solution for sharing images, videos, and any other files._

File Drop is a lightweight, peer-to-peer (P2P) application that allows users to share files directly between devices without relying on centralized servers. Built with privacy and simplicity in mind, it leverages Docker for easy deployment and P2P protocols for efficient, secure file transfers. With File Drop, you can upload anything without the fear of being censored since your data is relayed through your own computer. Even if your node loses its internet connection, your files persist across the entire IPFS network, ensuring availability.

![File Drop](https://github.com/user-attachments/assets/8d427693-8ee4-4c5f-a67c-6c2991c13f27)

File Drop is not built for permanent storage. Think of it as a way to share files temporarily on networks like Nostr, forums, and other apps. Since IPFS itself can be memory-intensive, we’ve designed File Drop to be lightweight. If you need permanent storage, you can edit 2-3 lines in the code and set pinning to `true`, but that’s not our aim. File Drop supports any file type—be it images, videos, text files, or anything else—with a current limit of 250 MB per file, as the IPFS network isn’t mature enough to handle larger files reliably. However, with a powerful enough computer, there are practically no limits.

![File Drop](https://github.com/user-attachments/assets/ff683fd8-d7c0-4378-81d4-a6342890cb86)
![File Drop](https://github.com/user-attachments/assets/0d7c6291-0194-470c-a07c-ef748b39337f)

## Installation

Run the following command to start File Drop:

```bash
docker run -d --restart unless-stopped \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  --name file-drop \
  -e STORAGE_MAX=50GB \
  ghcr.io/besoeasy/file-drop:main
```

## Portainer Stack

For easy deployment with Portainer, use this stack configuration:

```yaml
version: "3.8"

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

**Steps to deploy with Portainer:**

1. Open Portainer and navigate to **Stacks**
2. Click **Add stack** and give it a name (e.g., "file-drop")
3. Paste the above YAML configuration in the editor
4. Adjust environment variables as needed
5. Click **Deploy the stack**

## Configuration

### Environment Variables

- **STORAGE_MAX**: Sets the maximum storage limit for the IPFS repository.
  - Default: `200GB`
  - Example: `-e STORAGE_MAX=50GB` limits IPFS storage to 50 GB
  - Formats: Supports standard units like `MB`, `GB`, `TB`
  - Note: This controls how much disk space IPFS can use for storing files and metadata

## Usage

Access the application via `http://localhost:3232` in your browser (or your machine’s IP if remote).
