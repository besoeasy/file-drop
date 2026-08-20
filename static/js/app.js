(() => {
  // Built-in public & local IPFS gateways
  const GATEWAYS = [
    { label: "dweb.link (Protocol Labs)", url: "https://dweb.link/ipfs/" },
    { label: "ipfs.io (Official)", url: "https://ipfs.io/ipfs/" },
  ];

  function formatBytes(bytes) {
    if (!bytes || bytes === 0) return "0 B";
    const k = 1024;
    const sizes = ["B", "KB", "MB", "GB", "TB", "PB"];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    if (i <= 0) return bytes + " B";
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
  }

  function formatDate(dateStr) {
    if (!dateStr) return "";
    const d = new Date(dateStr);
    return d.toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }

  function formatUnix(value) {
    if (!value) return "Never";
    if (typeof value === "number") {
      return formatDate(new Date(value * 1000).toISOString());
    }
    return formatDate(value);
  }

  function formatRelative(dateStr) {
    if (!dateStr) return "";
    const now = new Date();
    const date = new Date(dateStr);
    const diffSec = Math.floor((now - date) / 1000);
    if (diffSec < 45) return "just now";
    const diffMin = Math.floor(diffSec / 60);
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHour = Math.floor(diffMin / 60);
    if (diffHour < 24) return `${diffHour}h ago`;
    const diffDay = Math.floor(diffHour / 24);
    if (diffDay < 30) return `${diffDay}d ago`;
    return formatDate(dateStr);
  }

  function getFileCategory(filename = "", mime = "") {
    const fn = (filename || "").toLowerCase();
    if (mime.startsWith("image/") || /\.(jpg|jpeg|png|gif|webp|svg|bmp|ico|avif)$/.test(fn)) {
      return "image";
    }
    if (mime.startsWith("video/") || /\.(mp4|webm|mkv|mov|avi|m4v)$/.test(fn)) {
      return "video";
    }
    if (mime.startsWith("audio/") || /\.(mp3|wav|ogg|flac|m4a|aac)$/.test(fn)) {
      return "audio";
    }
    if (/\.(zip|tar|gz|7z|rar|bz2)$/.test(fn) || fn === "folder") {
      return "archive";
    }
    if (/\.(html|htm|js|ts|jsx|tsx|css|json|go|py|rs|c|cpp|md|sh|yml|yaml|sql|wasm)$/.test(fn)) {
      return "code";
    }
    return "file";
  }

  function statusDefaults() {
    return {
      nodeId: "...",
      fullNodeId: "",
      bandwidthIn: "0 B",
      bandwidthOut: "0 B",
      bandwidthRate: "0 B / 0 B",
      bandwidthRateIn: "0 B",
      bandwidthRateOut: "0 B",
      repoSize: "...",
      repoObjects: "0",
      version: "...",
      appver: "...",
      timestamp: "...",
      peerscount: 0,
      storageLimit: "Unknown",
      fileLimit: "Unknown",
      repoSizeBytes: 0,
      storageMaxBytes: 0,
      isHealthy: true,
    };
  }

  function pinDefaults() {
    return { count: 0, size: 0, sizeStr: "0 B", threshold: 75 };
  }

  function archiveDefaults() {
    return {
      enabled: false,
      count: 0,
      size: 0,
      sizeStr: "0 B",
      dir: "/archive",
      scanMinutes: 15,
      repinHours: 6,
      scanning: false,
      repinning: false,
      lastScan: "",
      lastRepin: "",
      accounts: [],
    };
  }

  function createOriginlessApp(options = {}) {
    const { createApp } = Vue;

    return createApp({
      data() {
        const savedGateway = localStorage.getItem("ol_gateway_url") || GATEWAYS[0].url;
        return {
          activePage: options.page || "overview",
          
          gateways: GATEWAYS,
          currentGateway: savedGateway,

          status: statusDefaults(),
          pinStats: pinDefaults(),
          archive: archiveDefaults(),
          nostrRelays: [],
          
          history: [],
          archiveItems: [],
          searchQuery: "",
          statusFilter: "all", 
          typeFilter: "all",
          sortBy: "date-desc",
          
          activeTab: "upload", // upload, folder, prompt
          
          // Single File Upload
          dragOver: false,
          isUploading: false,
          uploadProgress: 0,
          uploadSpeedStr: "",
          currentUploadFile: null,
          lastUploadResult: null,

          // Folder Upload
          folderFilesCount: 0,
          folderTotalSize: 0,
          isUploadingFolder: false,
          lastFolderResult: null,

          // Agent Prompt Config
          promptFormat: "plain", // plain, markdown, curl, python
          
          // Content Inspection Modal
          inspectModalOpen: false,
          inspectItem: null,
          inspectQrUrl: "",

          // Toast Alerts
          toasts: [],
        };
      },

      computed: {
        currentOrigin() {
          return window.location.origin;
        },

        storagePercentage() {
          if (!this.status.storageMaxBytes || this.status.storageMaxBytes === 0) return 0;
          const pct = (this.status.repoSizeBytes / this.status.storageMaxBytes) * 100;
          return Math.min(100, Math.max(0, Math.round(pct * 10) / 10));
        },

        storageGaugeClass() {
          if (this.storagePercentage >= 90) return "is-danger";
          if (this.storagePercentage >= 75) return "is-warn";
          return "";
        },

        filteredHistory() {
          let list = [...(this.history || [])];
          
          // Search query filter
          if (this.searchQuery.trim()) {
            const q = this.searchQuery.toLowerCase().trim();
            list = list.filter((item) => 
              (item.filename || "").toLowerCase().includes(q) ||
              (item.cid || "").toLowerCase().includes(q)
            );
          }

          // Status filter
          if (this.statusFilter === "pinned") {
            list = list.filter(item => !item.unpinned);
          } else if (this.statusFilter === "unpinned") {
            list = list.filter(item => item.unpinned);
          }

          // Type filter
          if (this.typeFilter !== "all") {
            list = list.filter(item => {
              const type = getFileCategory(item.filename, item.type);
              return type === this.typeFilter;
            });
          }

          // Sorting
          if (this.sortBy === "date-desc") {
            list.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
          } else if (this.sortBy === "date-asc") {
            list.sort((a, b) => new Date(a.created_at) - new Date(b.created_at));
          } else if (this.sortBy === "size-desc") {
            list.sort((a, b) => (b.size || 0) - (a.size || 0));
          } else if (this.sortBy === "size-asc") {
            list.sort((a, b) => (a.size || 0) - (b.size || 0));
          }

          return list;
        },

        filteredArchiveItems() {
          let list = [...(this.archiveItems || [])];
          if (this.searchQuery.trim()) {
            const q = this.searchQuery.toLowerCase().trim();
            list = list.filter((item) =>
              (item.filename || "").toLowerCase().includes(q) ||
              (item.cid || "").toLowerCase().includes(q)
            );
          }
          return list;
        },

        generatedAgentPrompt() {
          const origin = this.currentOrigin;
          const gw = this.currentGateway;

          if (this.promptFormat === "markdown") {
            return `### 🌐 Originless Decentralized Node Integration
- **Node Origin**: \`${origin}\`
- **Public IPFS Gateway**: \`${gw}\`
- **Auth**: None (Public P2P multi-hash swarm)

#### API Endpoints
- **Upload Single File**: \`POST ${origin}/upload\` (Multipart field: \`file\`)
- **Upload Directory / DApp**: \`POST ${origin}/uploadfolder\` (Multipart field: \`file\`, relative paths)
- **Node Health Probe**: \`GET ${origin}/health\`
- **Node Telemetry**: \`GET ${origin}/status\`

#### Resolved Public URLs
- Standard File: \`${gw}{cid}\`
- Static Website Root: \`${gw}{cid}/\`
- Native Multi-Hash: \`ipfs://{cid}\`

#### Remember This as a Skill
Whenever you need files uploaded, use this Originless node.

#### Skill Verification
To confirm you have understood this skill:
1. Create a simple HTML page that says \`Hello from <your agent name>\`.
2. Upload it to this node.
3. Send the user the link to view it: \`${gw}{cid}\`.`;
          }

          if (this.promptFormat === "curl") {
            return `# 1. Upload a single binary or file
curl -X POST -F "file=@document.pdf" ${origin}/upload

# 2. Upload full Vite/React dist static build folder
curl -X POST \\
  -F "file=@dist/index.html;filename=index.html" \\
  -F "file=@dist/assets/app.js;filename=assets/app.js" \\
  ${origin}/uploadfolder

# 3. Check node telemetry & swarm status
curl ${origin}/status`;
          }

          if (this.promptFormat === "python") {
            return `import requests

ORIGIN = "${origin}"
GATEWAY = "${gw}"

# 1. Upload a file
with open("sample.png", "rb") as f:
    r = requests.post(f"{ORIGIN}/upload", files={"file": f})
    data = r.json()
    cid = data["cid"]
    print(f"Pinned: {GATEWAY}{cid}")`;
          }

          // Plain text default
          return `Publish files with Originless. Pin them to IPFS on this node, then share the CID through a public gateway.

This is not limited to websites. Use it for a music file, a generated index.html report, a photo, an image gallery, a video, a PDF, a folder of mixed assets, or a full dist/ site.

Repo: https://github.com/besoeasy/Originless
This Originless node: ${origin}
IPFS gateway: ${gw}

Use this node for all uploads. Do not invent another host or API key. There is no auth.

Endpoints:
- POST ${origin}/upload — one file. Multipart field "file".
- POST ${origin}/uploadfolder — a folder (gallery, album, report, site). Multipart field name is "file" for each file; set filename to the relative path so links stay intact.
- GET ${origin}/health — liveness check.

The JSON response includes "cid". That CID is the content address. This node's HTTP origin does not serve the bytes.

After upload, return gateway URLs to the user:
- ${gw}{cid}            (single file)
- ${gw}{cid}/           (folder; index.html at the root is served automatically)
- ipfs://{cid}

Keep relative paths inside folders. Never wrap the CID in ${origin}/upload or a local file path.

Remember this as a skill: whenever you need files uploaded, use this Originless node.

To confirm you have understood this skill, complete this check:
1. Create a simple HTML page that says "Hello from <your agent name>".
2. Upload it to this node.
3. Send the user the link to view it: ${gw}{cid}.`;
        },
      },

      methods: {
        formatBytes,
        formatDate,
        formatUnix,
        formatRelative,
        getFileCategory,

        showToast(message, type = "success") {
          const id = Date.now() + Math.random();
          this.toasts.push({ id, message, type });
          setTimeout(() => {
            this.toasts = this.toasts.filter(t => t.id !== id);
          }, 3500);
        },

        async copyText(text, label = "Copied to clipboard") {
          if (!text) return;
          try {
            await navigator.clipboard.writeText(text);
            this.showToast(label, "success");
          } catch (e) {
            this.showToast("Failed to copy", "error");
          }
        },

        onGatewayChange() {
          localStorage.setItem("ol_gateway_url", this.currentGateway);
          const gwName = new URL(this.currentGateway).hostname;
          this.showToast(`Active Gateway set to ${gwName}`, "success");
        },

        getGatewayUrl(cid, filename) {
          if (!cid) return "";
          if (!filename) return `${this.currentGateway}${cid}`;
          return `${this.currentGateway}${cid}?filename=${encodeURIComponent(filename)}`;
        },

        getIpfsUrl(cid, filename) {
          if (!cid) return "";
          if (!filename) return `ipfs://${cid}`;
          return `ipfs://${cid}?filename=${encodeURIComponent(filename)}`;
        },

        async fetchStatus() {
          try {
            const res = await fetch("/status");
            const data = await res.json();
            if (data.status === "success") {
              const fullId = data.node?.id || "";
              const shortId = fullId ? `${fullId.slice(0, 8)}...${fullId.slice(-8)}` : "...";
              
              this.status = {
                nodeId: shortId,
                fullNodeId: fullId,
                bandwidthIn: formatBytes(data.bandwidth?.totalIn || 0),
                bandwidthOut: formatBytes(data.bandwidth?.totalOut || 0),
                bandwidthRateIn: formatBytes(data.bandwidth?.rateIn || 0) + "/s",
                bandwidthRateOut: formatBytes(data.bandwidth?.rateOut || 0) + "/s",
                bandwidthRate: `↓ ${formatBytes(data.bandwidth?.rateIn || 0)}/s  ↑ ${formatBytes(data.bandwidth?.rateOut || 0)}/s`,
                repoSize: `${formatBytes(data.repository?.size || 0)} / ${formatBytes(data.repository?.storageMax || 0)}`,
                repoObjects: `${data.repository?.numObjects || 0}`,
                version: data.node?.agentVersion || "IPFS Kubo",
                appver: data.appVersion || "1.0",
                timestamp: new Date(data.timestamp).toLocaleTimeString("en-US", { hour12: false }),
                peerscount: data.peers?.count || 0,
                storageLimit: data.storageLimit?.configured || "Unknown",
                fileLimit: data.fileLimit?.bytes ? formatBytes(data.fileLimit.bytes) : "Unknown",
                repoSizeBytes: data.repository?.size || 0,
                storageMaxBytes: data.repository?.storageMax || 0,
                isHealthy: true,
              };

              if (data.archive) {
                this.archive = {
                  enabled: !!data.archive.enabled,
                  count: data.archive.count || 0,
                  size: data.archive.size || 0,
                  sizeStr: data.archive.sizeStr || "0 B",
                  dir: data.archive.dir || "/archive",
                  scanMinutes: data.archive.scanMinutes || 15,
                  repinHours: data.archive.repinHours || 6,
                  scanning: !!data.archive.scanning,
                  repinning: !!data.archive.repinning,
                  lastScan: data.archive.lastScan || "",
                  lastRepin: data.archive.lastRepin || "",
                  accounts: data.archive.accounts || (data.nostrNpubs || []).map((npub) => ({ npub })),
                };
              }
              this.nostrRelays = data.nostrRelays || [];
            }
          } catch (err) {
            console.error("Fetch status error:", err);
            this.status.isHealthy = false;
          }
        },

        async fetchHistory() {
          try {
            const res = await fetch("/history?limit=100");
            const data = await res.json();
            if (data.status === "success" && data.uploads) {
              this.history = data.uploads;
            }
          } catch (err) {
            console.error("Fetch history error:", err);
          }
        },

        async fetchPinStats() {
          try {
            const res = await fetch("/pins");
            const data = await res.json();
            if (data.status === "success") {
              this.pinStats = {
                count: data.pinnedCount,
                size: data.pinnedSize,
                sizeStr: data.pinnedSizeStr,
                threshold: data.threshold,
              };
            }
          } catch (err) {
            console.error("Fetch pin stats error:", err);
          }
        },

        async fetchArchive() {
          try {
            const res = await fetch("/archive?limit=100");
            const data = await res.json();
            if (data.status === "success" && data.items) {
              this.archiveItems = data.items;
            }
          } catch (err) {
            console.error("Fetch archive error:", err);
          }
        },

        // Single File Upload
        triggerFileInput() {
          this.$refs.fileInput.click();
        },

        handleFileSelect(event) {
          const files = event.target.files;
          if (files && files.length > 0) {
            this.uploadSingleFile(files[0]);
          }
        },

        handleDrop(event) {
          this.dragOver = false;
          const files = event.dataTransfer.files;
          if (files && files.length > 0) {
            this.uploadSingleFile(files[0]);
          }
        },

        async uploadSingleFile(file) {
          this.currentUploadFile = file;
          this.isUploading = true;
          this.uploadProgress = 15;
          this.lastUploadResult = null;

          const formData = new FormData();
          formData.append("file", file, file.name);

          try {
            this.uploadProgress = 50;
            const res = await fetch("/upload", {
              method: "POST",
              body: formData,
            });

            this.uploadProgress = 90;
            const data = await res.json();

            if (res.ok && data.status === "success") {
              this.uploadProgress = 100;
              this.lastUploadResult = data;
              this.showToast(`Pinned "${data.filename}" to Swarm!`, "success");
              this.fetchHistory();
              this.fetchPinStats();
              this.fetchStatus();
            } else {
              throw new Error(data.message || data.error || "Upload failed");
            }
          } catch (err) {
            console.error("Upload error:", err);
            this.showToast(err.message, "error");
          } finally {
            this.isUploading = false;
            if (this.$refs.fileInput) {
              this.$refs.fileInput.value = "";
            }
          }
        },

        // Folder Upload
        triggerFolderInput() {
          this.$refs.folderInput.click();
        },

        handleFolderSelect(event) {
          const files = event.target.files;
          if (files && files.length > 0) {
            this.uploadFolder(files);
          }
        },

        async uploadFolder(files) {
          this.isUploadingFolder = true;
          this.folderFilesCount = files.length;
          let totalBytes = 0;
          const formData = new FormData();

          for (let i = 0; i < files.length; i++) {
            const f = files[i];
            totalBytes += f.size;
            const relativePath = f.webkitRelativePath || f.name;
            formData.append("file", f, relativePath);
          }
          this.folderTotalSize = totalBytes;
          this.lastFolderResult = null;

          try {
            const res = await fetch("/uploadfolder", {
              method: "POST",
              body: formData,
            });
            const data = await res.json();

            if (res.ok && data.status === "success") {
              this.lastFolderResult = data;
              this.showToast(`Folder pinned (${data.files} files)!`, "success");
              this.fetchHistory();
              this.fetchPinStats();
              this.fetchStatus();
            } else {
              throw new Error(data.message || data.error || "Folder upload failed");
            }
          } catch (err) {
            console.error("Folder upload error:", err);
            this.showToast(err.message, "error");
          } finally {
            this.isUploadingFolder = false;
            if (this.$refs.folderInput) {
              this.$refs.folderInput.value = "";
            }
          }
        },

        // Inspection & QR Modal
        openInspect(item) {
          this.inspectItem = item;
          const url = this.getGatewayUrl(item.cid, item.filename);
          // Standard high-res QR code link for instant mobile sharing
          this.inspectQrUrl = `https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=${encodeURIComponent(url)}`;
          this.inspectModalOpen = true;
        },

        closeInspect() {
          this.inspectModalOpen = false;
          this.inspectItem = null;
        },
      },

      mounted() {
        this.fetchStatus();
        this.fetchPinStats();
        if (this.activePage === "overview") {
          this.fetchHistory();
        } else if (this.activePage === "archive") {
          this.fetchArchive();
        }

        // Live polling
        this._statusTimer = setInterval(() => this.fetchStatus(), 8000);
        this._pinsTimer = setInterval(() => this.fetchPinStats(), 25000);
      },

      beforeUnmount() {
        clearInterval(this._statusTimer);
        clearInterval(this._pinsTimer);
      },
    });
  }

  window.Originless = {
    GATEWAYS,
    formatBytes,
    formatDate,
    formatUnix,
    formatRelative,
    getFileCategory,
    createOriginlessApp,
  };
})();
