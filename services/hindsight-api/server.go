package main

import (
	"encoding/json"
	"net/http"
)

// Server holds the HTTP handlers for the Hindsight memory API.
type Server struct {
	store    *Store
	embedder Embedder
}

// NewServer wires the store and embedder into a Server.
func NewServer(store *Store, embedder Embedder) *Server {
	return &Server{store: store, embedder: embedder}
}

// Routes returns the service's HTTP mux.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /memory/read", s.handleRead)
	mux.HandleFunc("POST /memory/write", s.handleWrite)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleRead implements GET /memory/read per the install doc:
// exact-scope rows (same task_id + scope) plus vector similarity over
// cross-task entries when the caller asks for recall.
func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	q, err := ParseReadQuery(r.URL.Query())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}

	resp := ReadResponse{Entries: []MemoryEntry{}}
	plan := PlanRead(q.Scope, q.Query)

	// Exact-scope rows: the caller's own session/task scope.
	if q.TaskID != "" && plan.ExactScope {
		rows, err := s.store.ReadExactScope(r.Context(), q.TaskID, q.Scope, q.Limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
		resp.Entries = append(resp.Entries, rows...)
	}

	// Vector similarity over cross-task entries (scoped visibility rule:
	// other tasks' session rows stay private; cross-task rows are shared).
	if q.Query != "" && plan.CrossTask {
		vec, err := s.embedder.Embed(r.Context(), q.Query)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: "embed query: " + err.Error()})
			return
		}
		rows, err := s.store.ReadCrossTask(r.Context(), vec, q.Limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
			return
		}
		resp.Entries = append(resp.Entries, rows...)
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleWrite implements POST /memory/write: embed body.summary, insert row.
func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "invalid JSON body: " + err.Error()})
		return
	}
	if _, err := ParseWrite(req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: err.Error()})
		return
	}

	vec, err := s.embedder.Embed(r.Context(), req.Body.Summary)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: "embed body: " + err.Error()})
		return
	}

	respBody, err := s.store.Write(r.Context(), req, vec)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorBody{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, respBody)
}

type errorBody struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
