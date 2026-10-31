package main

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// runIngest is the one-shot `llmwiki-api ingest` entrypoint used by the
// compose llmwiki-ingest job: clone CONTENT_REPO (depth 1) → chunk all
// markdown → embed → replace the index. Exits non-zero on failure.
func runIngest(cfg Config, embedder Embedder) {
	if _, err := IngestCLI(context.Background(), cfg, embedder); err != nil {
		log.Fatalf("llmwiki-ingest: %v", err)
	}
}

// IngestCLI runs the full reindex and returns stats (shared by the CLI and
// POST /wiki/ingest).
func IngestCLI(ctx context.Context, cfg Config, embedder Embedder) (IngestStats, error) {
	if cfg.DBDSN == "" {
		return IngestStats{}, fmt.Errorf("DB_DSN is empty")
	}
	if cfg.ContentRepo == "" {
		return IngestStats{}, fmt.Errorf("CONTENT_REPO is empty (token-auth HTTPS per install doc §2b)")
	}

	dir, err := os.MkdirTemp("", "llmwiki-content-")
	if err != nil {
		return IngestStats{}, err
	}
	defer os.RemoveAll(dir)

	if err := cloneRepo(ctx, cfg.ContentRepo, dir); err != nil {
		return IngestStats{}, fmt.Errorf("clone content repo: %w", err)
	}

	files, err := collectMarkdown(dir)
	if err != nil {
		return IngestStats{}, err
	}

	var chunks []Chunk
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return IngestStats{}, err
		}
		rel, _ := filepath.Rel(dir, f)
		chunks = append(chunks, ChunkMarkdown(rel, string(data))...)
	}

	store, err := NewStore(cfg.DBDSN, cfg.EmbeddingDim)
	if err != nil {
		return IngestStats{}, err
	}
	defer store.Close()

	if err := store.Reindex(ctx, chunks, embedder, func(done, total int) {
		log.Printf("llmwiki-ingest: embedded %d/%d", done, total)
	}); err != nil {
		return IngestStats{}, err
	}
	return IngestStats{Files: len(files), Chunks: len(chunks)}, nil
}

// cloneRepo runs `git clone --depth 1` via os/exec (token auth rides in the
// CONTENT_REPO URL per the install doc).
func cloneRepo(ctx context.Context, repo, dir string) error {
	return runGit(ctx, "clone", "--depth", "1", repo, dir)
}

func runGit(ctx context.Context, args ...string) error {
	return execGit(ctx, args...)
}

// collectMarkdown walks dir for .md files (skipping .git).
func collectMarkdown(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}
