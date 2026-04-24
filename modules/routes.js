// API route handlers (Node.js, axios-based)
const fs = require("fs");
const path = require("path");
const mime = require("mime-types");
const FormData = require("form-data");
const unzipper = require("unzipper");

const { IPFS_API, STORAGE_MAX, FILE_LIMIT, formatBytes, UPLOAD_TEMP_DIR } = require("./config");
const { axiosRequest } = require("./utils");
const { checkIPFSHealth, getIPFSStats } = require("./ipfs");
const { getGatewayUrl, refreshGateways } = require("./gateways");

const unlinkSafe = async (filePath, context) => {
  if (!filePath) return;

  try {
    await fs.promises.unlink(filePath);
  } catch (err) {
    if (err.code !== "ENOENT") {
      console.warn(`${context || "Failed to delete temp file"}: ${err.message}`);
    }
  }
};

const streamToFileWithLimit = (readableStream, filePath, sizeLimit) => new Promise((resolve, reject) => {
  if (!readableStream) {
    reject(new Error("Response has no body"));
    return;
  }

  const sink = fs.createWriteStream(filePath);
  let downloadedSize = 0;

  const onError = (err) => {
    sink.destroy();
    reject(err);
  };

  readableStream.on("data", (chunk) => {
    downloadedSize += chunk.length;
    if (downloadedSize > sizeLimit) {
      const error = new Error(`File size exceeds limit of ${formatBytes(sizeLimit)}`);
      error.code = "FILE_TOO_LARGE";
      readableStream.destroy(error);
    }
  });

  readableStream.on("error", onError);
  sink.on("error", onError);
  sink.on("finish", () => resolve(downloadedSize));

  readableStream.pipe(sink);
});

const parseIpfsAddResponse = (data) => {
  if (Array.isArray(data)) return data;
  if (data && typeof data === "object" && !Buffer.isBuffer(data)) return [data];

  let text = "";
  if (typeof data === "string") {
    text = data;
  } else if (Buffer.isBuffer(data)) {
    text = data.toString("utf8");
  }

  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      try {
        return JSON.parse(line);
      } catch {
        return null;
      }
    })
    .filter(Boolean);
};

const isSafeZipPath = (baseDir, entryPath) => {
  const safePath = path.resolve(baseDir, entryPath);
  const relative = path.relative(baseDir, safePath);
  return relative && !relative.startsWith("..") && !path.isAbsolute(relative);
};

const collectFilesRecursively = async (dirPath, collected = []) => {
  const entries = await fs.promises.readdir(dirPath, { withFileTypes: true });

  for (const entry of entries) {
    const entryPath = path.join(dirPath, entry.name);
    if (entry.isDirectory()) {
      await collectFilesRecursively(entryPath, collected);
    } else if (entry.isFile()) {
      collected.push(entryPath);
    }
  }

  return collected;
};

const removeDirSafe = async (dirPath, context) => {
  if (!dirPath) return;

  try {
    await fs.promises.rm(dirPath, { recursive: true, force: true });
  } catch (err) {
    console.warn(`${context || "Failed to delete temp directory"}: ${err.message}`);
  }
};

// Health check endpoint
const healthHandler = async (req, res) => {
  try {
    const { healthy, peers, error } = await checkIPFSHealth();

    if (healthy) {
      res.status(200).json({ status: "healthy", peers });
    } else {
      res.status(503).json({ status: "unhealthy", peers: peers || 0, reason: error || "No peers connected" });
    }
  } catch (err) {
    res.status(503).json({ status: "unhealthy", error: err.message });
  }
};

