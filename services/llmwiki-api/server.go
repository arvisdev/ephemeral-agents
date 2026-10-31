package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Server holds the HTTP handlers for the LLM-Wiki API.
type Server struct {
	store    *Store
	embedder Embedder
}

// NewServer wires store + embedder.
func NewServer(store *Store, embedder Embedder) *Server {
	return &Server{store: store, embedder: embedder}
}

// Routes returns the service's HTTP mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /wiki/query", s.handleQuery)
	mux.HandleFunc("POST /wiki/ingest", s.handleIngest)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleQuery implements GET /wiki/query?q=<topic>&k=5 → passages with
// source file + section (install doc §2b).
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	q, err := ParseQueryParams(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}
	vec, err := s.embedder.Embed(r.Context(), q.Query)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "embed query: " + err.Error()})
		return
	}
	passages, err := s.store.QueryTopK(r.Context(), vec, q.K)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, QueryResponse{Query: q.Query, Passages: passages})
}

// handleIngest implements POST /wiki/ingest → clone + reindex (same path as
// the one-shot `ingest` job).
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	cfg := ConfigFromEnv()
	if cfg.ContentRepo == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "CONTENT_REPO is not configured"})
		return
	}
	stats, err := IngestCLI(r.Context(), cfg, s.embedder)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

type errorBody struct {
	Error string `json:"error"`
}

// QueryResponse is the /wiki/query payload.
type QueryResponse struct {
	Query    string    `json:"query"`
	Passages []Passage `json:"passages"`
}

// IngestStats reports what one reindex did.
type IngestStats struct {
	Files  int `json:"files"`
	Chunks int `json:"chunks"`
}

// ParseQueryParams parses + validates the query endpoint's parameters
// (q, k). Pure; unit-tested.
func ParseQueryParams(vals url.Values) (QueryParams, error) {
	q := QueryParams{Query: strings.TrimSpace(vals.Get("q"))}
	if q.Query == "" {
		return q, fmt.Errorf("q is required (the topic to retrieve standards/skills for)")
	}
	if raw := vals.Get("k"); strings.TrimSpace(raw) == "" {
		q.K = 5
	} else {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return q, fmt.Errorf("k must be an integer, got %q", raw)
		}
		q.K = n
	}
	if q.K < 1 || q.K > 50 {
		return q, fmt.Errorf("k must be between 1 and 50, got %q", vals.Get("k"))
	}
	return q, nil
}

// QueryParams is a parsed /wiki/query request.
type QueryParams struct {
	Query string
	K     int
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
