const express = require("express");
const multer = require("multer");
const axios = require("axios");
const FormData = require("form-data");
const path = require("path");
const cors = require("cors");

const app = express();
const ipfsnode = "http://127.0.0.1:5001";

app.use(express.json());
app.use(express.urlencoded({ extended: true }));
app.use(cors());

app.use(express.static(path.join(__dirname, "public")));

const storage = multer.memoryStorage();
const upload = multer({
  storage,
  limits: { fileSize: 250 * 1024 * 1024 },
});

app.post(
  "/upload",
  (req, res, next) => {
    upload.single("file")(req, res, (err) => {
      if (err instanceof multer.MulterError && err.code === "LIMIT_FILE_SIZE") {
        console.error("File upload error: File too large");
        return res
          .status(400)
          .json({ error: "File too large. Max size is 100MB." });
      } else if (err) {
        console.error("File upload error:", err.message);
        return res.status(500).json({ error: err.message });
      }
      next();
    });
  },
  async (req, res) => {
    try {
      if (!req.file) return res.status(400).json({ error: "No file uploaded" });

      const formData = new FormData();
      formData.append("file", req.file.buffer, req.file.originalname);

      const response = await axios.post(ipfsnode + "/api/v0/add", formData, {
        headers: { ...formData.getHeaders() },
      });

      console.log({
        Name: req.file.originalname,
        Size: req.file.size,
        Type: req.file.mimetype,
        CID: response.data.Hash,
      });

      res.json({ cid: response.data.Hash });
    } catch (err) {
      console.error("Error uploading file to IPFS:", err.message);
      res.status(500).json({ error: err.message });
    }
  }
);

app.get("/status", async (req, res) => {
  try {
    const response_bw = await axios.post(
      ipfsnode + "/api/v0/stats/bw?interval=5m"
    );

    const response_repo = await axios.post(ipfsnode + "/api/v0/repo/stat");

    res.json({ bw: response_bw.data, repo: response_repo.data });
  } catch (err) {
    console.error("Error getting status from IPFS:", err.message);
    res.status(500).json({ error: err.message });
  }
});

const PORT = 3232;
app.listen(PORT, "0.0.0.0", () => {
  console.log(`Server running on http://0.0.0.0:${PORT}`);
});
