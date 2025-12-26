# File Drop API Specification

Complete API reference for integrating File Drop into your application.

---

## Table of Contents

- [Overview](#overview)
- [Base Configuration](#base-configuration)
- [Upload Strategies](#upload-strategies)
- [Endpoints](#endpoints)
  - [Upload File](#upload-file)
  - [Health Check](#health-check)
  - [Server Status](#server-status)
  - [IPFS Gateway Proxy](#ipfs-gateway-proxy)
- [Error Handling](#error-handling)
- [Client Implementation](#client-implementation)
- [Best Practices](#best-practices)

---

## Overview

File Drop provides a simple HTTP API for uploading files to IPFS. It supports single-file uploads up to 5GB with direct IPFS integration.

**Base URL:** `http://localhost:3232` (or your deployed server)

**Authentication:** None required

**Content-Type:** `multipart/form-data` for uploads

---

## Base Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `STORAGE_MAX` | `200GB` | Maximum IPFS repository size |
| `FILE_LIMIT` | `5GB` | Maximum file size per upload |
| `PORT` | `3232` | HTTP server port |
| `IPFS_API` | `http://127.0.0.1:5001` | IPFS API endpoint |

### System Limits

- **Max File Size:** 5GB (configurable via `FILE_LIMIT`)
- **Request Timeout:** 1 hour per upload

---

## Upload Strategy

**Single Upload**

- Entire file sent in one HTTP request
- Simple and straightforward
- Direct to IPFS upload
- Supports files up to 5GB

---

## Endpoints

### Upload File

Upload a file to IPFS and receive a content-addressed URL.

**Request:**
```http
POST /upload HTTP/1.1
Content-Type: multipart/form-data

file: <binary file data>
```

**Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `file` | binary | Yes | File data (up to 5GB) |

**Example (curl):**
```bash
curl -X POST \
  -F "file=@document.pdf" \
  http://localhost:3232/upload
```

**Response (200 OK):**
```json
{
  "status": "success",
  "url": "https://dweb.link/ipfs/QmXXXXXX?filename=document.pdf",
  "cid": "QmXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
  "size": 1048576,
  "type": "application/pdf",
  "filename": "document.pdf"
}
```

**Response Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Always "success" on successful upload |
| `url` | string | Public IPFS gateway URL for accessing the file |
| `cid` | string | IPFS Content Identifier (hash of the file) |
| `size` | number | File size in bytes |
| `type` | string | MIME type of the file |
| `filename` | string | Original filename |

**Error Response (400 Bad Request):**
```json
{
  "error": "No file uploaded",
  "status": "error",
  "message": "No file uploaded"
}
```

**Error Response (413 Payload Too Large):**
```json
{
  "error": "File too large",
  "message": "File exceeds the maximum allowed size of 5GB",
  "maxSize": "5GB"
}
```

---

### Health Check

Check server health and IPFS connectivity.

**Request:**
```http
GET /health HTTP/1.1
```

**Example (curl):**
```bash
curl http://localhost:3232/health
```

**Response (200 OK - Healthy):**
```json
{
  "status": "healthy",
  "peers": 42
}
```

**Response (503 Service Unavailable - Unhealthy):**
```json
{
  "status": "unhealthy",
  "peers": 0,
  "reason": "No peers connected"
}
```

**Health Criteria:**
- `healthy`: Connected to >= 1 IPFS peer
- `unhealthy`: No peer connections or IPFS unavailable

**Use Case:** Load balancing, server selection, health monitoring

---

### Server Status

Get detailed server and IPFS node statistics.

**Request:**
```http
GET /status HTTP/1.1
```

**Example (curl):**
```bash
curl http://localhost:3232/status
```

**Response (200 OK):**
```json
{
  "status": "success",
  "timestamp": "2025-12-25T12:00:00.000Z",
  "bandwidth": {
    "totalIn": 1073741824,
    "totalOut": 536870912,
    "rateIn": 1048576,
    "rateOut": 524288,
    "interval": "1h"
  },
  "repository": {
    "size": 10737418240,
    "storageMax": 214748364800,
    "numObjects": 12345,
    "path": "/data/ipfs",
    "version": "13"
  },
  "node": {
    "id": "QmXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX",
    "publicKey": "CAESIxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
    "addresses": [
      "/ip4/127.0.0.1/tcp/4001/p2p/QmXXXXXX",
      "/ip4/192.168.1.100/tcp/4001/p2p/QmXXXXXX"
    ],
    "agentVersion": "go-ipfs/0.20.0",
    "protocolVersion": "ipfs/0.1.0"
  },
  "peers": {
    "count": 42,
    "list": [
      {
        "Addr": "/ip4/1.2.3.4/tcp/4001",
        "Peer": "QmYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY"
      }
    ]
  },
  "storageLimit": {
    "configured": "200GB",
    "current": "10.00 GB"
  },
  "appVersion": "1.0.0"
}
```

**Response (503 Service Unavailable):**
```json
{
  "error": "Failed to retrieve IPFS status",
  "details": "Connection refused",
  "status": "failed",
  "timestamp": "2025-12-25T12:00:00.000Z"
}
```

---

### IPFS Gateway Proxy

Access IPFS content through the File Drop server.

**Request:**
```http
GET /ipfs/{cid} HTTP/1.1
GET /ipfs/{cid}/{filename} HTTP/1.1
```

**Parameters:**

| Field | Type | Description |
|-------|------|-------------|
| `cid` | string | IPFS Content ID |
| `filename` | string | Optional path within IPFS content |

**Example (curl):**
```bash
# Direct CID access
curl http://localhost:3232/ipfs/QmXXXXXX

# With filename
curl http://localhost:3232/ipfs/QmXXXXXX/document.pdf
```

**Response (200 OK):**
- Content-Type set based on file type
- Binary file data streamed
- Headers: `Cache-Control: public, max-age=31536000, immutable`

**Response (404 Not Found):**
```json
{
  "error": "IPFS content not found",
  "cid": "QmXXXXXX"
}
```

**Response (500 Internal Server Error):**
```json
{
  "error": "Failed to fetch from IPFS gateway",
  "details": "Timeout"
}
```

---

## Error Handling

### HTTP Status Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| `200` | Success | Upload completed successfully |
| `400` | Bad Request | No file provided, invalid parameters |
| `413` | Payload Too Large | File exceeds `FILE_LIMIT` |
| `500` | Internal Server Error | IPFS error, server error |
| `503` | Service Unavailable | IPFS node not responding |

### Error Response Format

All errors follow this structure:

```json
{
  "error": "Error category",
  "message": "Human-readable description",
  "status": "error",
  "timestamp": "2025-12-25T12:00:00.000Z"
}
```

### Common Errors

#### No File Uploaded

**Status:** `400 Bad Request`

```json
{
  "error": "No file uploaded",
  "status": "error",
  "message": "No file uploaded",
  "timestamp": "2025-12-25T12:00:00.000Z"
}
```

#### File Too Large

**Status:** `413 Payload Too Large`

```json
{
  "error": "File too large",
  "message": "File exceeds the maximum allowed size of 5GB",
  "maxSize": "5GB"
}
```

#### IPFS Upload Failed

**Status:** `500 Internal Server Error`

```json
{
  "error": "Failed to upload to IPFS",
  "details": "Connection timeout",
  "status": "error",
  "message": "Failed to upload to IPFS",
  "timestamp": "2025-12-25T12:00:00.000Z"
}
```

#### IPFS Node Unavailable

**Status:** `503 Service Unavailable`

```json
{
  "error": "Failed to retrieve IPFS status",
  "details": "ECONNREFUSED",
  "status": "failed",
  "timestamp": "2025-12-25T12:00:00.000Z"
}
```

---

## Client Implementation

### JavaScript/TypeScript (Browser)

```javascript
async function uploadFile(file, onProgress) {
  const formData = new FormData();
  formData.append('file', file);

  const response = await fetch('http://localhost:3232/upload', {
    method: 'POST',
    body: formData
  });

  if (!response.ok) {
    throw new Error(`Upload failed: ${response.statusText}`);
  }

  return await response.json();
}

// Usage
const result = await uploadFile(fileInput.files[0]);
console.log('CID:', result.cid);
console.log('URL:', result.url);
```

### Python (requests)

```python
import requests

def upload_file(file_path, server_url="http://localhost:3232"):
    """Upload a file to File Drop server."""
    
    with open(file_path, 'rb') as f:
        files = {'file': f}
        response = requests.post(f"{server_url}/upload", files=files)
    
    if response.status_code != 200:
        raise Exception(f"Upload failed: {response.text}")
    
    return response.json()

# Usage
result = upload_file('document.pdf')
print(f"CID: {result['cid']}")
print(f"URL: {result['url']}")
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "io"
    "mime/multipart"
    "net/http"
    "os"
)

type UploadResponse struct {
    Status   string `json:"status"`
    URL      string `json:"url"`
    CID      string `json:"cid"`
    Size     int64  `json:"size"`
    Type     string `json:"type"`
    Filename string `json:"filename"`
}

func uploadFile(filePath string, serverURL string) (*UploadResponse, error) {
    file, err := os.Open(filePath)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    part, err := writer.CreateFormFile("file", filePath)
    if err != nil {
        return nil, err
    }
    
    _, err = io.Copy(part, file)
    if err != nil {
        return nil, err
    }
    
    writer.Close()

    req, err := http.NewRequest("POST", serverURL+"/upload", body)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result UploadResponse
    err = json.NewDecoder(resp.Body).Decode(&result)
    return &result, err
}

// Usage
func main() {
    result, err := uploadFile("document.pdf", "http://localhost:3232")
    if err != nil {
        panic(err)
    }
    println("CID:", result.CID)
    println("URL:", result.URL)
}
```

---

## Best Practices

### 1. Error Handling

```javascript
async function uploadWithErrorHandling(file) {
  try {
    const result = await uploadFile(file);
    return result;
  } catch (error) {
    if (error.response?.status === 413) {
      throw new Error('File too large. Maximum size is 5GB.');
    } else if (error.response?.status === 503) {
      throw new Error('Server unavailable. Please try again later.');
    } else if (error.code === 'ECONNABORTED') {
      throw new Error('Upload timeout. Please check your connection.');
    } else {
      throw new Error('Upload failed. Please try again.');
    }
  }
}
```

### 2. File Validation

```javascript
function validateFile(file, maxSize = 5 * 1024 * 1024 * 1024) {
  if (!file) {
    throw new Error('No file selected');
  }
  
  if (file.size > maxSize) {
    throw new Error(`File size exceeds ${maxSize / (1024**3).toFixed(2)}GB limit`);
  }
  
  if (file.size === 0) {
    throw new Error('File is empty');
  }
  
  return true;
}
```

### 3. Progress Tracking

```javascript
function setupProgressTracking(xhr, onProgress) {
  xhr.upload.addEventListener('progress', (event) => {
    if (event.lengthComputable) {
      const percentComplete = (event.loaded / event.total) * 100;
      if (onProgress) {
        onProgress(percentComplete);
      }
    }
  });
}
```

### 4. Server Health Checking

```javascript
async function checkServerHealth(serverUrl) {
  try {
    const response = await fetch(`${serverUrl}/health`, { timeout: 5000 });
    const data = await response.json();
    return data.status === 'healthy';
  } catch (error) {
    return false;
  }
}
```

### 5. Caching and CID Verification

```javascript
// Cache uploaded files by CID to avoid re-uploads
const uploadCache = new Map();

async function uploadFileWithCache(file) {
  // Check if already uploaded
  const fileHash = await hashFile(file);
  
  if (uploadCache.has(fileHash)) {
    return uploadCache.get(fileHash);
  }
  
  // Upload and cache
  const result = await uploadFile(file);
  uploadCache.set(fileHash, result);
  
  return result;
}
```

### 6. Memory Management (Large Files)

```javascript
// Don't load entire file into memory
async function uploadLargeFile(file) {
  const CHUNK_SIZE = 256 * 1024;
  
  for (let offset = 0; offset < file.size; offset += CHUNK_SIZE) {
    // Read chunk on-demand
    const chunk = file.slice(offset, offset + CHUNK_SIZE);
    await uploadChunk(chunk);
    
    // Allow garbage collection between chunks
    await new Promise(resolve => setTimeout(resolve, 0));
  }
}
```

### 7. Timeout Configuration

```javascript
const config = {
  // Small files: 30 second timeout
  singleUploadTimeout: 30000,
  
  // Per-chunk timeout: 60 seconds
  chunkUploadTimeout: 60000,
  
  // Health check: 5 seconds
  healthCheckTimeout: 5000
};
```

### 8. IPFS Gateway Selection

```javascript
const IPFS_GATEWAYS = [
  'https://dweb.link/ipfs/',
  'https://ipfs.io/ipfs/',
  'https://gateway.pinata.cloud/ipfs/',
  'https://cloudflare-ipfs.com/ipfs/'
];

function getFileUrl(cid, filename, gatewayIndex = 0) {
  const gateway = IPFS_GATEWAYS[gatewayIndex];
  return `${gateway}${cid}?filename=${encodeURIComponent(filename)}`;
}
```

---

## Rate Limiting

File Drop does not implement rate limiting by default. For production deployments, consider:

- **Nginx rate limiting**: `limit_req_zone` and `limit_req`
- **Application-level limits**: Add middleware to track requests per IP
- **File size quotas**: Track total storage per user/IP

---

## Security Considerations

1. **No authentication** - Anyone can upload files
2. **Content addressing** - Files identified by content hash (CID)
3. **Temporary storage** - Files garbage collected when storage fills
4. **No encryption** - Files stored unencrypted on IPFS
5. **Public network** - Files accessible to anyone with the CID

**For private deployments:**
- Use reverse proxy with authentication (nginx, Cloudflare)
- Implement API key system in application layer
- Run IPFS in private network mode
- Add encryption layer before upload

---

## Performance Tips

### Server-Side

1. **Increase storage limit** - Set `STORAGE_MAX` based on available disk
2. **Monitor peer count** - Higher peer count = better propagation
3. **Enable SSD storage** - Faster IPFS operations
4. **Adjust IPFS config** - Tune `Datastore` and `Gateway` settings

### Client-Side

1. **Use chunked uploads** for files > 5MB
2. **Upload 3-5 chunks in parallel** for optimal speed
3. **Implement retry logic** with exponential backoff
4. **Show progress feedback** to users
5. **Compress before upload** for text-based files

---

## Support & Troubleshooting

### Common Issues

**Upload stuck at 99%:**
- IPFS upload may take time for large files
- Check server logs for IPFS errors
- Verify IPFS daemon is running

**"No peers connected":**
- IPFS needs time to bootstrap (1-2 minutes)
- Check firewall allows port 4001
- Verify network connectivity

**"File too large" errors:**
- Increase `FILE_LIMIT` environment variable
- Check available disk space
- Consider splitting file into parts

### Debug Mode

Enable detailed logging:

```bash
docker logs -f file-drop
```

Look for:
- Chunk reception logs
- IPFS upload progress
- Error messages with timestamps

---

## Contributing

Found a bug or want to contribute? Visit:
- **GitHub**: [besoeasy/file-drop](https://github.com/besoeasy/file-drop)
- **Issues**: Report bugs or request features

---

## License

MIT License - See [LICENSE](LICENSE) file for details
