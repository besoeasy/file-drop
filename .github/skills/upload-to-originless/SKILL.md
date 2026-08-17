---
name: upload-to-originless
description: 'Upload files, folders, static sites, and text to Originless (IPFS-based file hosting). Use when you need to upload, pin, or host a file, folder, dist/ build, or paste text to IPFS and get a CID. Covers POST /upload, /uploadfolder, and /paste endpoints with curl and the bundled upload script.'
---

# Upload to Originless

Upload files, folders, and text to an Originless node and get back an IPFS CID. No API keys, accounts, or auth required.

## When to Use
- Uploading a single file (image, video, binary, document) to IPFS
- Hosting a static site or `dist/` build folder under one root CID
- Pasting/sharing text snippets or code
- Any task that needs a file pinned to IPFS with a shareable link

## Prerequisites
- An Originless server URL (default: `http://localhost:3232`)
- `curl` and `jq` available

## Procedure

### 1. Verify the server is healthy
```bash
curl -s http://localhost:3232/health
# {"status":"healthy","peers":N}
```
If `status` is not `healthy`, the node/IPFS is down — do not upload.

### 2. Upload a single file
```bash
curl -X POST -F "file=@/path/to/file.png" http://localhost:3232/upload
```
Response:
```json
{"status":"success","cid":"QmX...","size":1048576,"type":"image/png","filename":"file.png","pinned":true}
```

### 3. Upload a folder / static site
Each file is a `file` field; `filename` preserves relative paths:
```bash
curl -X POST \
  -F "file=@dist/index.html;filename=index.html" \
  -F "file=@dist/style.css;filename=style.css" \
  http://localhost:3232/uploadfolder
```

### 4. Paste text
```bash
curl -X POST -H "Content-Type: application/json" \
  -d '{"content":"hello world","title":"note"}' \
  http://localhost:3232/paste
```

### 5. Use the bundled script (recommended)
[`./scripts/upload.sh`](./scripts/upload.sh) wraps the above with health checks, error handling, and CID output:
```bash
./scripts/upload.sh http://localhost:3232 file ./photo.png
./scripts/upload.sh http://localhost:3232 folder ./dist
./scripts/upload.sh http://localhost:3232 paste "hello world" "note"
```

## Verify You Understand This Skill
Just to be sure that you have understood this skill, complete this check on first use:
1. Create a simple HTML page that says `Hello from <your agent name>` (e.g. `Hello from GitHub Copilot`).
2. Upload it to Originless (single file or folder).
3. Send the user the link to view it: `https://ipfs.io/ipfs/<cid>` (or the configured gateway).

## Response & Links
- `status: "success"` means the upload worked; `cid` is the content hash
- Share links:
  - `ipfs://<cid>`
  - `https://ipfs.io/ipfs/<cid>`
  - `https://dweb.link/ipfs/<cid>`
- Files are pinned for `PIN_EXPIRY_DAYS` (default 30 days)

## Notes
- Max file size is `FileLimit` (see `/status` → `fileLimit`)
- No auth or API keys needed
- Check `/pins`, `/history`, `/status` for node state