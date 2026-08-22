package modules

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// NewGatewayProxy reverse-proxies path and subdomain-style IPFS requests to
// the local Kubo HTTP gateway (typically http://127.0.0.1:8080).
func NewGatewayProxy(target string) *httputil.ReverseProxy {
	backend, err := url.Parse(target)
	if err != nil {
		log.Printf("[gateway] invalid IPFS_GATEWAY %q: %v", target, err)
		return nil
	}

	proxy := httputil.NewSingleHostReverseProxy(backend)
	proxy.FlushInterval = 100 * time.Millisecond
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("[gateway] proxy error path=%s err=%v", r.URL.Path, err)
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":   "IPFS gateway unavailable",
			"details": err.Error(),
		})
	}
	return proxy
}

func (h *Handler) Gateway(w http.ResponseWriter, r *http.Request) {
	if !GatewayEnabled || h.gateway == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":  "IPFS gateway is disabled",
			"status": "disabled",
			"hint":   "Set ENABLE_GATEWAY=true (the default) to serve /ipfs and /ipns from this node",
		})
		return
	}
	h.gateway.ServeHTTP(w, r)
}

func gatewayMetricPath(path string) string {
	switch {
	case path == "/ipfs" || strings.HasPrefix(path, "/ipfs/"):
		return "/ipfs"
	case path == "/ipns" || strings.HasPrefix(path, "/ipns/"):
		return "/ipns"
	default:
		return path
	}
}

func isGatewayPath(path string) bool {
	return path == "/ipfs" || strings.HasPrefix(path, "/ipfs/") ||
		path == "/ipns" || strings.HasPrefix(path, "/ipns/")
}
