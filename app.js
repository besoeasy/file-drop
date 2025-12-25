const express = require("express");
const multer = require("multer");
const axios = require("axios");
const FormData = require("form-data");
const path = require("path");
const cors = require("cors");
const compression = require("compression");
const fs = require("fs");
const { promisify } = require("util");
const mime = require("mime-types");
const unlinkAsync = promisify(fs.unlink);

// Parse human-readable size format (e.g., "5GB", "50MB") to bytes
const parseSize = (sizeStr) => {
  const units = {
    B: 1,
    KB: 1024,
    MB: 1024 * 1024,
    GB: 1024 * 1024 * 1024,
    TB: 1024 * 1024 * 1024 * 1024,
  };

  const match = sizeStr.trim().match(/^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$/i);
  if (!match) {
    throw new Error(`Invalid size format: ${sizeStr}`);
  }

  const value = parseFloat(match[1]);
  const unit = match[2].toUpperCase();
  return Math.floor(value * units[unit]);
};

// Constants
const IPFS_API = "http://127.0.0.1:5001";
const PORT = 3232;
const STORAGE_MAX = process.env.STORAGE_MAX || "200GB";
const FILE_LIMIT = parseSize(process.env.FILE_LIMIT || "5GB");
const HOST = "0.0.0.0";
const UPLOAD_TEMP_DIR = "/tmp/filedrop";

// Ensure temp directory exists
if (!fs.existsSync(UPLOAD_TEMP_DIR)) {
  fs.mkdirSync(UPLOAD_TEMP_DIR, { recursive: true });
}

// Cleanup stale chunk uploads every 5 minutes
const CHUNK_TIMEOUT = 30 * 60 * 1000; // 30 minutes
setInterval(() => {
  const now = Date.now();
  for (const [uploadId, tracking] of chunkTracking.entries()) {
    if (now - tracking.startTime > CHUNK_TIMEOUT) {
      console.log(`Cleaning up stale upload: ${uploadId}`);
      chunkTracking.delete(uploadId);
      
      // Clean up temp files
      const tempPath = path.join(UPLOAD_TEMP_DIR, uploadId);
      unlinkAsync(tempPath).catch(() => {});
    }
  }
}, 5 * 60 * 1000);

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
  limits: {
    fileSize: FILE_LIMIT, // Max file size in bytes
  },
  fileFilter: (req, file, cb) => {
    cb(null, true);
  },
});

// Error handling middleware
const errorHandler = (err, req, res, next) => {
  // Handle Multer file size limit error
  if (err instanceof multer.MulterError && err.code === "LIMIT_FILE_SIZE") {
    return res.status(413).json({
      error: "File too large",
      message: `File exceeds the maximum allowed size of ${process.env.FILE_LIMIT || "5GB"}`,
      maxSize: process.env.FILE_LIMIT || "5GB",
    });
  }

  console.error("Unexpected error:", err.stack);
  res.status(500).json({ error: "Internal server error" });
};

// IPFS Gateway Proxy - allows access to IPFS content via /ipfs/CID
app.get("/ipfs/:cid*", async (req, res) => {
  try {
    const ipfsPath = req.path; // e.g., /ipfs/QmXxx or /ipfs/QmXxx/file.txt

    // Forward the request to local IPFS gateway
    const response = await axios.get(`http://127.0.0.1:8080${ipfsPath}`, {
      responseType: "stream",
      timeout: 30000, // 30 second timeout
    });

    // Forward headers from IPFS gateway
    res.set({
      "Content-Type": response.headers["content-type"],
      "Content-Length": response.headers["content-length"],
      "Cache-Control": response.headers["cache-control"] || "public, max-age=31536000, immutable",
    });

    // Pipe the response stream
    response.data.pipe(res);
  } catch (err) {
    console.error("IPFS gateway proxy error:", err.message);

    if (err.response?.status === 404) {
      res.status(404).json({ error: "IPFS content not found", cid: req.params.cid });
    } else {
      res.status(500).json({ error: "Failed to fetch from IPFS gateway", details: err.message });
    }
  }
});

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

// Track ongoing chunked uploads to prevent race conditions
const chunkTracking = new Map();

