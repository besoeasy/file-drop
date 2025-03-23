# File Drop

*A decentralized, open-source solution for sharing images and videos using P2P technology.*

## Overview

File Drop is a lightweight, peer-to-peer (P2P) application that allows users to share images and videos directly between devices without relying on centralized servers. Built with privacy and simplicity in mind, it leverages Docker for easy deployment and P2P protocols for efficient, secure file transfers.


## Features

- **P2P Sharing**: Share files directly between peers, no middleman required.
- **Image & Video Support**: Optimized for common media formats (e.g., JPG, PNG, MP4).
- **Dockerized**: Quick setup with a single Docker command.
- **Open Source**: Free to use, modify, and contribute to.

File Drop is not built for permanent storage. Think of it as a way to share files temporarily on networks like Nostr, forums, and other apps. Since IPFS itself can be memory-intensive, we’ve designed File Drop to be lightweight. If you need permanent storage, you can edit 2-3 lines in the code and set pinning to true, but that’s not our aim.

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

1. Launch the container using the command above.
2. Access the application via `http://localhost:3232` in your browser (or your machine’s IP if remote).
