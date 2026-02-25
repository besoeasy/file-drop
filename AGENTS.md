# Originless — Agent Guide

Originless is a self-hosted, anonymous, decentralized file storage backend built on IPFS. As an agent you can upload files, host static sites, mirror remote assets, and manage persistent pins — all via a simple REST API and without accounts or API keys for public operations.

---

## Base URL

```
https://originless.besoeasy.com
```

---

## Use Cases

---

### 1. Share a file with a user

Upload any file (image, PDF, video, archive, etc.) to get a permanent IPFS link.

```bash
curl -X POST -F "file=@/path/to/report.pdf" https://originless.besoeasy.com/upload
```

**Response:**

```json
{
  "status": "success",
  "cid": "QmX...",
  "url": "https://dweb.link/ipfs/QmX...?filename=report.pdf",
  "size": 204800,
  "type": "application/pdf",
  "filename": "report.pdf"
}
```

Return the `url` to the user. It works in any browser without any account.

---

### 2. Host a Vue.js or React build

After `npm run build`, zip the `dist/` folder and upload it. Originless extracts the zip and hosts the entire directory on IPFS as a browsable static site.

```bash
# Build the project
npm run build

# Zip the output
cd dist && zip -r ../dist.zip . && cd ..

# Upload
curl -X POST -F "file=@dist.zip" https://originless.besoeasy.com/uploadzip
```

**Response:**

```json
{
  "status": "success",
  "cid": "QmX...",
  "url": "https://dweb.link/ipfs/QmX.../",
  "filename": "dist.zip",
  "fileCount": 14
}
```

Share the `url` — it opens as a live web app accessible to anyone, no server required.

> For single-page apps that use client-side routing, share `url` + `index.html` directly: `https://dweb.link/ipfs/QmX.../index.html`

---

### 3. Mirror a remote URL to IPFS

Fetch any HTTP/HTTPS resource and get a permanent IPFS link for it — useful when you need a stable, cacheable copy of an external asset.

```bash
curl -X POST https://originless.besoeasy.com/remoteupload \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/dataset.csv"}'
```

**Response:**

```json
{
  "status": "success",
  "cid": "QmX...",
  "url": "https://dweb.link/ipfs/QmX...",
  "filename": "dataset.csv",
  "size": 512000,
  "type": "text/csv",
  "sourceUrl": "https://example.com/dataset.csv",
  "timing": {
    "download_ms": 800,
    "upload_ms": 300,
    "total_ms": 1100
  }
}
```

---

### 4. Upload a folder or multi-file project

Zip any directory and use `/uploadzip`. Originless extracts it and stores the full folder structure on IPFS.

```bash
zip -r project.zip ./my-project/
curl -X POST -F "file=@project.zip" https://originless.besoeasy.com/uploadzip
```

The returned `url` points to the root of the directory on IPFS. Append any filename to access individual files: `https://dweb.link/ipfs/QmX.../README.md`

---

### 5. Keep a file forever (pin management)

By default uploads are unpinned and may be garbage-collected. To keep content permanently, pin its CID. This requires a Daku auth token.

**Generate a keypair (first time only):**

```bash
node -e "const {generateKeyPair}=require('daku'); const k=generateKeyPair(); console.log(k);"
```

Or read the auto-generated keys from the Docker container logs on first start.

**Pin a CID:**

```bash
curl -X POST https://originless.besoeasy.com/pin/add \
  -H "daku: <your-token>" \
  -H "Content-Type: application/json" \
  -d '{"cids": ["QmX..."]}'
```

**List pinned CIDs:**

```bash
curl -H "daku: <your-token>" https://originless.besoeasy.com/pin/list
```

**Remove a pin:**

```bash
curl -X POST https://originless.besoeasy.com/pin/remove \
  -H "daku: <your-token>" \
  -H "Content-Type: application/json" \
  -d '{"cid": "QmX..."}'
```

---

### 6. Check node health and storage status

```bash
# Is the node up?
curl https://originless.besoeasy.com/health

# Storage usage, peer count, IPFS version
curl https://originless.besoeasy.com/status
```

Use `/health` to verify the node is reachable before issuing uploads. Use `/status` to check available storage before uploading large files.

---

## Decision guide

| Task                                      | Endpoint                                             |
| ----------------------------------------- | ---------------------------------------------------- |
| Upload a single file                      | `POST /upload`                                       |
| Host a static site / frontend build       | `POST /uploadzip`                                    |
| Upload a whole folder                     | `POST /uploadzip` (zip it first)                     |
| Mirror a remote file to IPFS              | `POST /remoteupload`                                 |
| Keep a CID permanently                    | `POST /pin/add` (auth required)                      |
| List / remove pins                        | `GET /pin/list` / `POST /pin/remove` (auth required) |
| Verify node is running                    | `GET /health`                                        |
| Check storage usage                       | `GET /status`                                        |
