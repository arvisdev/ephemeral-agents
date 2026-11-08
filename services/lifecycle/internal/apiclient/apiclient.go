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

// Package apiclient is the worker's HTTP client for the lifecycle service.
// The worker never holds a Docker client across retries — it only talks to
// the lifecycle API (M2 delivery plan: service boundary).
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// Client calls the lifecycle service over the compose network.
type Client struct {
	Base string // e.g. http://lifecycle:8090
	HTTP *http.Client
}

func New(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// Spawn POSTs the task config; the lifecycle computes and owns the TTL.
func (c *Client) Spawn(ctx context.Context, cfg taskconfig.TaskConfig) (taskconfig.SpawnResponse, error) {
	var out taskconfig.SpawnResponse
	if err := c.do(ctx, http.MethodPost, "/spawn", cfg, &out); err != nil {
		return out, err
	}
	if _, err := taskconfig.ParseExpires(out.Expires); err != nil {
		return out, fmt.Errorf("lifecycle returned bad expires: %w", err)
	}
	return out, nil
}

// Status GETs the container state for a task.
func (c *Client) Status(ctx context.Context, taskID string) (taskconfig.StatusResponse, error) {
	var out taskconfig.StatusResponse
	err := c.do(ctx, http.MethodGet, "/status?task="+url.QueryEscape(taskID), nil, &out)
	return out, err
}

// Reap is idempotent server-side; calling it for an absent task succeeds.
func (c *Client) Reap(ctx context.Context, taskID, reason string) error {
	return c.do(ctx, http.MethodPost,
		"/reap?task="+url.QueryEscape(taskID)+"&reason="+url.QueryEscape(reason), nil, nil)
}

// ImagePresent checks the deploy gate remotely (GET /report).
func (c *Client) ImagePresent(ctx context.Context) (string, error) {
	var rep map[string]any
	if err := c.do(ctx, http.MethodGet, "/report", nil, &rep); err != nil {
		return "", err
	}
	img, _ := rep["pinned_image"].(string)
	return img, nil
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rd)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var e map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("lifecycle %s %s: %d %s", method, path, resp.StatusCode, e["error"])
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
