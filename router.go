package main

import (
	"net/http"
)

func NewRouter(ipfsClient *Client, janitorManager *Manager) http.Handler {
	metrics := NewMetrics()
	handler := NewHandler(ipfsClient, janitorManager, metrics)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)
	mux.HandleFunc("POST /paste", handler.Paste)
	mux.HandleFunc("GET /paste/{cid}", handler.GetPaste)
	mux.HandleFunc("GET /history", handler.History)
	mux.HandleFunc("GET /pins", handler.PinStats)
	mux.HandleFunc("GET /metrics", metrics.Handler(janitorManager, ipfsClient))

	// Serve embedded web UI assets directly
	staticFS := GetFS()
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return Chain(
		metrics.Middleware(mux),
		CORS,
		Gzip,
	)
}