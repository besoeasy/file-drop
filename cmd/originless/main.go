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

	"github.com/besoeasy/originless/internal/api"
	"github.com/besoeasy/originless/internal/config"
	"github.com/besoeasy/originless/internal/db"
	"github.com/besoeasy/originless/internal/ipfs"
	"github.com/besoeasy/originless/internal/janitor"
)

func main() {
	if err := os.MkdirAll(config.UploadTempDir, 0o755); err != nil {
		log.Fatalf("failed to create upload temp directory: %v", err)
	}

	dataDir := "/data"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "originless.db")
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	storageMaxBytes, err := config.ParseSize(config.StorageMax)
	if err != nil {
		log.Fatalf("invalid STORAGE_MAX: %v", err)
	}

	ipfsClient := ipfs.NewClient()
	janitorMgr := janitor.New(database, ipfsClient, storageMaxBytes)

	log.Printf("[STARTUP] running janitor reconciliation...")
	janitorMgr.Reconcile()

	janitorCtx, janitorCancel := context.WithCancel(context.Background())
	go janitorMgr.Run(janitorCtx, time.Duration(config.JanitorInterval)*time.Minute)

	router := api.NewRouter(ipfsClient, janitorMgr)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
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

	janitorCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[SHUTDOWN] HTTP_SERVER_ERROR error=%v", err)
	} else {
		log.Printf("[SHUTDOWN] HTTP_SERVER_CLOSED")
	}
}
