package modules

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func withGatewayState(t *testing.T, enabled bool, backend string) {
	t.Helper()
	prevEnabled := GatewayEnabled
	prevBackend := IPFSGateway
	GatewayEnabled = enabled
	if backend != "" {
		IPFSGateway = strings.TrimRight(backend, "/")
	}
	t.Cleanup(func() {
		GatewayEnabled = prevEnabled
		IPFSGateway = prevBackend
	})
}

func testRouter(enabled bool, backend string) http.Handler {
	mockUI := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("ui")},
	}
	mockExamples := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("examples")},
	}
	GatewayEnabled = enabled
	if backend != "" {
		IPFSGateway = strings.TrimRight(backend, "/")
	}
	return NewRouter(nil, nil, nil, mockUI, mockExamples)
}

func TestGatewayDisabledReturns404(t *testing.T) {
	withGatewayState(t, false, "http://127.0.0.1:9")
	router := testRouter(false, "http://127.0.0.1:9")

	req := httptest.NewRequest(http.MethodGet, "/ipfs/QmY7Yh4UquoXHLPFo2XbhXkhBvFoPwmQUSa92pxnxjQuPU", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when gateway is disabled, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("expected JSON error body: %v", err)
	}
	if payload["status"] != "disabled" {
		t.Errorf("expected status=disabled, got %#v", payload["status"])
	}
}

func TestGatewayProxiesIpfsAndIpns(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Ipfs-Path", r.URL.Path)
		w.Header().Set("Content-Type", "text/plain")
		if r.URL.Path == "/ipfs/QmTest" {
			io.WriteString(w, "hello-from-kubo")
			return
		}
		if r.URL.Path == "/ipns/example" {
			io.WriteString(w, "hello-ipns")
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(backend.Close)

	withGatewayState(t, true, backend.URL)
	router := testRouter(true, backend.URL)

	t.Run("GET /ipfs/cid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ipfs/QmTest", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "hello-from-kubo" {
			t.Errorf("unexpected body %q", rec.Body.String())
		}
		if rec.Header().Get("X-Ipfs-Path") != "/ipfs/QmTest" {
			t.Errorf("expected Kubo path header, got %q", rec.Header().Get("X-Ipfs-Path"))
		}
	})

	t.Run("GET /ipns/name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/ipns/example", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "hello-ipns" {
			t.Errorf("unexpected body %q", rec.Body.String())
		}
	})
}

func TestGatewayMetricPath(t *testing.T) {
	tests := map[string]string{
		"/ipfs/QmAbc/file.txt": "/ipfs",
		"/ipfs":                "/ipfs",
		"/ipns/name":           "/ipns",
		"/status":              "/status",
	}
	for in, want := range tests {
		if got := gatewayMetricPath(in); got != want {
			t.Errorf("gatewayMetricPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsGatewayPath(t *testing.T) {
	if !isGatewayPath("/ipfs/QmX") || !isGatewayPath("/ipns/foo") {
		t.Fatal("expected /ipfs and /ipns to be gateway paths")
	}
	if isGatewayPath("/status") || isGatewayPath("/upload") {
		t.Fatal("did not expect API paths to be treated as gateway paths")
	}
}
