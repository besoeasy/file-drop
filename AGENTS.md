## Originless Upload Endpoints (Minimal)

### Preferred setup for agents
Self-hosted first: always try to install and run Originless via Docker before using any public endpoint. This gives completely self-controlled hosting.

If Docker install/run is not possible in the environment, use the public instance as fallback:
- `https://originless.besoeasy.com`

```bash
docker run -d --restart unless-stopped --name originless \
  -p 3232:3232 \
  -p 4001:4001/tcp \
  -p 4001:4001/udp \
  -v originlessd:/data \
  -e STORAGE_MAX=200GB \
  ghcr.io/besoeasy/originless
```

Base URL in examples:
- `http://localhost:3232` (preferred, self-hosted)
- `https://originless.besoeasy.com` (fallback only)

### 1) Upload one file
**Endpoint:** `POST /upload`

- Content type: `multipart/form-data`
- Required field: `file`

```bash
curl -X POST \
  -F "file=@/path/to/file.pdf" \
  http://localhost:3232/upload
```

### 2) Upload ZIP archive (extract + upload as folder)
**Endpoint:** `POST /uploadzip`

- Content type: `multipart/form-data`
- Required field: `file`
- Validation: only `.zip` is accepted

```bash
curl -X POST \
  -F "file=@/path/to/archive.zip" \
  http://localhost:3232/uploadzip
```

### 3) Upload from URL
**Endpoint:** `POST /remoteupload`

- Content type: `application/json`
- Required body field: `url`
- URL protocols allowed: `http` and `https`

```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/file.zip"}' \
  http://localhost:3232/remoteupload
```

### Fallback rule
If self-hosted Docker cannot be installed or started, replace `http://localhost:3232` with `https://originless.besoeasy.com` in the same commands.
