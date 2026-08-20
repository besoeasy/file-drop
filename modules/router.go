package modules

import (
	"io/fs"
	"net/http"
)

func NewRouter(ipfsClient *Client, janitorManager *Manager, archiver *Archiver, uiFS fs.FS, examplesFS fs.FS) http.Handler {
	metrics := NewMetrics()
	handler := NewHandler(ipfsClient, janitorManager, metrics, archiver, examplesFS)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)
	mux.HandleFunc("GET /history", handler.History)
	mux.HandleFunc("GET /pins", handler.PinStats)
	mux.HandleFunc("GET /archive", handler.ArchiveList)
	mux.HandleFunc("GET /metrics", metrics.Handler(janitorManager, ipfsClient, archiver))

	// Dynamic examples endpoint & routes
	mux.HandleFunc("GET /api/examples", handler.ExamplesList)
	mux.HandleFunc("GET /examples/manifest.json", handler.ExamplesList)

	// Direct serving of examples pages (live disk fallback when running locally)
	mux.HandleFunc("GET /examples", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/examples/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/examples/", func(w http.ResponseWriter, r *http.Request) {
		fsys := GetExamplesFS(examplesFS)
		http.StripPrefix("/examples/", http.FileServer(http.FS(fsys))).ServeHTTP(w, r)
	})

	// The Nostr archive page is only available when at least one npub is configured.
	// Pattern without a method so the guard covers GET, HEAD, and any other verb
	// (otherwise non-GET requests fall through to the "/" FileServer).
	mux.HandleFunc("/archive.html", func(w http.ResponseWriter, r *http.Request) {
		if len(NostrNpubs) == 0 {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		http.ServeFileFS(w, r, uiFS, "archive.html")
	})
	mux.HandleFunc("GET /library.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})

	mux.Handle("/", http.FileServer(http.FS(uiFS)))

	return Chain(
		metrics.Middleware(mux),
		CORS,
		Gzip,
	)
}
