const express = require("express");
const multer = require("multer");
const axios = require("axios");
const FormData = require("form-data");
const path = require("path");
const cors = require("cors");
const compression = require("compression");
const fs = require("fs");
const crypto = require("crypto");
const { promisify } = require("util");
const unlinkAsync = promisify(fs.unlink);

// Constants
const IPFS_API = "http://127.0.0.1:5001";
const PORT = 3232;
const STORAGE_MAX = process.env.STORAGE_MAX || "200GB";
const HOST = "0.0.0.0";
const UPLOAD_TEMP_DIR = "/tmp/filedrop";

// Ensure temp directory exists
if (!fs.existsSync(UPLOAD_TEMP_DIR)) {
  fs.mkdirSync(UPLOAD_TEMP_DIR, { recursive: true });
}

// Helper function to calculate SHA256 hash of a file
const calculateSHA256 = (filePath) => {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash("sha256");
    const stream = fs.createReadStream(filePath);
    stream.on("data", (data) => hash.update(data));
    stream.on("end", () => resolve(hash.digest("hex")));
    stream.on("error", reject);
  });
};

// Initialize Express app
const app = express();

// Middleware setup
app.use(compression());
app.use(express.json());
app.use(express.urlencoded({ extended: true }));
app.use(cors());
app.use(express.static(path.join(__dirname, "public")));

// Configure multer for file uploads with disk storage for streaming
const storage = multer.diskStorage({
  destination: (req, file, cb) => {
    cb(null, UPLOAD_TEMP_DIR);
  },
  filename: (req, file, cb) => {
    // Generate unique filename with timestamp
    const uniqueSuffix = Date.now() + "-" + Math.round(Math.random() * 1e9);
    cb(null, uniqueSuffix + "-" + file.originalname);
  },
});

const upload = multer({
  storage,
  fileFilter: (req, file, cb) => {
    cb(null, true);
  },
});

// Error handling middleware
const errorHandler = (err, req, res, next) => {
  console.error("Unexpected error:", err.stack);
  res.status(500).json({ error: "Internal server error" });
};

// Health check endpoint for Docker
app.get("/health", async (req, res) => {
  try {
    const peersResponse = await axios.post(`${IPFS_API}/api/v0/swarm/peers`, { timeout: 5000 });
    const peerCount = peersResponse.data.Peers?.length || 0;

    if (peerCount >= 1) {
      res.status(200).json({ status: "healthy", peers: peerCount });
    } else {
      res.status(503).json({ status: "unhealthy", peers: peerCount, reason: "No peers connected" });
    }
  } catch (err) {
    res.status(503).json({ status: "unhealthy", error: err.message });
  }
});

