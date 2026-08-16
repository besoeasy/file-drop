(() => {
  const LEGACY = {
    stats: "/",
    upload: "/publish.html",
    paste: "/publish.html#paste",
    files: "/library.html",
    archive: "/archive.html",
    api: "/docs.html",
  };

  const path = location.pathname;
  if (path === "/" || path.endsWith("/index.html")) {
    const key = location.hash.replace(/^#\/?/, "");
    if (LEGACY[key]) location.replace(LEGACY[key]);
  }

  const PAGES = [
    { id: "overview", href: "/", label: "Overview", icon: "fa-chart-line", group: "Look" },
    { id: "publish", href: "/publish.html", label: "Publish", icon: "fa-cloud-arrow-up", group: "Create" },
    { id: "library", href: "/library.html", label: "Library", icon: "fa-folder-open", group: "Keep" },
    { id: "archive", href: "/archive.html", label: "Archive", icon: "fa-box-archive", group: "Keep" },
    { id: "docs", href: "/docs.html", label: "Docs", icon: "fa-book-open", group: "Learn" },
  ];

  function formatBytes(bytes) {
    const sizes = ["Bytes", "KB", "MB", "GB", "TB"];
    if (!bytes) return "0 Bytes";
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(2) + " " + sizes[i];
  }

  function formatDate(dateStr) {
    if (!dateStr) return "";
    return new Date(dateStr).toLocaleString("en-US", {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
  }

  function formatUnix(value) {
    if (!value) return "never";
    if (typeof value === "number") {
      return formatDate(new Date(value * 1000).toISOString());
    }
    return formatDate(value);
  }

  function statusDefaults() {
    return {
      nodeId: "…",
      bandwidthIn: "0 B",
      bandwidthOut: "0 B",
      bandwidthRate: "0 B / 0 B",
      repoSize: "…",
      repoObjects: "…",
      version: "…",
      appver: "…",
      timestamp: "…",
      peerscount: 0,
      storageLimit: "Unknown",
      fileLimit: "Unknown",
      repoSizeBytes: 0,
      storageMaxBytes: 0,
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
      sizeStr: "0 Bytes",
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

  function applyStatus(vm, data) {
    vm.status = {
      nodeId: data.node.id.slice(0, 8) + "…" + data.node.id.slice(-8),
      bandwidthIn: formatBytes(data.bandwidth.totalIn),
      bandwidthOut: formatBytes(data.bandwidth.totalOut),
      bandwidthRate: `IN: ${formatBytes(data.bandwidth.rateIn)} / OUT: ${formatBytes(data.bandwidth.rateOut)}`,
      repoSize: `${formatBytes(data.repository.size)} / ${formatBytes(data.repository.storageMax)}`,
      repoObjects: `${data.repository.numObjects}`,
      version: `${data.node.agentVersion}`,
      appver: `${data.appVersion}`,
      timestamp: new Date(data.timestamp).toLocaleTimeString("en-US", { hour12: false }),
      peerscount: data.peers.count,
      storageLimit: data.storageLimit?.configured || "Unknown",
      fileLimit: data.fileLimit?.bytes ? formatBytes(data.fileLimit.bytes) : "Unknown",
      repoSizeBytes: data.repository.size || 0,
      storageMaxBytes: data.repository.storageMax || 0,
    };
    vm.archive = {
      enabled: !!(data.archive && data.archive.enabled),
      count: data.archive?.count || 0,
      size: data.archive?.size || 0,
      sizeStr: data.archive?.sizeStr || "0 Bytes",
      dir: data.archive?.dir || "/archive",
      scanMinutes: data.archive?.scanMinutes || 15,
      repinHours: data.archive?.repinHours || 6,
      scanning: !!(data.archive && data.archive.scanning),
      repinning: !!(data.archive && data.archive.repinning),
      lastScan: data.archive?.lastScan || "",
      lastRepin: data.archive?.lastRepin || "",
      accounts: data.archive?.accounts || (data.nostrNpubs || []).map((npub) => ({ npub })),
    };
    vm.nostrRelays = data.nostrRelays || [];
  }

  function navHTML(active) {
    const links = (page) =>
      `<a href="${page.href}" class="nav-link${page.id === active ? " is-active" : ""}">${page.label}</a>`;

    const desktop = PAGES.map(links).join("");
    const mobile = PAGES.map(
      (page) =>
        `<a href="${page.href}" class="${page.id === active ? "is-active" : ""}"><i class="fas ${page.icon}"></i>${page.label}</a>`
    ).join("");

    return `
      <header class="topbar">
        <div class="topbar-inner">
          <a class="brand" href="/">
            <span class="brand-mark" aria-hidden="true"></span>
            <span class="brand-name">originless</span>
          </a>
          <nav class="nav-groups">${desktop}</nav>
          <div class="top-actions">
            <div class="peers"><span class="dot"></span><span class="mono" data-ol-peers>{{ status.peerscount }} peers</span></div>
            <a class="btn btn-brand" href="/publish.html">Publish</a>
          </div>
        </div>
        <nav class="mobile-nav">${mobile}</nav>
      </header>`;
  }

  function footHTML() {
    return `
      <footer class="foot">
        <div class="foot-inner">
          <a href="https://github.com/besoeasy/Originless" target="_blank" rel="noopener">GitHub</a>
          <a href="/docs.html">Docs</a>
          <a href="/examples/">Examples</a>
          <span data-ol-ver>v{{ status.appver }}</span>
        </div>
      </footer>`;
  }

  function injectChrome(active, opts = {}) {
    const nav = document.querySelector("[data-ol-nav]");
    const foot = document.querySelector("[data-ol-foot]");
    const vue = opts.vue !== false;
    if (nav) nav.outerHTML = vue ? navHTML(active) : navHTML(active).replace("{{ status.peerscount }} peers", "— peers");
    if (foot) foot.outerHTML = vue ? footHTML() : footHTML().replace("v{{ status.appver }}", "v…");
  }

  function hydrateChrome() {
    fetch("/status")
      .then((r) => r.json())
      .then((data) => {
        if (data.status !== "success") return;
        const peers = document.querySelector("[data-ol-peers]");
        const ver = document.querySelector("[data-ol-ver]");
        if (peers) peers.textContent = (data.peers?.count || 0) + " peers";
        if (ver) ver.textContent = "v" + (data.appVersion || "");
      })
      .catch(() => {});
  }

  function pageMixin() {
    return {
      data() {
        return {
          copied: "",
          currentUrl: window.location.origin,
          gatewayBase: "https://dweb.link/ipfs/",
          history: [],
          archiveItems: [],
          nostrRelays: [],
          pinStats: pinDefaults(),
          status: statusDefaults(),
          archive: archiveDefaults(),
        };
      },
      methods: {
        formatBytes,
        formatDate,
        formatUnix,
        copyText(text, key) {
          navigator.clipboard.writeText(text).then(() => {
            this.copied = key;
            setTimeout(() => {
              this.copied = "";
            }, 2000);
          });
        },
        async fetchStatus() {
          try {
            const response = await fetch("/status");
            const data = await response.json();
            if (data.status === "success") applyStatus(this, data);
          } catch (error) {
            console.error(error);
          }
        },
        async fetchHistory() {
          try {
            const response = await fetch("/history?limit=20");
            const data = await response.json();
            if (data.status === "success" && data.uploads) this.history = data.uploads;
          } catch (error) {
            console.error(error);
          }
        },
        async fetchPinStats() {
          try {
            const response = await fetch("/pins");
            const data = await response.json();
            if (data.status === "success") {
              this.pinStats = {
                count: data.pinnedCount,
                size: data.pinnedSize,
                sizeStr: data.pinnedSizeStr,
                threshold: data.threshold,
              };
            }
          } catch (error) {
            console.error(error);
          }
        },
        async fetchArchive() {
          try {
            const response = await fetch("/archive?limit=20");
            const data = await response.json();
            if (data.status === "success" && data.items) this.archiveItems = data.items;
          } catch (error) {
            console.error(error);
          }
        },
        getGatewayUrlForCid(cid, filename) {
          return this.getIpfsUrlWithFilename(cid, filename, this.gatewayBase);
        },
        getIpfsUrlWithFilename(cid, filename, gateway = "") {
          if (!filename) return gateway ? `${gateway}${cid}` : `ipfs://${cid}`;
          const encodedFilename = encodeURIComponent(filename);
          if (gateway) return `${gateway}${cid}?filename=${encodedFilename}`;
          return `ipfs://${cid}?filename=${encodedFilename}`;
        },
      },
      mounted() {
        this.fetchStatus();
        this._olStatusTimer = setInterval(() => this.fetchStatus(), 10000);
        this._olPinsTimer = setInterval(() => this.fetchPinStats(), 30000);
      },
      beforeUnmount() {
        clearInterval(this._olStatusTimer);
        clearInterval(this._olPinsTimer);
      },
    };
  }

  const API_ENDPOINTS = [
    {
      method: "POST",
      path: "/upload",
      description: "Upload a single file. Streams directly to the node and pins it to IPFS.",
      curl: 'curl -X POST -F "file=@photo.png" http://localhost:3232/upload',
    },
    {
      method: "POST",
      path: "/uploadfolder",
      description: "Upload a folder / DApp build directory. Preserves relative paths on IPFS.",
      curl: 'curl -X POST -F "file=@dist/index.html;filename=index.html" -F "file=@dist/style.css;filename=style.css" http://localhost:3232/uploadfolder',
    },
    {
      method: "POST",
      path: "/paste",
      description: "Pin a text snippet. Accepts JSON with content and an optional title.",
      curl: `curl -X POST -H "Content-Type: application/json" -d '{"content":"Hello IPFS","title":"greeting"}' http://localhost:3232/paste`,
    },
    {
      method: "GET",
      path: "/paste/{cid}",
      description: "Fetch a pinned paste's raw text by CID.",
      curl: "curl http://localhost:3232/paste/QmX...",
    },
    {
      method: "GET",
      path: "/status",
      description: "Node telemetry: peers, bandwidth, repository usage, client version.",
      curl: "curl http://localhost:3232/status",
    },
    {
      method: "GET",
      path: "/pins",
      description: "Pin statistics: pinned count, total size, storage limit and eviction threshold.",
      curl: "curl http://localhost:3232/pins",
    },
    {
      method: "GET",
      path: "/history",
      description: "Upload history with CID, filename, size and pin status.",
      curl: "curl 'http://localhost:3232/history?limit=20'",
    },
    {
      method: "GET",
      path: "/archive",
      description: "List IPFS objects permanently archived from configured Nostr npubs.",
      curl: "curl http://localhost:3232/archive",
    },
    {
      method: "GET",
      path: "/health",
      description: "Liveness probe for orchestration and monitoring.",
      curl: "curl http://localhost:3232/health",
    },
  ];

  window.OL = {
    formatBytes,
    formatDate,
    formatUnix,
    injectChrome,
    hydrateChrome,
    pageMixin,
    API_ENDPOINTS,
  };
})();
