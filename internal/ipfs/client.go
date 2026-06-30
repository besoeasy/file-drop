package ipfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/besoeasy/originless/internal/config"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type HealthResult struct {
	Healthy bool
	Peers   int
	Error   string
}

type Stats struct {
	Bandwidth struct {
		TotalIn  int64  `json:"totalIn"`
		TotalOut int64  `json:"totalOut"`
		RateIn   int64  `json:"rateIn"`
		RateOut  int64  `json:"rateOut"`
		Interval string `json:"interval"`
	} `json:"bandwidth"`
	Repository struct {
		Size       int64  `json:"size"`
		StorageMax int64  `json:"storageMax"`
		NumObjects int64  `json:"numObjects"`
		Path       string `json:"path"`
		Version    string `json:"version"`
	} `json:"repository"`
	Node struct {
		ID              string `json:"id"`
		PublicKey       string `json:"publicKey"`
		AgentVersion    string `json:"agentVersion"`
		ProtocolVersion string `json:"protocolVersion"`
	} `json:"node"`
	Peers struct {
		Count int `json:"count"`
	} `json:"peers"`
}

type addResponse struct {
	Name string `json:"Name"`
	Hash string `json:"Hash"`
	Size string `json:"Size"`
}

func NewClient() *Client {
	return &Client{
		baseURL: config.IPFSAPI,
		httpClient: &http.Client{
			Timeout: 0,
		},
	}
}

func (c *Client) CheckHealth(ctx context.Context) HealthResult {
	peers, err := c.postJSON(ctx, "/api/v0/swarm/peers", 5*time.Second, nil)
	if err != nil {
		return HealthResult{Healthy: false, Error: err.Error()}
	}

	count := 0
	if rawPeers, ok := peers["Peers"].([]any); ok {
		count = len(rawPeers)
	}

	return HealthResult{
		Healthy: count >= 1,
		Peers:   count,
	}
}

func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	bw, err := c.postJSON(ctx, "/api/v0/stats/bw?interval=5m", 5*time.Second, nil)
	if err != nil {
		return nil, err
	}

	repo, err := c.postJSON(ctx, "/api/v0/repo/stat", 5*time.Second, nil)
	if err != nil {
		return nil, err
	}

	id, err := c.postJSON(ctx, "/api/v0/id", 5*time.Second, nil)
	if err != nil {
		return nil, err
	}

	peers, err := c.postJSON(ctx, "/api/v0/swarm/peers", 5*time.Second, nil)
	if err != nil {
		return nil, err
	}

	stats := &Stats{}
	stats.Bandwidth.TotalIn = jsonInt64(bw["TotalIn"])
	stats.Bandwidth.TotalOut = jsonInt64(bw["TotalOut"])
	stats.Bandwidth.RateIn = jsonInt64(bw["RateIn"])
	stats.Bandwidth.RateOut = jsonInt64(bw["RateOut"])
	stats.Bandwidth.Interval = "1h"

	stats.Repository.Size = jsonInt64(repo["RepoSize"])
	stats.Repository.StorageMax = jsonInt64(repo["StorageMax"])
	stats.Repository.NumObjects = jsonInt64(repo["NumObjects"])
	stats.Repository.Path = jsonString(repo["RepoPath"])
	stats.Repository.Version = jsonString(repo["Version"])

	stats.Node.ID = jsonString(id["ID"])
	stats.Node.PublicKey = jsonString(id["PublicKey"])
	stats.Node.AgentVersion = jsonString(id["AgentVersion"])
	stats.Node.ProtocolVersion = jsonString(id["ProtocolVersion"])

	if rawPeers, ok := peers["Peers"].([]any); ok {
		stats.Peers.Count = len(rawPeers)
	}

	return stats, nil
}

func (c *Client) AddFile(ctx context.Context, filePath, filename, contentType string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v0/add?pin=false", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.do(req, time.Hour)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	var result addResponse
	if err := json.NewDecoder(resp).Decode(&result); err != nil {
		return "", err
	}

	if result.Hash == "" {
		return "", fmt.Errorf("invalid IPFS response for file upload")
	}

	_ = contentType
	return result.Hash, nil
}

func (c *Client) AddDirectory(ctx context.Context, files map[string]string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for relativePath, absolutePath := range files {
		part, err := writer.CreateFormFile("file", relativePath)
		if err != nil {
			return "", err
		}

		file, err := os.Open(absolutePath)
		if err != nil {
			return "", err
		}

		if _, err := io.Copy(part, file); err != nil {
			file.Close()
			return "", err
		}
		file.Close()
	}

	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v0/add?pin=false&wrap-with-directory=true&recursive=true",
		body,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.do(req, time.Hour)
	if err != nil {
		return "", err
	}
	defer resp.Close()

	entries, err := parseAddResponses(resp)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return "", fmt.Errorf("invalid IPFS response for folder upload")
	}

	root := entries[len(entries)-1]
	if root.Hash == "" {
		return "", fmt.Errorf("invalid IPFS response for folder upload")
	}

	return root.Hash, nil
}

func (c *Client) postJSON(ctx context.Context, path string, timeout time.Duration, body io.Reader) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}

	reader, err := c.do(req, timeout)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var payload map[string]any
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *Client) do(req *http.Request, timeout time.Duration) (io.ReadCloser, error) {
	client := c.httpClient
	if timeout > 0 {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, resp.Status, strings.TrimSpace(string(body)))
	}

	return resp.Body, nil
}

func parseAddResponses(reader io.Reader) ([]addResponse, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	entries := make([]addResponse, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var entry addResponse
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func jsonInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		parsed, _ := strconvParseInt(typed)
		return parsed
	default:
		return 0
	}
}

func jsonString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func strconvParseInt(value string) (int64, error) {
	var parsed int64
	_, err := fmt.Sscan(value, &parsed)
	return parsed, err
}

func MimeType(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return "application/octet-stream"
	}
	if mime := mimeTypeByExtension(ext); mime != "" {
		return mime
	}
	return "application/octet-stream"
}

func mimeTypeByExtension(ext string) string {
	switch strings.ToLower(ext) {
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".txt":
		return "text/plain"
	case ".wasm":
		return "application/wasm"
	default:
		return ""
	}
}