// Enhanced status endpoint
app.get("/status", async (req, res) => {
  try {
    // Fetch multiple IPFS stats concurrently
    const [bwResponse, repoResponse, idResponse] = await Promise.all([
      axios.post(`${IPFS_API}/api/v0/stats/bw?interval=5m`, { timeout: 5000 }),
      axios.post(`${IPFS_API}/api/v0/repo/stat`, { timeout: 5000 }),
      axios.post(`${IPFS_API}/api/v0/id`, { timeout: 5000 }),
    ]);

    // Format bandwidth data
    const bandwidth = {
      totalIn: bwResponse.data.TotalIn,
      totalOut: bwResponse.data.TotalOut,
      rateIn: bwResponse.data.RateIn,
      rateOut: bwResponse.data.RateOut,
      interval: "1h",
    };

    // Format repository stats
    const repo = {
      size: repoResponse.data.RepoSize,
      storageMax: repoResponse.data.StorageMax,
      numObjects: repoResponse.data.NumObjects,
      path: repoResponse.data.RepoPath,
      version: repoResponse.data.Version,
    };

    // Get GC configuration info
    let gcInfo = {
      enabled: true,
      period: "200h",
      lastRun: "Unknown",
    };

    try {
      const configResponse = await axios.post(`${IPFS_API}/api/v0/config/show`, { timeout: 3000 });
      if (configResponse.data && configResponse.data.Datastore) {
        gcInfo.period = configResponse.data.Datastore.GCPeriod || "200h";
      }
    } catch (configErr) {
      console.log("Could not fetch GC config:", configErr.message);
    }

    // Node identity info
    const nodeInfo = {
      id: idResponse.data.ID,
      publicKey: idResponse.data.PublicKey,
      addresses: idResponse.data.Addresses,
      agentVersion: idResponse.data.AgentVersion,
      protocolVersion: idResponse.data.ProtocolVersion,
    };

    const peersResponse = await axios.post(`${IPFS_API}/api/v0/swarm/peers`, {
      timeout: 5000,
    });

    const connectedPeers = {
      count: peersResponse.data.Peers.length,
      list: peersResponse.data.Peers,
    };

    // Get app version from package.json
    const { version: appVersion } = require("./package.json");

    // Format file size limit in human readable form
    const formatBytes = (bytes) => {
      const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
      if (bytes === 0) return "0 Bytes";
      const i = Math.floor(Math.log(bytes) / Math.log(1024));
      return (bytes / Math.pow(1024, i)).toFixed(2) + " " + sizes[i];
    };

    res.json({
      status: "success",
      timestamp: new Date().toISOString(),
      bandwidth,
      repository: repo,
      node: nodeInfo,
      peers: connectedPeers,
      garbageCollection: gcInfo,
      storageLimit: {
        configured: STORAGE_MAX,
        current: formatBytes(repo.storageMax),
      },
      appVersion,
    });
  } catch (err) {
    console.error("Status check error:", {
      message: err.message,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    });

    res.status(503).json({
      error: "Failed to retrieve IPFS status",
      details: err.message,
      status: "failed",
      timestamp: new Date().toISOString(),
    });
  }
});

// Store for mapping SHA256 hashes to CIDs
const blobStore = new Map(); // sha256 -> { cid, size, type, uploaded, filename }

// CORS preflight for /upload and /:sha256
app.options(["/upload", "/upload/:sha256", "/:sha256"], (req, res) => {
  res.header("Access-Control-Allow-Methods", "GET, HEAD, PUT, DELETE");
  res.header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-SHA-256, X-Content-Length, X-Content-Type");
  res.header("Access-Control-Max-Age", "86400");
  res.status(204).send();
});

// HEAD /upload - Upload requirements (BUD-06)
app.head("/upload", (req, res) => {
  const sha256 = req.headers["x-sha-256"];
  const contentLength = req.headers["x-content-length"];
  const contentType = req.headers["x-content-type"];

  // Validate required headers
  if (!sha256) {
    res.header("X-Reason", "Missing X-SHA-256 header");
    return res.status(400).send();
  }

  if (!contentLength) {
    res.header("X-Reason", "Missing X-Content-Length header");
    return res.status(411).send();
  }

  // Optional: Add size limits
  const maxSize = 100 * 1024 * 1024 * 1024; // 100GB
  if (parseInt(contentLength) > maxSize) {
    res.header("X-Reason", "File too large. Max allowed size is 100GB");
    return res.status(413).send();
  }

  // Optional: Validate content type
  if (contentType && !contentType.match(/^[a-z]+\/[a-z0-9\-\+\.]+$/i)) {
    res.header("X-Reason", "Invalid X-Content-Type header format");
    return res.status(400).send();
  }

  // Upload can proceed
  res.status(200).send();
});

