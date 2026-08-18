package modules

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics collects Prometheus-style metrics for the originless node.
// It is intentionally dependency-free: metrics are rendered directly in the
// Prometheus text exposition format (version 0.0.4).
type Metrics struct {
	mu       sync.Mutex
	requests map[string]*atomic.Int64

	errors     atomic.Int64
	uploads    atomic.Int64
	uploadSize atomic.Int64

	ipfsHealthy atomic.Int64
	peers       atomic.Int64
	pinnedCount atomic.Int64
	pinnedSize  atomic.Int64
	storageUsed atomic.Int64

	archiveSaved     atomic.Int64
	archiveSavedSize atomic.Int64
	archiveErrors    atomic.Int64
	archiveCount     atomic.Int64
	archiveSize      atomic.Int64
	archiveRepinned  atomic.Int64
	archiveRepinErrs atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[string]*atomic.Int64)}
}

// Middleware counts every HTTP request by URL path and flags 4xx/5xx responses
// as errors. It is the outermost middleware so it sees all traffic.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.IncRequest(r.URL.Path)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if rec.status >= http.StatusBadRequest {
			m.IncError()
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (m *Metrics) IncRequest(path string) {
	m.mu.Lock()
	c, ok := m.requests[path]
	if !ok {
		c = &atomic.Int64{}
		m.requests[path] = c
	}
	m.mu.Unlock()
	c.Add(1)
}

func (m *Metrics) IncError() { m.errors.Add(1) }

func (m *Metrics) IncUpload(size int64) {
	m.uploads.Add(1)
	m.uploadSize.Add(size)
}

func (m *Metrics) SetIPFS(healthy bool, peers int) {
	if healthy {
		m.ipfsHealthy.Store(1)
	} else {
		m.ipfsHealthy.Store(0)
	}
	m.peers.Store(int64(peers))
}

func (m *Metrics) SetPinned(count, size int64) {
	m.pinnedCount.Store(count)
	m.pinnedSize.Store(size)
}

func (m *Metrics) SetStorageUsed(size int64) { m.storageUsed.Store(size) }

func (m *Metrics) SetArchive(count, size int64) {
	m.archiveCount.Store(count)
	m.archiveSize.Store(size)
}