// Status endpoint
const statusHandler = async (req, res) => {
  try {
    const stats = await getIPFSStats();
    const { version: appVersion } = require("../package.json");

    res.json({
      status: "success",
      timestamp: new Date().toISOString(),
      bandwidth: stats.bandwidth,
      repository: stats.repository,
      node: stats.node,
      peers: stats.peers,
      storageLimit: {
        configured: STORAGE_MAX,
        current: formatBytes(stats.repository.storageMax),
      },
      fileLimit: {
        configured: process.env.FILE_LIMIT || "5GB",
        bytes: FILE_LIMIT,
        formatted: formatBytes(FILE_LIMIT),
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
};


// Upload handler
const uploadHandler = async (req, res) => {
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

    filePath = req.file.path;

    // Prepare file for IPFS using Node stream
    const formData = new FormData();

    // Detect correct MIME type from file extension
    const mimeType = mime.lookup(req.file.originalname) || req.file.mimetype || "application/octet-stream";

    formData.append("file", fs.createReadStream(filePath), {
      filename: req.file.originalname,
      contentType: mimeType,
    });

    // Upload to IPFS
    const uploadStart = Date.now();
    console.log(`Starting IPFS upload for ${req.file.originalname} ...`);

    const response = await axiosRequest({
      url: `${IPFS_API}/api/v0/add?pin=false`,
      method: "POST",
      data: formData,
      headers: formData.getHeaders(),
      maxContentLength: Infinity,
      maxBodyLength: Infinity,
    }, 3600000);

    const responseJson = response.data;

    // Detailed logging
    const uploadDetails = {
      name: req.file.originalname,
      size_bytes: req.file.size,
      mime_type: mimeType,
      cid: responseJson.Hash,
      upload_duration_ms: Date.now() - uploadStart,
      timestamp: new Date().toISOString(),
    };
    console.log("File uploaded successfully:", uploadDetails);

    // Clean up temp file after successful upload
    await unlinkSafe(filePath, "Failed to delete temp file");

    // Simple response
    res.json({
      status: "success",
      url: await getGatewayUrl(responseJson.Hash, req.file.originalname),
      cid: responseJson.Hash,
      size: uploadDetails.size_bytes,
      type: mimeType,
      filename: req.file.originalname,
    });
  } catch (err) {
    if (filePath) {
      await unlinkSafe(filePath, "Failed to delete temp file on error");
    }

    const multer = require("multer");
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

// Upload ZIP handler - extracts zip and uploads folder to IPFS
const uploadZipHandler = async (req, res) => {
  let zipPath = null;
  let extractDir = null;

  try {
    if (!req.file) {
      return res.status(400).json({
        error: "No file uploaded",
        status: "error",
        message: "No file uploaded",
        timestamp: new Date().toISOString(),
      });
    }

    const originalName = req.file.originalname || "archive.zip";
    const ext = path.extname(originalName).toLowerCase();

    if (ext !== ".zip") {
      return res.status(400).json({
        error: "Invalid file type",
        status: "error",
        message: "Only .zip archives are supported",
        timestamp: new Date().toISOString(),
      });
    }

    zipPath = req.file.path;
    extractDir = await fs.promises.mkdtemp(path.join(UPLOAD_TEMP_DIR, "zip-"));

    const directory = await unzipper.Open.file(zipPath);
    let extractedBytes = 0;
    let extractedFiles = 0;

    for (const entry of directory.files) {
      if (entry.type === "Directory") continue;

      const entryPath = String(entry.path || "").replace(/\\/g, "/").replace(/^\/+/, "");
      if (!entryPath) continue;

      if (!isSafeZipPath(extractDir, entryPath)) {
        throw new Error(`Invalid zip entry path: ${entryPath}`);
      }

      const targetPath = path.resolve(extractDir, entryPath);
      await fs.promises.mkdir(path.dirname(targetPath), { recursive: true });

      const written = await streamToFileWithLimit(entry.stream(), targetPath, FILE_LIMIT);
      extractedBytes += written;
      extractedFiles += 1;

      if (extractedBytes > FILE_LIMIT) {
        throw new Error(`Extracted size exceeds limit of ${formatBytes(FILE_LIMIT)}`);
      }
    }

    if (!extractedFiles) {
      return res.status(400).json({
        error: "Archive is empty",
        status: "error",
        message: "Zip archive contained no files",
        timestamp: new Date().toISOString(),
      });
    }

    const files = await collectFilesRecursively(extractDir);
    if (!files.length) {
      return res.status(400).json({
        error: "Archive is empty",
        status: "error",
        message: "Zip archive contained no files",
        timestamp: new Date().toISOString(),
      });
    }

    const formData = new FormData();
    files.forEach((filePath) => {
      const relativePath = path.relative(extractDir, filePath).split(path.sep).join("/");
      const mimeType = mime.lookup(filePath) || "application/octet-stream";
      formData.append("file", fs.createReadStream(filePath), {
        filename: relativePath,
        contentType: mimeType,
      });
    });

    const uploadStart = Date.now();
    console.log(`Starting IPFS folder upload for ${originalName} ...`);

    const response = await axiosRequest({
      url: `${IPFS_API}/api/v0/add?pin=false&wrap-with-directory=true&recursive=true`,
      method: "POST",
      data: formData,
      headers: formData.getHeaders(),
      maxContentLength: Infinity,
      maxBodyLength: Infinity,
      responseType: "text",
    }, 3600000);

    const entries = parseIpfsAddResponse(response.data);
    const rootEntry = entries[entries.length - 1];

    if (!rootEntry || !rootEntry.Hash) {
      throw new Error("Invalid IPFS response for folder upload");
    }

    const cid = rootEntry.Hash;
    const uploadDetails = {
      name: originalName,
      files: extractedFiles,
      size_bytes: extractedBytes,
      cid,
      upload_duration_ms: Date.now() - uploadStart,
      timestamp: new Date().toISOString(),
    };

    console.log("Folder uploaded successfully:", uploadDetails);

    res.json({
      status: "success",
      cid,
      url: await getGatewayUrl(cid),
      files: extractedFiles,
      size: extractedBytes,
      filename: originalName,
    });
  } catch (err) {
    console.error("ZIP upload error:", {
      message: err.message,
      stack: err.stack,
      timestamp: new Date().toISOString(),
    });

    res.status(500).json({
      error: "Failed to upload zip to IPFS",
      details: err.message,
      status: "error",
      message: "Failed to upload zip to IPFS",
      timestamp: new Date().toISOString(),
    });
  } finally {
    if (zipPath) {
      await unlinkSafe(zipPath, "Failed to delete temp zip file");
    }
    if (extractDir) {
      await removeDirSafe(extractDir, "Failed to delete extracted zip folder");
    }
  }
};

module.exports = {
  healthHandler,
  statusHandler,
  uploadHandler,
  uploadZipHandler,
};