// Blossom BUD-01: PUT /upload/:sha256 - Upload blob by SHA256
app.put("/upload/:sha256", async (req, res) => {
  const expectedSha256 = req.params.sha256.toLowerCase();
  let tempFilePath = null;

  try {
    // Save raw body to temp file
    tempFilePath = path.join(UPLOAD_TEMP_DIR, `blossom-${Date.now()}-${expectedSha256}`);
    const writeStream = fs.createWriteStream(tempFilePath);
    
    await new Promise((resolve, reject) => {
      req.pipe(writeStream);
      writeStream.on("finish", resolve);
      writeStream.on("error", reject);
    });

    // Calculate SHA256 hash to verify
    const actualSha256 = await calculateSHA256(tempFilePath);
    
    if (actualSha256 !== expectedSha256) {
      await unlinkAsync(tempFilePath).catch(console.warn);
      return res.status(400).json({
        error: "SHA256 mismatch",
        expected: expectedSha256,
        actual: actualSha256,
      });
    }

    // Get file stats
    const stats = fs.statSync(tempFilePath);
    const fileSize = stats.size;
    const contentType = req.headers["content-type"] || "application/octet-stream";

    // Upload to IPFS
    const formData = new FormData();
    const fileStream = fs.createReadStream(tempFilePath);
    formData.append("file", fileStream, {
      filename: expectedSha256,
      contentType: contentType,
      knownLength: fileSize,
    });

    console.log(`Uploading blob ${expectedSha256} to IPFS...`);
    const response = await axios.post(`${IPFS_API}/api/v0/add`, formData, {
      headers: { ...formData.getHeaders() },
      timeout: 3600000,
      maxContentLength: Infinity,
      maxBodyLength: Infinity,
    });

    const cid = response.data.Hash;
    const uploaded = Math.floor(Date.now() / 1000);

    // Store in blob store
    blobStore.set(expectedSha256, {
      cid,
      size: fileSize,
      type: contentType,
      uploaded,
      filename: expectedSha256,
    });

    console.log(`Blob uploaded: sha256=${expectedSha256}, cid=${cid}, size=${fileSize}`);

    // Clean up temp file
    await unlinkAsync(tempFilePath).catch(console.warn);

    // Return Blossom-compatible response (BUD-02)
    res.json({
      url: `https://dweb.link/ipfs/${cid}`,
      sha256: expectedSha256,
      size: fileSize,
      type: contentType,
      uploaded: uploaded,
      cid: cid,
    });
  } catch (err) {
    if (tempFilePath) {
      await unlinkAsync(tempFilePath).catch(console.warn);
    }

    console.error("Blossom upload error:", err.message);
    res.status(500).json({
      error: "Failed to upload blob",
      details: err.message,
    });
  }
});

// Blossom BUD-02: GET /:sha256 - Retrieve blob by SHA256
app.get("/:sha256", async (req, res) => {
  const sha256 = req.params.sha256.toLowerCase();
  
  // Check if it's a valid SHA256 (64 hex chars)
  if (!/^[a-f0-9]{64}$/i.test(sha256)) {
    return res.status(404).json({ error: "Invalid SHA256 format" });
  }

  const blob = blobStore.get(sha256);
  if (!blob) {
    return res.status(404).json({ error: "Blob not found" });
  }

  try {
    // Redirect to IPFS gateway
    res.redirect(`https://dweb.link/ipfs/${blob.cid}`);
  } catch (err) {
    console.error("Blob retrieval error:", err.message);
    res.status(500).json({ error: "Failed to retrieve blob" });
  }
});

// HEAD /:sha256 - Check if blob exists
app.head("/:sha256", (req, res) => {
  const sha256 = req.params.sha256.toLowerCase();
  
  if (!/^[a-f0-9]{64}$/i.test(sha256)) {
    return res.status(404).send();
  }

  const blob = blobStore.get(sha256);
  if (!blob) {
    return res.status(404).send();
  }

  res.header("Content-Type", blob.type);
  res.header("Content-Length", blob.size.toString());
  res.header("X-SHA-256", sha256);
  res.status(200).send();
});

// Blossom BUD-04: GET /list/:pubkey - List blobs (simplified)
app.get("/list/:pubkey?", (req, res) => {
  const blobs = Array.from(blobStore.entries()).map(([sha256, data]) => ({
    url: `https://dweb.link/ipfs/${data.cid}`,
    sha256,
    size: data.size,
    type: data.type,
    uploaded: data.uploaded,
    cid: data.cid,
  }));

  res.json(blobs);
});

