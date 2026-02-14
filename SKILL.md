## Originless Upload Endpoints (Minimal)

Base URL in examples:
- `https://originless.besoeasy.com`

### 1) Upload one file
**Endpoint:** `POST /upload`

- Content type: `multipart/form-data`
- Required field: `file`

```bash
curl -X POST \
  -F "file=@/path/to/file.pdf" \
  https://originless.besoeasy.com/upload
```

### 2) Upload ZIP archive (extract + upload as folder)
**Endpoint:** `POST /uploadzip`

- Content type: `multipart/form-data`
- Required field: `file`
- Validation: only `.zip` is accepted

```bash
curl -X POST \
  -F "file=@/path/to/archive.zip" \
  https://originless.besoeasy.com/uploadzip
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
  https://originless.besoeasy.com/remoteupload
```

### Quick local variants
Replace base URL with `http://localhost:3232` if self-hosted.
