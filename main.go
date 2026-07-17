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
	"github.com/besoeasy/originless/internal/pin"
	"github.com/besoeasy/originless/internal/store"
)

func main() {
	if err := os.MkdirAll(config.UploadTempDir, 0o755); err != nil {
		log.Fatalf("failed to create upload temp directory: %v", err)
	}

	dataDir := envOrDefault("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "originless.db")
	db, err := store.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	storageMaxBytes, err := config.ParseSize(config.StorageMax)
	if err != nil {
		log.Fatalf("invalid STORAGE_MAX: %v", err)
	}

	ipfsClient := ipfs.NewClient()
	pinMgr := pin.New(db, ipfsClient, storageMaxBytes)

	log.Printf("[STARTUP] running reconciliation...")
	pinMgr.Reconcile()

	go pinMgr.Run(time.Duration(config.JanitorInterval) * time.Minute)

	handler := handlers.New(ipfsClient, pinMgr)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /status", handler.Status)
	mux.HandleFunc("POST /upload", handler.Upload)
	mux.HandleFunc("POST /uploadfolder", handler.UploadFolder)
	mux.HandleFunc("GET /history", handler.History)
	mux.HandleFunc("GET /pins", handler.PinStats)

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

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func projectRoot() string {
	if root := os.Getenv("ORIGINLESS_ROOT"); root != "" {
		return root
	}
	return "."
}