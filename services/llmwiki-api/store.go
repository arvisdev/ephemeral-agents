package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// Store is the LLM-Wiki pgvector store (table wiki_chunks; created by
// deploy/m4/llmwiki/initdb.sql — CREATE EXTENSION vector, table with
// source, section, text, embedding vector(dim)).
type Store struct {
	db  *sql.DB
	dim int
}

// NewStore opens the DB pool.
func NewStore(dsn string, dim int) (*Store, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DB_DSN is empty")
	}
	if dim <= 0 {
		return nil, fmt.Errorf("embedding dim must be positive, got %d", dim)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	return &Store{db: db, dim: dim}, nil
}

// Close releases the pool.
func (s *Store) Close() error { return s.db.Close() }

// Passage is one retrieved chunk as returned by /wiki/query.
type Passage struct {
	Source  string  `json:"source"`  // file the passage came from
	Section string  `json:"section"` // heading within the file
	Text    string  `json:"text"`
	Score   float64 `json:"score"` // cosine similarity
}

// QueryTopK returns the top-k passages by pgvector cosine similarity
// (ORDER BY embedding <=> q). Same embedding model/dims as ingest — the
// install doc's TC-M4-03 rule.
func (s *Store) QueryTopK(ctx context.Context, vec []float32, k int) ([]Passage, error) {
	if len(vec) != s.dim {
		return nil, fmt.Errorf("query vector dim %d != table dim %d", len(vec), s.dim)
	}
	lit := vectorLiteral(vec)
	rows, err := s.db.QueryContext(ctx,
		`SELECT source, section, text, 1 - (embedding <=> $1::vector) AS score
		   FROM wiki_chunks
		  ORDER BY embedding <=> $1::vector
		  LIMIT $2`, lit, k)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Passage{}
	for rows.Next() {
		var p Passage
		if err := rows.Scan(&p.Source, &p.Section, &p.Text, &p.Score); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Reindex replaces the whole index inside one transaction: DELETE all rows,
// then insert every chunk with its embedding. (Full reindex per the POC
// curation cadence: commit markdown → run ingest.)
func (s *Store) Reindex(ctx context.Context, chunks []Chunk, embedder Embedder, progress func(done, total int)) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM wiki_chunks`); err != nil {
		return err
	}

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO wiki_chunks (source, section, text, embedding)
		 VALUES ($1, $2, $3, $4::vector)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, ch := range chunks {
		vec, err := embedder.Embed(ctx, ch.Text)
		if err != nil {
			return fmt.Errorf("embed %s#%s: %w", ch.Source, ch.Section, err)
		}
		if len(vec) != s.dim {
			return fmt.Errorf("embedder dim %d != table dim %d (EMBEDDING_DIM mismatch)", len(vec), s.dim)
		}
		if _, err := stmt.ExecContext(ctx, ch.Source, ch.Section, ch.Text, vectorLiteral(vec)); err != nil {
			return err
		}
		if progress != nil {
			progress(i+1, len(chunks))
		}
	}
	return tx.Commit()
}

// Count returns the number of indexed chunks.
func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM wiki_chunks`).Scan(&n)
	return n, err
}

// CheckHealthy pings the DB for /healthz.
func (s *Store) CheckHealthy() error { return s.db.Ping() }

// vectorLiteral renders a float vector as a pgvector literal '[1,2,3]'.
func vectorLiteral(vec []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range vec {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%g", v)
	}
	b.WriteByte(']')
	return b.String()
}
