// Copyright 2026 A&R Visual Development Inc (arvisdev)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command hindsight-api is the M4 Hindsight R/W memory API (vm-hindsight, :8081).
//
// Endpoints (M4 install doc §1b):
//
//	GET  /healthz                      — liveness (always 200 while the process is up)
//	GET  /memory/read?task_id=&scope=&q=&k= — top-k relevant entries; exact-scope rows
//	                                        plus vector similarity over cross-task entries
//	POST /memory/write                 — {task_id, agent_ref, scope, body} → embed + insert
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	cfg := ConfigFromEnv()
	store, err := NewStore(cfg.DBDSN, cfg.EmbeddingDim)
	if err != nil {
		log.Fatalf("hindsight-api: open store: %v", err)
	}
	defer store.Close()

	embedder, err := NewEmbedder(cfg.EmbeddingProvider, cfg.EmbeddingAPIKey, cfg.EmbeddingDim)
	if err != nil {
		log.Fatalf("hindsight-api: embedder: %v", err)
	}

	srv := NewServer(store, embedder)
	log.Printf("hindsight-api: listening on %s (embeddings=%s)", cfg.Listen, cfg.EmbeddingProvider)
	if err := http.ListenAndServe(cfg.Listen, srv.Routes()); err != nil {
		log.Fatalf("hindsight-api: %v", err)
	}
}

// Config is the service configuration, sourced from the environment per the
// compose block in deploy/m4/hindsight/compose.yaml.
type Config struct {
	Listen            string // LISTEN, default 0.0.0.0:8081
	DBDSN             string // DB_DSN
	EmbeddingProvider string // EMBEDDINGS ("http" or "none")
	EmbeddingAPIKey   string // EMBEDDING_API_KEY
	EmbeddingURL      string // EMBEDDING_URL (http provider endpoint)
	EmbeddingModel    string // EMBEDDING_MODEL (http provider model id)
	EmbeddingDim      int    // EMBEDDING_DIM, default 1536
}

// ConfigFromEnv builds Config from the process environment.
func ConfigFromEnv() Config {
	c := Config{
		Listen:            getenv("LISTEN", "0.0.0.0:8081"),
		DBDSN:             os.Getenv("DB_DSN"),
		EmbeddingProvider: getenv("EMBEDDINGS", "none"),
		EmbeddingAPIKey:   os.Getenv("EMBEDDING_API_KEY"),
		EmbeddingURL:      os.Getenv("EMBEDDING_URL"),
		EmbeddingModel:    os.Getenv("EMBEDDING_MODEL"),
	}
	c.EmbeddingDim = atoiDefault(os.Getenv("EMBEDDING_DIM"), 1536)
	return c
}