// Upload endpoint (multipart form for web UI)
app.put("/upload", upload.single("file"), async (req, res) => {
  let filePath = null;

  try {
    // Validate file presence
    if (!req.file) {
      return res.status(400).json({
        error: "No file uploaded",
        status: "error",
        message: "No file uploaded",
        timestamp: new Date().toISOString(),
      });
    }

    // Check for chunked upload parameters
    const { uploadId, chunkIndex, totalChunks } = req.body;
    const isChunked = uploadId !== undefined && chunkIndex !== undefined;

    if (isChunked) {
      // Chunked Upload Logic
      const currentChunk = parseInt(chunkIndex);
      const total = parseInt(totalChunks);
      const tempFinalPath = path.join(UPLOAD_TEMP_DIR, uploadId);

      // Append chunk to the final file
      // Using sync operations for 256KB chunks is acceptable and ensures order if requests arrive sequentially
      const chunkData = fs.readFileSync(req.file.path);
      fs.appendFileSync(tempFinalPath, chunkData);

      // Delete the chunk file from multer
      await unlinkAsync(req.file.path).catch(console.warn);

      // If this is not the last chunk, return success immediately
      if (currentChunk + 1 < total) {
        return res.json({ status: "success" });
      }

      // Last chunk received: Set filePath to the assembled file
      filePath = tempFinalPath;

      // Note: We need to give it the original name for the IPFS upload to use correct filename
      // The simplest way is to rename it or just pass metadata to the IPFS step
      // The existing logic below expects 'req.file.originalname' for headers
      // We will let the flow continue to the IPFS upload section using the assembled filePath
      console.log(`File assembly complete for ${req.file.originalname} (${total} chunks)`);

    } else {
      // Standard Upload Logic
      filePath = req.file.path;
    }

    // --- IPFS Upload Logic (Shared) ---
    // Calculate SHA256 hash before uploading
    const sha256Hash = await calculateSHA256(filePath);
    
    // Prepare file for IPFS using stream
    const formData = new FormData();
    const fileStream = fs.createReadStream(filePath);

    formData.append("file", fileStream, {
      filename: req.file.originalname,
      contentType: req.file.mimetype,
      knownLength: isChunked ? fs.statSync(filePath).size : req.file.size,
    });

    // Upload to IPFS
    const uploadStart = Date.now();
    console.log(`Starting IPFS upload for ${req.file.originalname} ...`);

    const response = await axios.post(`${IPFS_API}/api/v0/add`, formData, {
      headers: { ...formData.getHeaders() },
      timeout: 3600000, // 1 hour timeout
      maxContentLength: Infinity,
      maxBodyLength: Infinity,
    });

    // Detailed logging
    const uploadDetails = {
      name: req.file.originalname,
      size_bytes: isChunked ? fs.statSync(filePath).size : req.file.size,
      mime_type: req.file.mimetype,
      cid: response.data.Hash,
      sha256: sha256Hash,
      upload_duration_ms: Date.now() - uploadStart,
      timestamp: new Date().toISOString(),
    };
    console.log("File uploaded successfully:", uploadDetails);

    const uploaded = Math.floor(Date.now() / 1000);

    // Store in blob store for Blossom compatibility
    blobStore.set(sha256Hash, {
      cid: response.data.Hash,
      size: uploadDetails.size_bytes,
      type: req.file.mimetype,
      uploaded: uploaded,
      filename: req.file.originalname,
    });

    // Clean up temp file after successful upload
    await unlinkAsync(filePath).catch((err) => console.warn("Failed to delete temp file:", err.message));

    // Blossom-compatible response format with additional fields
    res.json({
      status: "success",
      url: `https://dweb.link/ipfs/${response.data.Hash}`,
      sha256: sha256Hash,
      size: uploadDetails.size_bytes,
      type: req.file.mimetype,
      uploaded: uploaded,
      cid: response.data.Hash,
      filename: req.file.originalname,
    });
  } catch (err) {
    if (filePath) {
      await unlinkAsync(filePath).catch((cleanupErr) => console.warn("Failed to delete temp file on error:", cleanupErr.message));
    }

    if (err instanceof multer.MulterError) {
      return res.status(400).json({
        error: err.message,
        status: "error",
        message: err.message,
        timestamp: new Date().toISOString(),
      });
    }

    console.error("IPFS upload error:", {
      message: err.message,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    });

    res.status(500).json({
      error: "Failed to upload to IPFS",
      details: err.message,
      status: "error",
      message: "Failed to upload to IPFS",
      timestamp: new Date().toISOString(),
    });
  }
});

// Apply error handler
app.use(errorHandler);

// Start server
app.listen(PORT, HOST, () => {
  console.log(`Server running on http://${HOST}:${PORT}`);
  console.log(`IPFS API endpoint: ${IPFS_API}`);
});
