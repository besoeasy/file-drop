package upload

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/besoeasy/originless/internal/config"
)

var (
	ErrNoFile       = errors.New("no file uploaded")
	ErrFileTooLarge = errors.New("file too large")
)

type SavedFile struct {
	Path         string
	OriginalName string
	Size         int64
}

func SaveMultipartFile(r *http.Request, limit int64) (*SavedFile, error) {
	if err := os.MkdirAll(config.UploadTempDir, 0o755); err != nil {
		return nil, err
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		if part.FormName() != "file" {
			part.Close()
			continue
		}

		originalName := filepath.Base(part.FileName())
		if originalName == "" || originalName == "." {
			part.Close()
			return nil, ErrNoFile
		}

		tmpFile, err := os.CreateTemp(config.UploadTempDir, "upload-*")
		if err != nil {
			part.Close()
			return nil, err
		}

		written, err := io.Copy(tmpFile, io.LimitReader(part, limit+1))
		part.Close()
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return nil, err
		}

		if written > limit {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return nil, fmt.Errorf("%w: exceeds the maximum allowed size of %s", ErrFileTooLarge, config.FormatBytes(limit))
		}

		if err := tmpFile.Close(); err != nil {
			os.Remove(tmpFile.Name())
			return nil, err
		}

		return &SavedFile{
			Path:         tmpFile.Name(),
			OriginalName: originalName,
			Size:         written,
		}, nil
	}

	return nil, ErrNoFile
}

func RemovePath(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete temp file: %v", err)
	}
}
