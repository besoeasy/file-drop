package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	ipfs      *Client
	janitor   *Manager
	metrics   *Metrics
	semaphore chan struct{}
}

func NewHandler(ipfsClient *Client, janitorManager *Manager, metrics *Metrics) *Handler {
	return &Handler{
		ipfs:      ipfsClient,
		janitor:   janitorManager,
		metrics:   metrics,
		semaphore: make(chan struct{}, MaxConcurrentOps),
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
			"configured": StorageMax,
			"current":    FormatBytes(stats.Repository.StorageMax),
		},
		"fileLimit": map[string]any{
			"configured": FormatBytes(FileLimit),
			"bytes":      FileLimit,
		},
		"appVersion": AppVersion,
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

	r.Body = http.MaxBytesReader(w, r.Body, FileLimit+1024*1024)

	saved, err := SaveMultipartFile(r, FileLimit)
	if err != nil {
		h.handleUploadError(w, err)
		return
	}
	defer RemovePath(saved.Path)

	mimeType := MimeType(saved.OriginalName)
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
	h.metrics.IncUpload(saved.Size)

	pinned := true
	if err := h.janitor.PinOnUpload(cid, saved.OriginalName, saved.Size); err != nil {
		log.Printf("Pin failed for %s: %v", cid, err)
		pinned = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"cid":      cid,
		"size":     saved.Size,
		"type":     mimeType,
		"filename": saved.OriginalName,
		"pinned":   pinned,
	})
}

func (h *Handler) UploadFolder(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, FileLimit+1024*1024)

	saved, err := SaveMultipartFiles(r, FileLimit)
	if err != nil {
		h.handleUploadError(w, err)
		return
	}
	defer RemoveAll(saved.Files)

	start := time.Now()
	log.Printf("Starting IPFS folder upload: %d files ...", saved.Count)

	cid, err := h.ipfs.AddDirectory(r.Context(), saved.Files)
	if err != nil {
		log.Printf("Folder upload error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload folder to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload folder to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	log.Printf("Folder uploaded successfully: files=%d size_bytes=%d cid=%s upload_duration_ms=%d",
		saved.Count, saved.Total, cid, time.Since(start).Milliseconds())
	h.metrics.IncUpload(saved.Total)

	pinned := true
	if err := h.janitor.PinOnUpload(cid, "folder", saved.Total); err != nil {
		log.Printf("Pin failed for folder %s: %v", cid, err)
		pinned = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"cid":    cid,
		"files":  saved.Count,
		"size":   saved.Total,
		"pinned": pinned,
	})
}

type PasteRequest struct {
	Content  string `json:"content"`
	Title    string `json:"title"`
	Language string `json:"language"`
}

func (h *Handler) Paste(w http.ResponseWriter, r *http.Request) {
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

	r.Body = http.MaxBytesReader(w, r.Body, PasteLimit+1024*1024)

	var req PasteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "Invalid JSON body",
			"status":    "error",
			"message":   "Expected JSON with a \"content\" field",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "Empty paste",
			"status":    "error",
			"message":   "Paste content cannot be empty",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if int64(len(content)) > PasteLimit {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":   "Paste too large",
			"message": "Paste exceeds the maximum allowed size",
			"maxSize": FormatBytes(PasteLimit),
		})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "paste-" + time.Now().Format("20060102-150405")
	}

	tmpFile, err := os.CreateTemp(UploadTempDir, "paste-*")
	if err != nil {
		log.Printf("Paste temp file error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "Failed to create temp file",
			"details": err.Error(),
		})
		return
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		log.Printf("Paste write error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "Failed to write paste",
			"details": err.Error(),
		})
		return
	}
	if err := tmpFile.Close(); err != nil {
		log.Printf("Paste close error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "Failed to write paste",
			"details": err.Error(),
		})
		return
	}

	start := time.Now()
	cid, err := h.ipfs.AddFile(r.Context(), tmpFile.Name(), title+".txt")
	if err != nil {
		log.Printf("Paste IPFS error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":     "Failed to upload to IPFS",
			"details":   err.Error(),
			"status":    "error",
			"message":   "Failed to upload to IPFS",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	log.Printf("Paste pinned: title=%s size_bytes=%d cid=%s upload_duration_ms=%d",
		title, len(content), cid, time.Since(start).Milliseconds())
	h.metrics.IncPaste()

	pinned := true
	if err := h.janitor.PinOnUpload(cid, title, int64(len(content))); err != nil {
		log.Printf("Pin failed for paste %s: %v", cid, err)
		pinned = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "success",
		"cid":      cid,
		"size":     len(content),
		"title":    title,
		"language": req.Language,
		"pinned":   pinned,
	})
}

func (h *Handler) GetPaste(w http.ResponseWriter, r *http.Request) {
	cid := r.PathValue("cid")
	if cid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": "Missing CID",
		})
		return
	}

	content, err := h.ipfs.Cat(r.Context(), cid, PasteLimit)
	if err != nil {
		log.Printf("Paste fetch error for %s: %v", cid, err)
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error":   "Failed to fetch paste",
			"details": err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(content)
}

func (h *Handler) handleUploadError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNoFile):
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":     "No file uploaded",
			"status":    "error",
			"message":   err.Error(),
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	case errors.Is(err, ErrFileTooLarge):
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"error":   "File too large",
			"message": err.Error(),
			"maxSize": FormatBytes(FileLimit),
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

func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	uploads, err := h.janitor.GetHistory(limit, offset)
	if err != nil {
		log.Printf("History error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":   "Failed to fetch history",
			"details": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "success",
		"uploads": uploads,
		"limit":   limit,
		"offset":  offset,
	})
}

func (h *Handler) PinStats(w http.ResponseWriter, r *http.Request) {
	count, size, err := h.janitor.GetStats()
	if err != nil {
		log.Printf("Pin stats error: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "Failed to fetch pin stats",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "success",
		"pinnedCount":   count,
		"pinnedSize":    size,
		"pinnedSizeStr": FormatBytes(size),
		"storageLimit":  StorageMax,
		"threshold":     PinThreshold,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}