// Shared upload handler logic
const handleUpload = async (req, res) => {
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
      const chunkTrackPath = path.join(UPLOAD_TEMP_DIR, `${uploadId}.chunks`);

      // Initialize chunk tracking for this upload
      if (!chunkTracking.has(uploadId)) {
        chunkTracking.set(uploadId, {
          receivedChunks: new Set(),
          totalChunks: total,
          originalName: req.file.originalname,
          startTime: Date.now(),
        });
      }

      const tracking = chunkTracking.get(uploadId);

      // Check for duplicate chunk
      if (tracking.receivedChunks.has(currentChunk)) {
        console.log(`Duplicate chunk ${currentChunk} for upload ${uploadId}`);
        await unlinkAsync(req.file.path).catch(console.warn);
        return res.json({ status: "success", message: "Chunk already received" });
      }

      // Use async stream operations instead of sync reads
      const chunkStream = fs.createReadStream(req.file.path);
      const writeStream = fs.createWriteStream(tempFinalPath, { flags: 'a' });
      
      await new Promise((resolve, reject) => {
        chunkStream.pipe(writeStream);
        writeStream.on('finish', resolve);
        writeStream.on('error', reject);
        chunkStream.on('error', reject);
      });

      // Mark chunk as received
      tracking.receivedChunks.add(currentChunk);
      
      // Delete the chunk file from multer
      await unlinkAsync(req.file.path).catch(console.warn);

      console.log(`Received chunk ${currentChunk + 1}/${total} for upload ${uploadId}`);

      // If this is not the last chunk OR not all chunks received, return success
      if (tracking.receivedChunks.size < total) {
        return res.json({ 
          status: "success",
          chunksReceived: tracking.receivedChunks.size,
          chunksTotal: total
        });
      }

      // All chunks received: Set filePath to the assembled file
      filePath = tempFinalPath;
      
      console.log(`File assembly complete for ${tracking.originalName} (${total} chunks, took ${Date.now() - tracking.startTime}ms)`);
      
      // Clean up tracking
      chunkTracking.delete(uploadId);

    } else {
      // Standard Upload Logic
      filePath = req.file.path;
    }

    // --- IPFS Upload Logic (Shared) ---
    // Prepare file for IPFS using stream
    const formData = new FormData();
    const fileStream = fs.createReadStream(filePath);

    // Detect correct MIME type from file extension
    const mimeType = mime.lookup(req.file.originalname) || req.file.mimetype || 'application/octet-stream';

    formData.append("file", fileStream, {
      filename: req.file.originalname,
      contentType: mimeType,
      knownLength: isChunked ? fs.statSync(filePath).size : req.file.size,
    });

    // Upload to IPFS
    const uploadStart = Date.now();
    console.log(`Starting IPFS upload for ${req.file.originalname} ...`);

    const response = await axios.post(`${IPFS_API}/api/v0/add?pin=false`, formData, {
      headers: { ...formData.getHeaders() },
      timeout: 3600000, // 1 hour timeout
      maxContentLength: Infinity,
      maxBodyLength: Infinity,
    });

    // Detailed logging
    const uploadDetails = {
      name: req.file.originalname,
      size_bytes: isChunked ? fs.statSync(filePath).size : req.file.size,
      mime_type: mimeType,
      cid: response.data.Hash,
      upload_duration_ms: Date.now() - uploadStart,
      timestamp: new Date().toISOString(),
    };
    console.log("File uploaded successfully:", uploadDetails);

    // Clean up temp file after successful upload
    await unlinkAsync(filePath).catch((err) => console.warn("Failed to delete temp file:", err.message));

    // Simple response
    res.json({
      status: "success",
      url: `https://dweb.link/ipfs/${response.data.Hash}?filename=${encodeURIComponent(req.file.originalname)}`,
      cid: response.data.Hash,
      size: uploadDetails.size_bytes,
      type: mimeType,
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
};

// PUT Upload endpoint
app.put("/upload", upload.single("file"), handleUpload);

// POST Upload endpoint (alternative method)
app.post("/upload", upload.single("file"), handleUpload);

// Apply error handler
app.use(errorHandler);

// Start server
app.listen(PORT, HOST, () => {
  console.log(`Server running on http://${HOST}:${PORT}`);
  console.log(`IPFS API endpoint: ${IPFS_API}`);
});
