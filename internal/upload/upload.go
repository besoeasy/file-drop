package upload

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/besoeasy/originless/internal/config"
)

var (
	ErrNoFile                = errors.New("no file uploaded")
	ErrFileTooLarge          = errors.New("file too large")
	ErrTooManyFiles          = errors.New("zip archive contains too many files")
	ErrSuspiciousCompression = errors.New("suspicious compression ratio")
	ErrExtractedSizeExceeded = errors.New("extracted size exceeds limit")
	ErrEmptyArchive          = errors.New("zip archive contained no files")
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

type ExtractedArchive struct {
	Dir        string
	FileCount  int
	TotalBytes int64
}

func ExtractZip(zipPath string, limit int64) (*ExtractedArchive, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	extractDir, err := os.MkdirTemp(config.UploadTempDir, "zip-*")
	if err != nil {
		return nil, err
	}

	result := &ExtractedArchive{Dir: extractDir}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(extractDir)
		}
	}()

	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() {
			continue
		}

		entryPath := strings.TrimPrefix(strings.ReplaceAll(entry.Name, "\\", "/"), "/")
		if entryPath == "" {
			continue
		}

		if !isSafeZipPath(extractDir, entryPath) {
			return nil, fmt.Errorf("invalid zip entry path: %s", entryPath)
		}

		if result.FileCount >= config.MaxZipFiles {
			return nil, fmt.Errorf("%w (max %d)", ErrTooManyFiles, config.MaxZipFiles)
		}

		if entry.CompressedSize64 > 0 && entry.UncompressedSize64/entry.CompressedSize64 > config.MaxCompression {
			return nil, fmt.Errorf("%w for %s (%d:1)", ErrSuspiciousCompression, entryPath, entry.UncompressedSize64/entry.CompressedSize64)
		}

		if result.TotalBytes >= limit || entry.UncompressedSize64 > uint64(limit-result.TotalBytes) {
			return nil, fmt.Errorf("%w of %s", ErrExtractedSizeExceeded, config.FormatBytes(limit))
		}

		targetPath := filepath.Join(extractDir, entryPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return nil, err
		}

		source, err := entry.Open()
		if err != nil {
			return nil, err
		}

		destination, err := os.Create(targetPath)
		if err != nil {
			source.Close()
			return nil, err
		}

		written, err := io.Copy(destination, source)
		source.Close()
		destination.Close()
		if err != nil {
			return nil, err
		}

		result.TotalBytes += written
		result.FileCount++
	}

	if result.FileCount == 0 {
		return nil, ErrEmptyArchive
	}

	cleanup = false
	return result, nil
}

func CollectFiles(root string) (map[string]string, error) {
	files := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		files[filepath.ToSlash(relative)] = path
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(files) == 0 {
		return nil, ErrEmptyArchive
	}

	return files, nil
}

func RemovePath(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Failed to delete temp file: %v\n", err)
	}
}

func RemoveDir(path string) {
	if path == "" {
		return
	}
	if err := os.RemoveAll(path); err != nil {
		fmt.Printf("Failed to delete temp directory: %v\n", err)
	}
}

func isSafeZipPath(baseDir, entryPath string) bool {
	target := filepath.Clean(filepath.Join(baseDir, entryPath))
	relative, err := filepath.Rel(baseDir, target)
	if err != nil {
		return false
	}
	return relative != "" && !strings.HasPrefix(relative, "..") && !filepath.IsAbs(relative)
}