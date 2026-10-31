package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Embedder turns text into a fixed-dimension vector. Provider selection is
// via EMBEDDINGS (env) — abstracted behind this interface per the plan.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// NewEmbedder builds the embedder named by EMBEDDINGS.
//   - "http": OpenAI-compatible POST {url} with {model, input} (EMBEDDING_URL,
//     EMBEDDING_MODEL, EMBEDDING_API_KEY)
//   - "none": dev fallback returning a zero vector (writes succeed but
//     similarity search is degenerate — POC/dev only)
func NewEmbedder(provider, apiKey string, dim int) (Embedder, error) {
	switch provider {
	case "http":
		return NewHTTPEmbedder(EmbeddingConfigFromEnv(apiKey, dim))
	case "none", "":
		return ZeroEmbedder{Dim: dim}, nil
	default:
		return nil, fmt.Errorf("unknown EMBEDDINGS provider %q (supported: http, none)", provider)
	}
}

// ZeroEmbedder returns all-zero vectors (dev fallback).
type ZeroEmbedder struct{ Dim int }

// Embed returns a zero vector of dimension Dim.
func (z ZeroEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, z.Dim), nil
}

// HTTPEmbedder calls an OpenAI-compatible embeddings endpoint.
type HTTPEmbedder struct {
	cfg    EmbeddingConfig
	client *http.Client
}

// EmbeddingConfig configures HTTPEmbedder.
type EmbeddingConfig struct {
	URL     string
	Model   string
	APIKey  string
	Dim     int
	Timeout time.Duration
}

// EmbeddingConfigFromEnv assembles the http-embedder config.
func EmbeddingConfigFromEnv(apiKey string, dim int) EmbeddingConfig {
	return EmbeddingConfig{
		URL:     getenv("EMBEDDING_URL", "https://api.openai.com/v1/embeddings"),
		Model:   getenv("EMBEDDING_MODEL", "text-embedding-3-small"),
		APIKey:  apiKey,
		Dim:     dim,
		Timeout: 10 * time.Second,
	}
}

// NewHTTPEmbedder validates config and returns a ready embedder.
func NewHTTPEmbedder(cfg EmbeddingConfig) (*HTTPEmbedder, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("http embedder: EMBEDDING_URL is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("http embedder: EMBEDDING_MODEL is required")
	}
	return &HTTPEmbedder{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}, nil
}

// Embed POSTs one input string and returns the first embedding vector.
func (h *HTTPEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	payload, err := json.Marshal(map[string]any{"model": h.cfg.Model, "input": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.URL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.cfg.APIKey)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding provider %s: %s", resp.Status, truncate(string(body), 200))
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("embedding provider: bad JSON: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embedding provider: empty data")
	}
	vec := parsed.Data[0].Embedding
	if len(vec) != h.cfg.Dim {
		return nil, fmt.Errorf("embedding provider returned dim %d, expected %d (EMBEDDING_DIM)", len(vec), h.cfg.Dim)
	}
	return vec, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
