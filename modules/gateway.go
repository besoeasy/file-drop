package modules

import (
	"log"
	"net"
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

func gatewayRequestLabel(r *http.Request) string {
	if isSubdomainGatewayHost(r.Host) {
		if strings.Contains(hostnameOnly(r.Host), ".ipns.") {
			return "/ipns"
		}
		return "/ipfs"
	}
	return gatewayMetricPath(r.URL.Path)
}

func isGatewayPath(path string) bool {
	return path == "/ipfs" || strings.HasPrefix(path, "/ipfs/") ||
		path == "/ipns" || strings.HasPrefix(path, "/ipns/")
}

func isGatewayRequest(r *http.Request) bool {
	return isGatewayPath(r.URL.Path) || isSubdomainGatewayHost(r.Host)
}

// isSubdomainGatewayHost reports Kubo origin-isolation hosts such as
// {cid}.ipfs.localhost:3232. Browsers resolve *.localhost to loopback, so
// those requests hit this process — they must go to Kubo, not the dashboard.
func isSubdomainGatewayHost(host string) bool {
	name := hostnameOnly(host)
	return strings.Contains(name, ".ipfs.") || strings.Contains(name, ".ipns.")
}

func hostnameOnly(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "[") {
		if end := strings.Index(host, "]"); end > 1 {
			return strings.ToLower(host[1:end])
		}
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		return strings.ToLower(h)
	}
	return strings.ToLower(host)
}
