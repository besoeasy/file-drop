 
# File Drop

*A decentralized, open-source solution for sharing images, videos, and any other files.*

File Drop is a lightweight, peer-to-peer (P2P) application that allows users to share files directly between devices without relying on centralized servers. Built with privacy and simplicity in mind, it leverages Docker for easy deployment and P2P protocols for efficient, secure file transfers. With File Drop, you can upload anything without the fear of being censored since your data is relayed through your own computer. Even if your node loses its internet connection, your files persist across the entire IPFS network, ensuring availability.

![File Drop](https://github.com/user-attachments/assets/8d427693-8ee4-4c5f-a67c-6c2991c13f27)

File Drop is not built for permanent storage. Think of it as a way to share files temporarily on networks like Nostr, forums, and other apps. Since IPFS itself can be memory-intensive, we’ve designed File Drop to be lightweight. If you need permanent storage, you can edit 2-3 lines in the code and set pinning to `true`, but that’s not our aim. File Drop supports any file type—be it images, videos, text files, or anything else—with a current limit of 250 MB per file, as the IPFS network isn’t mature enough to handle larger files reliably. However, with a powerful enough computer, there are practically no limits.

![File Drop](https://github.com/user-attachments/assets/ff683fd8-d7c0-4378-81d4-a6342890cb86)
![File Drop](https://github.com/user-attachments/assets/0d7c6291-0194-470c-a07c-ef748b39337f)


## Installation

Run the following command to start File Drop:
```bash
docker run -d --restart unless-stopped -p 3232:3232 --name file-drop ghcr.io/besoeasy/file-drop:main
```
- `-d`: Runs the container in detached mode.
- `--restart unless-stopped`: Automatically restarts the container unless manually stopped.
- `-p 3232:3232`: Maps port 3232 on your host to the container.
- `--name file-drop`: Names the container "file-drop".
- `ghcr.io/besoeasy/file-drop:main`: Pulls the latest image from GitHub Container Registry.

## Usage

Access the application via `http://localhost:3232` in your browser (or your machine’s IP if remote).
 
