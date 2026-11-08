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

// Package client is the executor-side helper for the Hindsight memory API
// (M4 install doc §1b: "this is what the executor entrypoints use").
//
// Usage:
//
//	c := client.New("http://vm-hindsight:8081")
//	cross, err := c.ReadCrossTask(ctx, "t2", "how to deploy X", 5)
//	own, err := c.ReadOwnScope(ctx, "t2", 5)
//	err = c.Write(ctx, "t2", "agents/coder", client.ScopeCrossTask, map[string]any{
//	    "summary":  "use X for Y",
//	    "learning": "X fails without pre-warming",
//	})
//
// curl examples:
//
//	curl -s "$HINDSIGHT_URL/memory/read?task_id=t1&scope=cross-task"
//	curl -s "$HINDSIGHT_URL/memory/read?task_id=t2&scope=cross-task&q=deploy" | jq .
//	curl -s -XPOST "$HINDSIGHT_URL/memory/write" \
//	  -H 'Content-Type: application/json' \
//	  -d '{"task_id":"t1","agent_ref":"agents/coder","scope":"cross-task",
//	       "body":{"summary":"use X for Y"}}'
//	curl -s "$HINDSIGHT_URL/healthz"
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Scope values mirrored from the API's contract (§1 scoping rules).
const (
	ScopeSession   = "session"
	ScopeCrossTask = "cross-task"
)

// Client talks to the Hindsight API over HTTP.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// New returns a Client for base (e.g. from $HINDSIGHT_URL).
func New(base string) *Client {
	return &Client{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Entry is one memory record as returned by Read*.
type Entry struct {
	TaskID   string          `json:"task_id"`
	AgentRef string          `json:"agent_ref"`
	Scope    string          `json:"scope"`
	Body     json.RawMessage `json:"body"`
	Created  string          `json:"created"`
	Score    float64         `json:"score,omitempty"`
}

type readResponse struct {
	Entries []Entry `json:"entries"`
}

type writeResponse struct {
	Created string `json:"created"`
	Scope   string `json:"scope"`
}

// ReadCrossTask fetches top-k cross-task entries relevant to query
// (this is what task 2 calls to see task 1's learnings — the G2 path).
func (c *Client) ReadCrossTask(ctx context.Context, taskID, query string, k int) ([]Entry, error) {
	return c.read(ctx, taskID, ScopeCrossTask, query, k)
}

// ReadOwnScope fetches the task's own session-scope rows.
func (c *Client) ReadOwnScope(ctx context.Context, taskID string, k int) ([]Entry, error) {
	return c.read(ctx, taskID, ScopeSession, "", k)
}

func (c *Client) read(ctx context.Context, taskID, scope, query string, k int) ([]Entry, error) {
	v := url.Values{"scope": {scope}}
	if taskID != "" {
		v.Set("task_id", taskID)
	}
	if query != "" {
		v.Set("q", query)
	}
	if k > 0 {
		v.Set("k", fmt.Sprint(k))
	}

	var out readResponse
	if err := c.do(ctx, http.MethodGet, "/memory/read?"+v.Encode(), nil, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

// Write persists one memory entry at task end. Body must include a
// "summary" field (it is what gets embedded).
func (c *Client) Write(ctx context.Context, taskID, agentRef, scope string, body map[string]any) error {
	req := map[string]any{"task_id": taskID, "agent_ref": agentRef, "scope": scope, "body": body}
	var out writeResponse
	return c.do(ctx, http.MethodPost, "/memory/write", req, &out)
}

// Healthy checks /healthz.
func (c *Client) Healthy(ctx context.Context) error {
	var out map[string]string
	return c.do(ctx, http.MethodGet, "/healthz", nil, &out)
}

func (c *Client) do(ctx context.Context, method, path string, reqBody, out any) error {
	var rd *bytes.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rd)
	if err != nil {
		return err
	}
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("hindsight %s %s: %s", method, path, resp.Status)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
