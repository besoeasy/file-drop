package modules

import (
	"io/fs"
	"net/http"
)

func NewRouter(ipfsClient *Client, janitorManager *Manager, archiver *Archiver, uiFS fs.FS) http.Handler {
	metrics := NewMetrics()
	handler := NewHandler(ipfsClient, janitorManager, metrics, archiver)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)
	mux.HandleFunc("POST /paste", handler.Paste)
	mux.HandleFunc("GET /paste/{cid}", handler.GetPaste)
	mux.HandleFunc("GET /history", handler.History)
	mux.HandleFunc("GET /pins", handler.PinStats)
	mux.HandleFunc("GET /archive", handler.ArchiveList)
	mux.HandleFunc("GET /metrics", metrics.Handler(janitorManager, ipfsClient, archiver))
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
