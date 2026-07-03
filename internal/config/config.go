package config

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
)

const (
	IPFSAPI       = "http://127.0.0.1:5001"
	Port          = 3232
	Host          = "0.0.0.0"
	UploadTempDir = "/tmp/originless"
	AppVersion    = "0.1.34"
)

var (
	StorageMax string
	FileLimit  int64
)

var sizePattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB|TB)$`)

func init() {
	StorageMax = envOrDefault("STORAGE_MAX", "200GB")

	storageMaxBytes, err := ParseSize(StorageMax)
	if err != nil {
		panic(fmt.Sprintf("invalid STORAGE_MAX: %v", err))
	}

	FileLimit = storageMaxBytes / 100
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func ParseSize(sizeStr string) (int64, error) {
	match := sizePattern.FindStringSubmatch(sizeStr)
	if match == nil {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, err
	}

	var unit int64
	switch match[2] {
	case "B", "b":
		unit = 1
	case "KB", "Kb", "kb":
		unit = 1024
	case "MB", "Mb", "mb":
		unit = 1024 * 1024
	case "GB", "Gb", "gb":
		unit = 1024 * 1024 * 1024
	case "TB", "Tb", "tb":
		unit = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit: %s", match[2])
	}

	return int64(math.Floor(value * float64(unit))), nil
}

func FormatBytes(bytes int64) string {
	sizes := []string{"Bytes", "KB", "MB", "GB", "TB"}
	if bytes == 0 {
		return "0 Bytes"
	}

	exp := int(math.Floor(math.Log(float64(bytes)) / math.Log(1024)))
	if exp >= len(sizes) {
		exp = len(sizes) - 1
	}

	value := float64(bytes) / math.Pow(1024, float64(exp))
	return fmt.Sprintf("%.2f %s", value, sizes[exp])
}