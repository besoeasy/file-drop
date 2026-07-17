package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/besoeasy/originless/internal/config"
	"github.com/besoeasy/originless/internal/handlers"
	"github.com/besoeasy/originless/internal/ipfs"
	"github.com/besoeasy/originless/internal/middleware"
)

func main() {
	if err := os.MkdirAll(config.UploadTempDir, 0o755); err != nil {
		log.Fatalf("failed to create upload temp directory: %v", err)
	}

	ipfsClient := ipfs.NewClient()
	handler := handlers.New(ipfsClient)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)

	publicDir := filepath.Join(projectRoot(), "public")
	mux.Handle("/", http.FileServer(http.Dir(publicDir)))

	root := middleware.Chain(
		mux,
		middleware.CORS,
		middleware.Gzip,
	)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("[STARTUP] SERVER_LISTENING host=%s port=%d url=http://%s:%d", config.Host, config.Port, config.Host, config.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("[SHUTDOWN] SIGNAL_RECEIVED signal=%s action=graceful_shutdown", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[SHUTDOWN] HTTP_SERVER_ERROR error=%v", err)
	} else {
		log.Printf("[SHUTDOWN] HTTP_SERVER_CLOSED")
	}
}


func projectRoot() string {
	if root := os.Getenv("ORIGINLESS_ROOT"); root != "" {
		return root
	}
	return "."
}