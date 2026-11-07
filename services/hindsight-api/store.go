package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/lib/pq"
)

// Store is the Postgres+pgvector memory store (table memory_entries per
// deploy/m4/hindsight/initdb.sql).
type Store struct {
	db  *sql.DB
	dim int
}

// NewStore opens the DB connection pool.
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

// MemoryEntry is one row of memory_entries as returned over the API.
type MemoryEntry struct {
	TaskID   string          `json:"task_id"`
	AgentRef string          `json:"agent_ref"`
	Scope    string          `json:"scope"`
	Body     json.RawMessage `json:"body"`
	Created  string          `json:"created"`
	Score    float64         `json:"score,omitempty"` // cosine similarity (cross-task reads only)
}

// WriteResponse acknowledges a write.
type WriteResponse struct {
	Created string `json:"created"`
	Scope   string `json:"scope"`
}

// ReadResponse is the /memory/read payload.
type ReadResponse struct {
	Entries []MemoryEntry `json:"entries"`
}

// ReadExactScope returns rows written by the same task in the same scope.
func (s *Store) ReadExactScope(ctx context.Context, taskID, scope string, limit int) ([]MemoryEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id, agent_ref, scope, body, created
		   FROM memory_entries
		  WHERE task_id = $1 AND scope = $2
		  ORDER BY created DESC
		  LIMIT $3`, taskID, scope, limit)
	if err != nil {
		return nil, err
	}
	return scanEntries(rows)
}

// ReadCrossTask returns the top-k cross-task entries by vector similarity.
// The query vector must match the table's dimension (set at initdb time).
func (s *Store) ReadCrossTask(ctx context.Context, vec []float32, limit int) ([]MemoryEntry, error) {
	if len(vec) != s.dim {
		return nil, fmt.Errorf("query vector dim %d != table dim %d", len(vec), s.dim)
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT task_id, agent_ref, scope, body, created,
		          CASE WHEN (embedding <=> $1::vector)::text = 'NaN' THEN 0
		               ELSE 1 - (embedding <=> $1::vector) END AS score
		   FROM memory_entries
		  WHERE scope = 'cross-task'
		  ORDER BY CASE WHEN (embedding <=> $1::vector)::text = 'NaN' THEN 0
		                ELSE 1 - (embedding <=> $1::vector) END
		  LIMIT $2`, vectorLiteral(vec), limit)
	if err != nil {
		return nil, err
	}
	return scanEntriesScored(rows)
}

// Write inserts one memory entry; body is stored as jsonb.
func (s *Store) Write(ctx context.Context, req WriteRequest, vec []float32) (WriteResponse, error) {
	if len(vec) != s.dim {
		return WriteResponse{}, fmt.Errorf("write vector dim %d != table dim %d", len(vec), s.dim)
	}
	body, err := json.Marshal(req.Body)
	if err != nil {
		return WriteResponse{}, err
	}
	var created string
	err = s.db.QueryRowContext(ctx,
		`INSERT INTO memory_entries (task_id, agent_ref, scope, body, embedding)
		 VALUES ($1, $2, $3, $4::jsonb, $5::vector)
		 RETURNING to_char(created, 'YYYY-MM-DD"T"HH24:MI:SSOF')`,
		req.TaskID, req.AgentRef, req.Scope, string(body), vectorLiteral(vec)).Scan(&created)
	if err != nil {
		return WriteResponse{}, err
	}
	return WriteResponse{Created: created, Scope: req.Scope}, nil
}

// CheckHealthy pings the DB for /healthz.
func (s *Store) CheckHealthy() error { return s.db.Ping() }

func scanEntries(rows *sql.Rows) ([]MemoryEntry, error) {
	out := []MemoryEntry{}
	for rows.Next() {
		var e MemoryEntry
		var body []byte
		if err := rows.Scan(&e.TaskID, &e.AgentRef, &e.Scope, &body, &e.Created); err != nil {
			return nil, err
		}
		e.Body = json.RawMessage(body)
		out = append(out, e)
	}
	return out, rows.Err()
}

func scanEntriesScored(rows *sql.Rows) ([]MemoryEntry, error) {
	out := []MemoryEntry{}
	for rows.Next() {
		var e MemoryEntry
		var body []byte
		if err := rows.Scan(&e.TaskID, &e.AgentRef, &e.Scope, &body, &e.Created, &e.Score); err != nil {
			return nil, err
		}
		e.Body = json.RawMessage(body)
		out = append(out, e)
	}
	return out, rows.Err()
}

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
