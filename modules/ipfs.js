// IPFS-related helper functions (Node.js, axios-based)
const { IPFS_API } = require("./config");
const { axiosRequest } = require("./utils");

const fetchJson = async (url, options = {}, timeoutMs = 10000) => {
  const res = await axiosRequest(
    {
      url,
      method: options.method || "GET",
      data: options.body,
      headers: options.headers,
    },
    timeoutMs
  );
  return res.data;
};

/**
 * Get the size of a CID
 * @param {string} cid - The CID to get size for
 * @returns {Promise<number>} - Size in bytes
 */
const getCidSize = async (cid) => {
  try {
    const statResponse = await fetchJson(
      `${IPFS_API}/api/v0/files/stat?arg=/ipfs/${encodeURIComponent(cid)}`,
      { method: "POST" },
      15000
    );
    return statResponse.CumulativeSize || statResponse.Size || 0;
  } catch (err) {
    // Try block/stat as fallback
    try {
      const blockResponse = await fetchJson(
        `${IPFS_API}/api/v0/block/stat?arg=${encodeURIComponent(cid)}`,
        { method: "POST" },
        15000
      );
      return blockResponse.Size || 0;
    } catch (blockErr) {
      return 0;
    }
  }
};

// Get total size of pinned content
const getPinnedSize = async () => {
  try {
    const pinResponse = await fetchJson(`${IPFS_API}/api/v0/pin/ls?type=recursive`, { method: "POST" }, 10000);
    const pins = pinResponse.Keys || {};
    const cids = Object.keys(pins);

    let totalSize = 0;
    for (const cid of cids) {
      const size = await getCidSize(cid);
      totalSize += size;
    }
    return { totalSize, count: cids.length };
  } catch (err) {
    console.error("Failed to get pinned size:", err.message);
    return { totalSize: 0, count: 0 };
  }
};

// Check IPFS health
const checkIPFSHealth = async () => {
  try {
    const peersResponse = await fetchJson(`${IPFS_API}/api/v0/swarm/peers`, { method: "POST" }, 5000);
    const peerCount = peersResponse.Peers?.length || 0;
    return { healthy: peerCount >= 1, peers: peerCount };
  } catch (err) {
    return { healthy: false, error: err.message };
  }
};

// Get comprehensive IPFS stats
const getIPFSStats = async () => {
  const [bwResponse, repoResponse, idResponse, peersResponse] = await Promise.all([
    fetchJson(`${IPFS_API}/api/v0/stats/bw?interval=5m`, { method: "POST" }, 5000),
    fetchJson(`${IPFS_API}/api/v0/repo/stat`, { method: "POST" }, 5000),
    fetchJson(`${IPFS_API}/api/v0/id`, { method: "POST" }, 5000),
    fetchJson(`${IPFS_API}/api/v0/swarm/peers`, { method: "POST" }, 5000),
  ]);

  return {
    bandwidth: {
      totalIn: bwResponse.TotalIn,
      totalOut: bwResponse.TotalOut,
      rateIn: bwResponse.RateIn,
      rateOut: bwResponse.RateOut,
      interval: "1h",
    },
    repository: {
      size: repoResponse.RepoSize,
      storageMax: repoResponse.StorageMax,
      numObjects: repoResponse.NumObjects,
      path: repoResponse.RepoPath,
      version: repoResponse.Version,
    },
    node: {
      id: idResponse.ID,
      publicKey: idResponse.PublicKey,
      agentVersion: idResponse.AgentVersion,
      protocolVersion: idResponse.ProtocolVersion,
    },
    peers: {
      count: peersResponse.Peers.length,
    },
  };
};

module.exports = {
  getCidSize,
  checkIPFSHealth,
  getIPFSStats,
};

