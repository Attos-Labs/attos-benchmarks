// Package cloud is the Attos Cloud path used by every benchmark: upload a dataset with an API key,
// let the cloud compile it, then download it with the official SDK Synchronizer.
// The only required setting is ATTOS_API_KEY.
package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	attos "github.com/Attos-Labs/attos-go"
)

// DefaultURL is the public Attos Cloud API. Override with ATTOS_API_URL (self-hosted or staging).
const DefaultURL = "https://api.attoscloud.com"

type Client struct {
	Key, BaseURL string
	HTTP         *http.Client
}

// FromEnv returns a client when ATTOS_API_KEY is set, otherwise nil.
func FromEnv() *Client {
	k := strings.TrimSpace(os.Getenv("ATTOS_API_KEY"))
	if k == "" {
		return nil
	}
	u := strings.TrimRight(os.Getenv("ATTOS_API_URL"), "/")
	if u == "" {
		u = DefaultURL
	}
	return &Client{Key: k, BaseURL: u, HTTP: &http.Client{Timeout: 30 * time.Minute}}
}

// Upload streams NDJSON ({"key":"..","value":".."} per line) to the compile endpoint and returns the dataset id.
func (c *Client) Upload(ctx context.Context, name string, ndjson io.Reader) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/api/v1/datasets/compile", ndjson)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-API-Key", c.Key)
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-Dataset-Name", name)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("cannot reach %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", errors.New("the API key was rejected (401): check ATTOS_API_KEY, create one under Settings > API Keys")
	case resp.StatusCode == http.StatusPaymentRequired:
		return "", fmt.Errorf("your plan cannot compile datasets (402): %s", strings.TrimSpace(string(body)))
	case resp.StatusCode == http.StatusTooManyRequests:
		return "", fmt.Errorf("monthly dataset-update limit reached (429): %s", strings.TrimSpace(string(body)))
	case resp.StatusCode/100 != 2:
		return "", fmt.Errorf("compile failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		ID, Status string
		RowCount   int64 `json:"row_count"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("unexpected compile response: %s", body)
	}
	return out.ID, nil
}

// Fetch downloads the compiled dataset with the SDK Synchronizer (no polling, no telemetry)
// into dir and returns the path of the cached .attos file.
func (c *Client) Fetch(ctx context.Context, id, dir string) (string, error) {
	cache := filepath.Join(dir, "sync-cache")
	s, err := attos.NewSynchronizer(ctx, id,
		attos.WithBaseURL(c.BaseURL), attos.WithAPIKey(c.Key), attos.WithCacheDir(cache),
		attos.WithSyncInterval(0), attos.WithTelemetryDisabled())
	if err != nil {
		return "", err
	}
	s.Close()
	p := filepath.Join(cache, id+".attos")
	if _, err := os.Stat(p); err != nil {
		return "", fmt.Errorf("sync finished but %s is missing", p)
	}
	return p, nil
}
