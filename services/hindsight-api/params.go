package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// WriteRequest is the POST /memory/write body
// ({task_id, agent_ref, scope, body} per the install doc).
type WriteRequest struct {
	TaskID   string    `json:"task_id"`
	AgentRef string    `json:"agent_ref"`
	Scope    string    `json:"scope"`
	Body     EntryBody `json:"body"`
}

// EntryBody is the memory entry payload; Summary is what gets embedded.
type EntryBody struct {
	Summary   string   `json:"summary"`
	Artifacts []string `json:"artifacts,omitempty"`
	Learning  string   `json:"learning,omitempty"`
}

// ParseWrite validates a write request (pure; unit-tested).
func ParseWrite(req WriteRequest) (WriteRequest, error) {
	if strings.TrimSpace(req.TaskID) == "" {
		return req, fmt.Errorf("task_id is required")
	}
	if strings.TrimSpace(req.AgentRef) == "" {
		return req, fmt.Errorf("agent_ref is required")
	}
	scope, err := NormalizeScope(req.Scope)
	if err != nil {
		return req, err
	}
	req.Scope = scope
	if strings.TrimSpace(req.Body.Summary) == "" {
		return req, fmt.Errorf("body.summary is required (it is what gets embedded)")
	}
	return req, nil
}

// ReadQuery is the parsed GET /memory/read query string.
type ReadQuery struct {
	TaskID string
	Scope  string
	Query  string
	Limit  int
}

// ParseReadQuery parses + validates the read endpoint's query parameters
// (task_id, scope, q, k). Pure; unit-tested.
func ParseReadQuery(vals url.Values) (ReadQuery, error) {
	q := ReadQuery{
		TaskID: strings.TrimSpace(vals.Get("task_id")),
		Query:  strings.TrimSpace(vals.Get("q")),
	}
	scope, err := NormalizeScope(vals.Get("scope"))
	if err != nil {
		return q, err
	}
	q.Scope = scope

	if raw := vals.Get("k"); strings.TrimSpace(raw) == "" {
		q.Limit = 5
	} else {
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return q, fmt.Errorf("k must be an integer, got %q", raw)
		}
		q.Limit = n
	}
	if q.Limit < 1 || q.Limit > 50 {
		return q, fmt.Errorf("k must be between 1 and 50, got %q", vals.Get("k"))
	}
	if q.TaskID == "" && q.Query == "" {
		return q, fmt.Errorf("at least one of task_id or q is required")
	}
	return q, nil
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n == 0 {
		return def
	}
	return n
}
