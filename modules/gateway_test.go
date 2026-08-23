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

func TestGatewayDoesNotDuplicateKuboCORS(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", "GET")
		w.Header().Add("Access-Control-Allow-Methods", "HEAD")
		w.Header().Add("Access-Control-Allow-Methods", "OPTIONS")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Add("Access-Control-Allow-Headers", "Range")
		w.Header().Set("X-Ipfs-Path", r.URL.Path)
		io.WriteString(w, "ok")
	}))
	t.Cleanup(backend.Close)

	withGatewayState(t, true, backend.URL)
	router := testRouter(true, backend.URL)

	req := httptest.NewRequest(http.MethodGet, "/ipfs/QmTest", nil)
	req.Header.Set("Origin", "https://gupt.app")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	origins := rec.Header().Values("Access-Control-Allow-Origin")
	if len(origins) != 1 || origins[0] != "*" {
		t.Fatalf("expected a single CORS origin *, got %#v", origins)
	}
	if got := rec.Header().Values("Access-Control-Allow-Headers"); len(got) < 2 {
		t.Fatalf("expected Kubo's Allow-Headers to pass through, got %#v", got)
	}
	for _, m := range rec.Header().Values("Access-Control-Allow-Methods") {
		if strings.Contains(m, "POST") {
			t.Fatalf("Originless CORS leaked onto the gateway: %#v", rec.Header().Values("Access-Control-Allow-Methods"))
		}
	}
}

func TestAPIHasSingleOriginlessCORS(t *testing.T) {
	withGatewayState(t, true, "http://127.0.0.1:9")
	router := testRouter(true, "http://127.0.0.1:9")

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://gupt.app")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	origins := rec.Header().Values("Access-Control-Allow-Origin")
	if len(origins) != 1 || origins[0] != "*" {
		t.Fatalf("expected a single CORS origin *, got %#v", origins)
	}
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

func TestIsSubdomainGatewayHost(t *testing.T) {
	yes := []string{
		"bafybeichqkffkyfqetlaucpshgkel2wtwm57twvyvp6sp6ok45cnggxu24.ipfs.localhost:3232",
		"bafybeiabc.ipfs.localhost",
		"k51qzi.ipns.localhost:3232",
		"QmY7Yh4UquoXHLPFo2XbhXkhBvFoPwmQUSa92pxnxjQuPU.ipfs.127.0.0.1:8080",
	}
	for _, host := range yes {
		if !isSubdomainGatewayHost(host) {
			t.Errorf("expected subdomain gateway host %q", host)
		}
	}
	no := []string{
		"",
		"localhost:3232",
		"localhost",
		"127.0.0.1:3232",
		"ipfs.localhost:3232",
		"example.com",
	}
	for _, host := range no {
		if isSubdomainGatewayHost(host) {
			t.Errorf("did not expect subdomain gateway host %q", host)
		}
	}
}

func TestSubdomainGatewayDoesNotServeDashboard(t *testing.T) {
	var sawHost string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawHost = r.Host
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, "pinned-site")
	}))
	t.Cleanup(backend.Close)

	withGatewayState(t, true, backend.URL)
	router := testRouter(true, backend.URL)

	req := httptest.NewRequest(http.MethodGet, "http://bafybeichqkffkyfqetlaucpshgkel2wtwm57twvyvp6sp6ok45cnggxu24.ipfs.localhost:3232/?filename=QmT9o78zKwGNR97T33LhsdhtS2a3c6cDbguEbnbGwKAZBC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "ui" {
		t.Fatal("subdomain host served the dashboard instead of Kubo")
	}
	if rec.Body.String() != "pinned-site" {
		t.Errorf("unexpected body %q", rec.Body.String())
	}
	if origins := rec.Header().Values("Access-Control-Allow-Origin"); len(origins) > 1 {
		t.Fatalf("duplicate CORS origin on subdomain gateway: %#v", origins)
	}
	if !strings.Contains(sawHost, ".ipfs.localhost") {
		t.Errorf("Kubo should see the subdomain Host, got %q", sawHost)
	}
}

func TestGatewayStatusReportsServing(t *testing.T) {
	withGatewayState(t, true, "http://127.0.0.1:8080")
	req := httptest.NewRequest(http.MethodGet, "http://localhost:3232/status", nil)
	got := gatewayStatus(req)
	if got["enabled"] != true || got["serving"] != true {
		t.Fatalf("expected enabled/serving true, got %#v", got)
	}
	if got["url"] != "http://localhost:3232/ipfs/{cid}" {
		t.Errorf("url = %#v", got["url"])
	}
	if got["ipnsUrl"] != "http://localhost:3232/ipns/{name}" {
		t.Errorf("ipnsUrl = %#v", got["ipnsUrl"])
	}
	if got["kubo"] != "http://127.0.0.1:8080/ipfs/{cid}" {
		t.Errorf("kubo = %#v", got["kubo"])
	}

	withGatewayState(t, false, "http://127.0.0.1:8080")
	off := gatewayStatus(req)
	if off["enabled"] != false || off["serving"] != false {
		t.Fatalf("expected enabled/serving false, got %#v", off)
	}
	if _, ok := off["url"]; ok {
		t.Errorf("disabled status should omit url, got %#v", off)
	}
}

func TestSubdomainGatewayDisabledReturns404(t *testing.T) {
	withGatewayState(t, false, "http://127.0.0.1:9")
	router := testRouter(false, "http://127.0.0.1:9")

	req := httptest.NewRequest(http.MethodGet, "http://bafybeiabc.ipfs.localhost:3232/", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when gateway is disabled, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() == "ui" {
		t.Fatal("disabled gateway must not fall through to the dashboard")
	}
}
