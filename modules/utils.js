// Shared HTTP utilities
const axios = require("axios");

/**
 * Make an axios request and throw on non-2xx responses.
 * Supports any responseType; caller controls config entirely.
 */
const axiosRequest = async (config, timeoutMs = 10000) => {
  const res = await axios({
    timeout: timeoutMs,
    validateStatus: () => true,
    ...config,
  });

  if (res.status < 200 || res.status >= 300) {
    let text = "";
    if (typeof res.data === "string") {
      text = res.data;
    } else if (Buffer.isBuffer(res.data)) {
      text = res.data.toString("utf8");
    } else if (res.data && typeof res.data === "object") {
      try {
        text = JSON.stringify(res.data);
      } catch {
        text = "";
      }
    }

    const error = new Error(`HTTP ${res.status} ${res.statusText}${text ? `: ${text}` : ""}`);
    error.status = res.status;
    throw error;
  }

  return res;
};

/**
 * Make an axios request with responseType "stream" and throw on non-2xx responses.
 */
const axiosStream = async (config, timeoutMs = 10000) => {
  const res = await axios({
    responseType: "stream",
    timeout: timeoutMs,
    validateStatus: () => true,
    ...config,
  });

  if (res.status < 200 || res.status >= 300) {
    const error = new Error(`Remote server returned ${res.status}: ${res.statusText}`);
    error.status = res.status;
    if (res.data && res.data.destroy) {
      res.data.destroy();
    }
    throw error;
  }

  return res;
};

module.exports = { axiosRequest, axiosStream };
