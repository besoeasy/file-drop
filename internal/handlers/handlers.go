package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/besoeasy/originless/internal/config"
	"github.com/besoeasy/originless/internal/ipfs"
	"github.com/besoeasy/originless/internal/upload"
)

type Handler struct {
	ipfs      *ipfs.Client
	semaphore chan struct{}
}

func New(ipfsClient *ipfs.Client) *Handler {
	return &Handler{
		ipfs:      ipfsClient,
		semaphore: make(chan struct{}, config.MaxConcurrentOps),
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	result := h.ipfs.CheckHealth(r.Context())

	if result.Healthy {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "healthy",
			"peers":  result.Peers,
		})
		return
	}

	reason := result.Error
	if reason == "" {
		reason = "No peers connected"
	}

	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"status": "unhealthy",
		"peers":  result.Peers,
		"reason": reason,
	})
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	stats, err := h.ipfs.GetStats(r.Context())
	if err != nil {
		log.Printf("Status check error: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":     "Failed to retrieve IPFS status",
			"details":   err.Error(),
			"status":    "failed",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "success",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"bandwidth": stats.Bandwidth,
		"repository": stats.Repository,
		"node":      stats.Node,
		"peers":     stats.Peers,
		"storageLimit": map[string]any{
			"configured": config.StorageMax,
			"current":    config.FormatBytes(stats.Repository.StorageMax),
		},
		"fileLimit": map[string]any{
			"configured": config.FormatBytes(config.FileLimit),
			"bytes":      config.FileLimit,
		},
		"appVersion": config.AppVersion,
	})
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	select {
	case h.semaphore <- struct{}{}:
		defer func() { <-h.semaphore }()
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error":     "Server busy",
			"status":    "error",
			"message":   "Too many concurrent uploads, try again later",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, config.FileLimit+1024*1024)

	saved, err := upload.SaveMultipartFile(r, config.FileLimit)
	if err != nil {
		h.handleUploadError(w, err)
		return
	}
	defer upload.RemovePath(saved.Path)

	mimeType := ipfs.MimeType(saved.OriginalName)
	start := time.Now()
	log.Printf("Starting IPFS upload for %s ...", saved.OriginalName)

	cid, err := h.ipfs.AddFile(r.Context(), saved.Path, saved.OriginalName)
	if err != nil {
		log.Printf("IPFS upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	log.Printf("File uploaded successfully: name=%s size_bytes=%d mime_type=%s cid=%s upload_duration_ms=%d",
		saved.OriginalName, saved.Size, mimeType, cid, time.Since(start).Milliseconds())

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"cid":      cid,
		"size":     saved.Size,
		"type":     mimeType,
		"filename": saved.OriginalName,
	})
}

func (h *Handler) handleUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, upload.ErrNoFile):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "No file uploaded",
			"status":    "error",
			"message":   err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	case errors.Is(err, upload.ErrFileTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":   "File too large",
			"message": err.Error(),
			"maxSize": config.FormatBytes(config.FileLimit),
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     err.Error(),
			"status":    "error",
			"message":   err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}