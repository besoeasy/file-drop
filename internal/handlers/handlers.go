package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strings"
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

func (h *Handler) UploadZip(w http.ResponseWriter, r *http.Request) {
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

	saved, err := upload.SaveMultipartFile(r, config.FileLimit)
	if err != nil {
		h.handleUploadError(w, err)
		return
	}
	defer upload.RemovePath(saved.Path)

	if strings.ToLower(filepath.Ext(saved.OriginalName)) != ".zip" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "Invalid file type",
			"status":    "error",
			"message":   "Only .zip archives are supported",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	archive, err := upload.ExtractZip(saved.Path, config.FileLimit)
	if err != nil {
		if strings.Contains(err.Error(), "zip archive contained no files") {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":     "Archive is empty",
				"status":    "error",
				"message":   "Zip archive contained no files",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		log.Printf("ZIP upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload zip to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload zip to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	defer upload.RemoveDir(archive.Dir)

	files, err := upload.CollectFiles(archive.Dir)
	if err != nil {
		if strings.Contains(err.Error(), "zip archive contained no files") {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error":     "Archive is empty",
				"status":    "error",
				"message":   "Zip archive contained no files",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		log.Printf("ZIP upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload zip to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload zip to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	start := time.Now()
	log.Printf("Starting IPFS folder upload for %s ...", saved.OriginalName)

	cid, err := h.ipfs.AddDirectory(r.Context(), files)
	if err != nil {
		log.Printf("ZIP upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload zip to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload zip to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	log.Printf("Folder uploaded successfully: name=%s files=%d size_bytes=%d cid=%s upload_duration_ms=%d",
		saved.OriginalName, archive.FileCount, archive.TotalBytes, cid, time.Since(start).Milliseconds())

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"cid":      cid,
		"files":    archive.FileCount,
		"size":     archive.TotalBytes,
		"filename": saved.OriginalName,
	})
}

func (h *Handler) handleUploadError(w http.ResponseWriter, err error) {
	if errors.Is(err, upload.ErrNoFile) {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "No file uploaded",
			"status":    "error",
			"message":   "No file uploaded",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if errors.Is(err, upload.ErrFileTooLarge) || strings.Contains(err.Error(), "file too large") {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":   "File too large",
			"message": err.Error(),
			"maxSize": config.FormatBytes(config.FileLimit),
		})
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]any{
		"error":     err.Error(),
		"status":    "error",
		"message":   err.Error(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}