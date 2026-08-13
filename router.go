package main

import (
	"net/http"
)

func NewRouter(ipfsClient *Client, janitorManager *Manager) http.Handler {
	handler := NewHandler(ipfsClient, janitorManager)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)
	mux.HandleFunc("GET /history", handler.History)
	mux.HandleFunc("GET /pins", handler.PinStats)

	// Serve embedded web UI assets directly
	staticFS := GetFS()
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	return Chain(
		mux,
		CORS,
		Gzip,
	)
}