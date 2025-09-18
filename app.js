const express = require("express");
const multer = require("multer");
const axios = require("axios");
const FormData = require("form-data");
const path = require("path");
const cors = require("cors");

// Constants
const IPFS_API = "http://127.0.0.1:5001";
const PORT = 3232;
const MAX_FILE_SIZE = 2000 * 1024 * 1024;
const HOST = "0.0.0.0";

// Initialize Express app
const app = express();

// Middleware setup
app.use(express.json());
app.use(express.urlencoded({ extended: true }));
app.use(cors());
app.use(express.static(path.join(__dirname, "public")));

// Configure multer for file uploads
const storage = multer.memoryStorage();
const upload = multer({
  storage,
  limits: { fileSize: MAX_FILE_SIZE },
  fileFilter: (req, file, cb) => {
    // Add file type validation if needed
    cb(null, true);
  },
});

// Error handling middleware
const errorHandler = (err, req, res, next) => {
  console.error("Unexpected error:", err.stack);
  res.status(500).json({ error: "Internal server error" });
};

// Upload endpoint
app.post("/uploadx", upload.single("file"), async (req, res) => {
  try {
    // Validate file presence
    if (!req.file) {
      return res.status(400).json({
        error: "No file uploaded",
        status: "failed",
        timestamp: new Date().toISOString(),
      });
    }

    // Prepare file for IPFS
    const formData = new FormData();
    formData.append("file", req.file.buffer, {
      filename: req.file.originalname,
      contentType: req.file.mimetype,
      knownLength: req.file.size,
    });

    // Upload to IPFS
    const uploadStart = Date.now();
    const response = await axios.post(`${IPFS_API}/api/v0/add`, formData, {
      headers: { ...formData.getHeaders() },
      timeout: 30000, // 30s timeout
    });

    // Detailed logging
    const uploadDetails = {
      name: req.file.originalname,
      size_bytes: req.file.size,
      mime_type: req.file.mimetype,
      cid: response.data.Hash,
      upload_duration_ms: Date.now() - uploadStart,
      timestamp: new Date().toISOString(),
    };
    console.log("File uploaded successfully:", uploadDetails);

    // Response with more details
    res.json({
      status: "success",
      cid: response.data.Hash,
      filename: req.file.originalname,
      size: req.file.size,
      details: uploadDetails,
    });
  } catch (err) {
    if (err instanceof multer.MulterError) {
      if (err.code === "LIMIT_FILE_SIZE") {
        return res.status(400).json({
          error: `File too large. Max size is ${MAX_FILE_SIZE / (1024 * 1024)}MB`,
          status: "failed",
          timestamp: new Date().toISOString(),
        });
      }
      return res.status(400).json({ error: err.message, status: "failed" });
    }

    console.error("IPFS upload error:", {
      message: err.message,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    });

    res.status(500).json({
      error: "Failed to upload to IPFS",
      details: err.message,
      status: "failed",
      timestamp: new Date().toISOString(),
    });
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

    res.json({
      status: "success",
      timestamp: new Date().toISOString(),
      bandwidth,
      repository: repo,
      node: nodeInfo,
      peers: connectedPeers,
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

// Apply error handler
app.use(errorHandler);

// Start server
app.listen(PORT, HOST, () => {
  console.log(`Server running on http://${HOST}:${PORT}`);
  console.log(`IPFS API endpoint: ${IPFS_API}`);
});
