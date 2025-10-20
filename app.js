const express = require("express");
const multer = require("multer");
const axios = require("axios");
const FormData = require("form-data");
const path = require("path");
const cors = require("cors");

// Constants
const IPFS_API = "http://127.0.0.1:5001";
const PORT = 3232;
const MAX_FILE_SIZE = parseInt(process.env.MAX_FILE_SIZE || 99000) * 1024 * 1024;
const STORAGE_MAX = process.env.STORAGE_MAX || "200GB";
const HOST = "0.0.0.0";
const SERVER_URL = process.env.SERVER_URL || `http://localhost:${PORT}`;

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

// NIP-96 server info endpoint
app.get("/.well-known/nostr/nip96.json", (req, res) => {
  const { version: appVersion } = require("./package.json");

  res.json({
    api_url: SERVER_URL,
    download_url: "https://dweb.link/ipfs",
    supported_nips: [96],
    tos_url: "https://github.com/besoeasy/file-drop",
    content_types: ["image/*", "video/*", "audio/*", "text/*", "application/*", "blob"],
    plans: {
      free: {
        name: "File Drop",
        is_nip98_required: false,
        url: `${SERVER_URL}/upload`,
        max_byte_size: MAX_FILE_SIZE,
        file_expiry: [2628000], // 30 days in seconds
      },
    },
  });
});

// Upload endpoint
app.post("/upload", upload.single("file"), async (req, res) => {
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

    // NIP-96 compatible response with your existing fields
    res.json({
      // Your existing fields
      status: "success",
      cid: response.data.Hash,
      filename: req.file.originalname,
      size: req.file.size,
      details: uploadDetails,

      // NIP-96 required fields
      message: "Upload successful",
      nip94_event: {
        tags: [
          ["url", `https://dweb.link/ipfs/${response.data.Hash}`],
          ["m", req.file.mimetype],
          ["x", response.data.Hash],
          ["size", req.file.size.toString()],
        ],
      },
    });
  } catch (err) {
    if (err instanceof multer.MulterError) {
      if (err.code === "LIMIT_FILE_SIZE") {
        return res.status(400).json({
          error: `File too large. Max size is ${MAX_FILE_SIZE / (1024 * 1024)}MB`,
          status: "error",
          message: `File too large. Max size is ${MAX_FILE_SIZE / (1024 * 1024)}MB`,
          timestamp: new Date().toISOString(),
        });
      }
      return res.status(400).json({
        error: err.message,
        status: "error",
        message: err.message,
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
      fileLimit: {
        bytes: MAX_FILE_SIZE,
        humanReadable: formatBytes(MAX_FILE_SIZE),
      },
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

// Apply error handler
app.use(errorHandler);

// Start server
app.listen(PORT, HOST, () => {
  console.log(`Server running on http://${HOST}:${PORT}`);
  console.log(`Public URL: ${SERVER_URL}`);
  console.log(`IPFS API endpoint: ${IPFS_API}`);
  console.log(`NIP-96 info: ${SERVER_URL}/.well-known/nostr/nip96.json`);
});