// Handler serves the /metrics endpoint in Prometheus text format. Gauges are
// refreshed on each scrape so they always reflect current state.
func (m *Metrics) Handler(janitor *Manager, ipfs *Client, archiver *Archiver) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if count, size, err := janitor.GetStats(); err == nil {
			m.SetPinned(count, size)
		}
		health := ipfs.CheckHealth(r.Context())
		m.SetIPFS(health.Healthy, health.Peers)
		if stats, err := ipfs.GetStats(r.Context()); err == nil {
			m.SetStorageUsed(stats.Repository.Size)
		}
		if archiver != nil {
			if count, size, err := archiver.GetStats(); err == nil {
				m.SetArchive(count, size)
			}
			m.archiveSaved.Store(archiver.saved.Load())
			m.archiveSavedSize.Store(archiver.savedSize.Load())
			m.archiveErrors.Store(archiver.errors.Load())
			m.archiveRepinned.Store(archiver.repinned.Load())
			m.archiveRepinErrs.Store(archiver.repinErrs.Load())
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		var sb strings.Builder
		sb.WriteString("# HELP originless_build_info Originless build information.\n")
		sb.WriteString("# TYPE originless_build_info gauge\n")
		fmt.Fprintf(&sb, "originless_build_info{version=%q} 1\n", AppVersion)

		sb.WriteString("# HELP originless_http_requests_total Total HTTP requests by path.\n")
		sb.WriteString("# TYPE originless_http_requests_total counter\n")
		m.mu.Lock()
		paths := make([]string, 0, len(m.requests))
		for p := range m.requests {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintf(&sb, "originless_http_requests_total{path=%q} %d\n", p, m.requests[p].Load())
		}
		m.mu.Unlock()

		sb.WriteString("# HELP originless_http_errors_total Total HTTP responses with status >= 400.\n")
		sb.WriteString("# TYPE originless_http_errors_total counter\n")
		fmt.Fprintf(&sb, "originless_http_errors_total %d\n", m.errors.Load())

		sb.WriteString("# HELP originless_uploads_total Total files uploaded.\n")
		sb.WriteString("# TYPE originless_uploads_total counter\n")
		fmt.Fprintf(&sb, "originless_uploads_total %d\n", m.uploads.Load())

		sb.WriteString("# HELP originless_upload_bytes_total Total bytes uploaded.\n")
		sb.WriteString("# TYPE originless_upload_bytes_total counter\n")
		fmt.Fprintf(&sb, "originless_upload_bytes_total %d\n", m.uploadSize.Load())

		sb.WriteString("# HELP originless_pinned_count Number of pinned uploads.\n")
		sb.WriteString("# TYPE originless_pinned_count gauge\n")
		fmt.Fprintf(&sb, "originless_pinned_count %d\n", m.pinnedCount.Load())

		sb.WriteString("# HELP originless_pinned_bytes Total bytes pinned.\n")
		sb.WriteString("# TYPE originless_pinned_bytes gauge\n")
		fmt.Fprintf(&sb, "originless_pinned_bytes %d\n", m.pinnedSize.Load())

		sb.WriteString("# HELP originless_storage_limit_bytes Configured storage limit.\n")
		sb.WriteString("# TYPE originless_storage_limit_bytes gauge\n")
		fmt.Fprintf(&sb, "originless_storage_limit_bytes %d\n", StorageMaxBytes)

		sb.WriteString("# HELP originless_storage_used_bytes Storage used by the IPFS repository.\n")
		sb.WriteString("# TYPE originless_storage_used_bytes gauge\n")
		fmt.Fprintf(&sb, "originless_storage_used_bytes %d\n", m.storageUsed.Load())

		sb.WriteString("# HELP originless_ipfs_healthy Whether the IPFS daemon is healthy (1) or not (0).\n")
		sb.WriteString("# TYPE originless_ipfs_healthy gauge\n")
		fmt.Fprintf(&sb, "originless_ipfs_healthy %d\n", m.ipfsHealthy.Load())

		sb.WriteString("# HELP originless_ipfs_peers Number of connected IPFS peers.\n")
		sb.WriteString("# TYPE originless_ipfs_peers gauge\n")
		fmt.Fprintf(&sb, "originless_ipfs_peers %d\n", m.peers.Load())

		sb.WriteString("# HELP originless_archive_count Number of permanently archived IPFS objects from Nostr.\n")
		sb.WriteString("# TYPE originless_archive_count gauge\n")
		fmt.Fprintf(&sb, "originless_archive_count %d\n", m.archiveCount.Load())

		sb.WriteString("# HELP originless_archive_bytes Total bytes in the permanent Nostr media archive.\n")
		sb.WriteString("# TYPE originless_archive_bytes gauge\n")
		fmt.Fprintf(&sb, "originless_archive_bytes %d\n", m.archiveSize.Load())

		sb.WriteString("# HELP originless_archive_saved_total IPFS objects saved to the archive this process.\n")
		sb.WriteString("# TYPE originless_archive_saved_total counter\n")
		fmt.Fprintf(&sb, "originless_archive_saved_total %d\n", m.archiveSaved.Load())

		sb.WriteString("# HELP originless_archive_saved_bytes_total Bytes saved to the archive this process.\n")
		sb.WriteString("# TYPE originless_archive_saved_bytes_total counter\n")
		fmt.Fprintf(&sb, "originless_archive_saved_bytes_total %d\n", m.archiveSavedSize.Load())

		sb.WriteString("# HELP originless_archive_errors_total Archive download errors this process.\n")
		sb.WriteString("# TYPE originless_archive_errors_total counter\n")
		fmt.Fprintf(&sb, "originless_archive_errors_total %d\n", m.archiveErrors.Load())

		sb.WriteString("# HELP originless_archive_repin_total Archive objects successfully re-pinned this process.\n")
		sb.WriteString("# TYPE originless_archive_repin_total counter\n")
		fmt.Fprintf(&sb, "originless_archive_repin_total %d\n", m.archiveRepinned.Load())

		sb.WriteString("# HELP originless_archive_repin_errors_total Archive re-pin failures this process.\n")
		sb.WriteString("# TYPE originless_archive_repin_errors_total counter\n")
		fmt.Fprintf(&sb, "originless_archive_repin_errors_total %d\n", m.archiveRepinErrs.Load())

		w.Write([]byte(sb.String()))
	}
}
