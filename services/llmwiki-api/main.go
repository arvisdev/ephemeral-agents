// Command llmwiki-api is the M4 LLM-Wiki retrieval + ingest service
// (vm-llm-wiki, :8080).
//
// Endpoints (M4 install doc §2b):
//
//	GET  /healthz        — liveness
//	GET  /wiki/query?q=&k= — top-k design-standard/skill passages (pgvector cosine)
//	POST /wiki/ingest    — one-shot reindex from CONTENT_REPO (clone → chunk → embed → upsert)
//
// Run as `llmwiki-api ingest` (argv[1] == "ingest") for the compose
// `llmwiki-ingest` one-shot job.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	cfg := ConfigFromEnv()

	embedder, err := NewEmbedder(cfg.EmbeddingProvider, cfg.EmbeddingAPIKey, cfg.EmbeddingDim)
	if err != nil {
		log.Fatalf("llmwiki-api: embedder: %v", err)
	}

	if len(os.Args) > 1 && os.Args[1] == "ingest" {
		runIngest(cfg, embedder) // exits non-zero on failure
		return
	}

	store, err := NewStore(cfg.DBDSN, cfg.EmbeddingDim)
	if err != nil {
		log.Fatalf("llmwiki-api: open store: %v", err)
	}
	defer store.Close()

	log.Printf("llmwiki-api: listening on %s (embeddings=%s)", cfg.Listen, cfg.EmbeddingProvider)
	if err := http.ListenAndServe(cfg.Listen, NewServer(store, embedder).Routes()); err != nil {
		log.Fatalf("llmwiki-api: %v", err)
	}
}

// Config is the service configuration per deploy/m4/llmwiki/compose.yaml.
type Config struct {
	Listen            string // LISTEN, default 0.0.0.0:8080
	DBDSN             string // DB_DSN
	ContentRepo       string // CONTENT_REPO (https clone URL with token)
	EmbeddingProvider string // EMBEDDINGS ("http" or "none")
	EmbeddingAPIKey   string // EMBEDDING_API_KEY
	EmbeddingURL      string // EMBEDDING_URL
	EmbeddingModel    string // EMBEDDING_MODEL
	EmbeddingDim      int    // EMBEDDING_DIM, default 1536
}

// ConfigFromEnv builds Config from the process environment.
func ConfigFromEnv() Config {
	c := Config{
		Listen:            getenv("LISTEN", "0.0.0.0:8080"),
		DBDSN:             getenv("DB_DSN", ""),
		ContentRepo:       getenv("CONTENT_REPO", ""),
		EmbeddingProvider: getenv("EMBEDDINGS", "none"),
		EmbeddingAPIKey:   os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingURL:      os.Getenv("EMBEDDING_URL"),
		EmbeddingModel:    os.Getenv("EMBEDDING_MODEL"),
	}
	c.EmbeddingDim = atoiDefault(os.Getenv("EMBEDDING_DIM"), 1536)
	return c
}
