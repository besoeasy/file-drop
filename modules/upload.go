package modules

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
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

type SavedFiles struct {
	Files map[string]string
	Count int
	Total int64
}

func SaveMultipartFile(r *http.Request, limit int64) (*SavedFile, error) {
	if err := os.MkdirAll(UploadTempDir, 0o755); err != nil {
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

		tmpFile, err := os.CreateTemp(UploadTempDir, "upload-*")
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
			return nil, fmt.Errorf("%w: exceeds the maximum allowed size of %s", ErrFileTooLarge, FormatBytes(limit))
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

func SaveMultipartFiles(r *http.Request, limit int64) (*SavedFiles, error) {
	if err := os.MkdirAll(UploadTempDir, 0o755); err != nil {
		return nil, err
	}

	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}

	files := make(map[string]string)
	var total int64

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			for _, p := range files {
				os.Remove(p)
			}
			return nil, err
		}

		if part.FormName() != "file" {
			part.Close()
			continue
		}

		relPath := part.FileName()
		if relPath == "" || relPath == "." {
			part.Close()
			continue
		}

		tmpFile, err := os.CreateTemp(UploadTempDir, "folder-*")
		if err != nil {
			part.Close()
			for _, p := range files {
				os.Remove(p)
			}
			return nil, err
		}

		written, err := io.Copy(tmpFile, io.LimitReader(part, limit-total+1))
		part.Close()
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			for _, p := range files {
				os.Remove(p)
			}
			return nil, err
		}

		total += written
		if total > limit {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			for _, p := range files {
				os.Remove(p)
			}
			return nil, fmt.Errorf("%w: exceeds the maximum allowed size of %s", ErrFileTooLarge, FormatBytes(limit))
		}

		if err := tmpFile.Close(); err != nil {
			os.Remove(tmpFile.Name())
			for _, p := range files {
				os.Remove(p)
			}
			return nil, err
		}

		files[filepath.ToSlash(relPath)] = tmpFile.Name()
	}

	if len(files) == 0 {
		return nil, ErrNoFile
	}

	return &SavedFiles{Files: files, Count: len(files), Total: total}, nil
}

func RemovePath(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete temp file: %v", err)
	}
}

func RemoveAll(paths map[string]string) {
	for _, p := range paths {
		RemovePath(p)
	}
}
