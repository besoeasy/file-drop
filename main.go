package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if err := os.MkdirAll(UploadTempDir, 0o755); err != nil {
		log.Fatalf("failed to create upload temp directory: %v", err)
	}

	dataDir := "/data"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "originless.db")
	database, err := NewStore(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	storageMaxBytes, err := ParseSize(StorageMax)
	if err != nil {
		log.Fatalf("invalid STORAGE_MAX: %v", err)
	}

	ipfsClient := NewClient()
	janitorMgr := NewJanitor(database, ipfsClient, storageMaxBytes)

	log.Printf("[STARTUP] running janitor reconciliation...")
	janitorMgr.Reconcile()

	janitorCtx, janitorCancel := context.WithCancel(context.Background())
	go janitorMgr.Run(janitorCtx, time.Duration(JanitorInterval)*time.Minute)

	if len(NostrNpubs) > 0 {
		log.Printf("[STARTUP] NOSTR_NPUBS configured count=%d keys=%v", len(NostrNpubs), NostrNpubs)
		for _, npub := range NostrNpubs {
			if !IsValidNpub(npub) {
				log.Printf("[STARTUP] WARNING: invalid Nostr npub key format: %q", npub)
			}
		}
		log.Printf("[STARTUP] NOSTR_RELAYS configured count=%d relays=%v", len(NostrRelays), NostrRelays)
	}

	router := NewRouter(ipfsClient, janitorMgr)

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%d", Host, Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("[STARTUP] SERVER_LISTENING host=%s port=%d url=http://%s:%d", Host, Port, Host, Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	sig := <-stop
	log.Printf("[SHUTDOWN] SIGNAL_RECEIVED signal=%s action=graceful_shutdown", sig)

	// Forward the signal to the IPFS daemon so it can shut down cleanly
	// (flush datastore, release locks) instead of being killed when the
	// container tears down.
	forwardSignalToIPFS(sig)

	janitorCancel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[SHUTDOWN] HTTP_SERVER_ERROR error=%v", err)
	} else {
		log.Printf("[SHUTDOWN] HTTP_SERVER_CLOSED")
	}

	// Give the IPFS daemon a moment to finish before we exit.
	waitForIPFSExit(5 * time.Second)
}

// forwardSignalToIPFS forwards the received termination signal to the IPFS
// daemon (started by the entrypoint with IPFS_PID exported) so it can shut
// down cleanly instead of being killed when the container tears down.
func forwardSignalToIPFS(sig os.Signal) {
	pid := ipfsDaemonPID()
	if pid == 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		log.Printf("[SHUTDOWN] failed to signal IPFS daemon (pid %d): %v", pid, err)
		return
	}
	log.Printf("[SHUTDOWN] forwarded %s to IPFS daemon (pid %d)", sig, pid)
}

// waitForIPFSExit waits up to timeout for the IPFS daemon to exit after being
// signaled, so its datastore is flushed before the container stops.
func waitForIPFSExit(timeout time.Duration) {
	pid := ipfsDaemonPID()
	if pid == 0 {
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !ipfsProcessAlive(pid) {
			log.Printf("[SHUTDOWN] IPFS daemon (pid %d) exited", pid)
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	log.Printf("[SHUTDOWN] IPFS daemon (pid %d) did not exit within %s", pid, timeout)
}

// ipfsProcessAlive reports whether the process is still running. A zombie
// (state 'Z') counts as exited: it has finished its shutdown work and is only
// waiting to be reaped by its parent.
func ipfsProcessAlive(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil {
		return false // process is gone
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	// Format: "pid (comm) state ..." — the state is the char after the last ')'
	// (comm may itself contain spaces/parentheses).
	if idx := strings.LastIndexByte(string(data), ')'); idx >= 0 && idx+2 < len(data) {
		if data[idx+2] == 'Z' {
			return false // zombie — effectively exited
		}
	}
	return true
}

// ipfsDaemonPID returns the PID of the IPFS daemon from the IPFS_PID
// environment variable, or 0 if unset or invalid.
func ipfsDaemonPID() int {
	pidStr := os.Getenv("IPFS_PID")
	if pidStr == "" {
		return 0
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}