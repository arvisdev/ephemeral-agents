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

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateRequest(t *testing.T) {
	ok := taskRequest{Title: "add health endpoint", AgentCapability: "dev", TaskBrief: "do the thing"}
	if err := validateRequest(ok); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		req  taskRequest
		want string
	}{
		{"missing title", taskRequest{AgentCapability: "dev", TaskBrief: "b"}, "title"},
		{"missing capability", taskRequest{Title: "t", TaskBrief: "b"}, "agent_capability"},
		{"missing brief", taskRequest{Title: "t", AgentCapability: "dev"}, "task_brief"},
		{"blank fields", taskRequest{Title: "  ", AgentCapability: "dev", TaskBrief: "b"}, "title"},
	} {
		err := validateRequest(tc.req)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want error naming %q, got %v", tc.name, tc.want, err)
		}
	}
	if err := validateRequest(taskRequest{Title: "t", AgentCapability: "dev", TaskBrief: strings.Repeat("x", 65*1024)}); err == nil {
		t.Error("oversized task_brief not rejected")
	}
}

func TestRoutes(t *testing.T) {
	var gotPath, gotBody string
	mgr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		b := make([]byte, r.ContentLength)
		r.Body.Read(b)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"task_id":"6f9619ff-8b86-d011-b42d-00c04fc964ff"}`))
	}))
	defer mgr.Close()

	gw := httptest.NewServer(newHandler(mgr.URL))
	defer gw.Close()

	resp, err := http.Post(gw.URL+"/", "application/json",
		strings.NewReader(`{"title":"t","agent_capability":"dev","task_brief":"b"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("gateway POST status = %d", resp.StatusCode)
	}
	if gotPath != "/tasks" {
		t.Errorf("proxied path = %q, want /tasks", gotPath)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("proxied body not JSON: %v (%s)", err, gotBody)
	}
	if body["agent_capability"] != "dev" || body["title"] != "t" {
		t.Errorf("proxied body = %s", gotBody)
	}
	var out map[string]string
	json.NewDecoder(resp.Body).Decode(&out)
	if out["task_id"] != "6f9619ff-8b86-d011-b42d-00c04fc964ff" {
		t.Errorf("response task_id = %v", out["task_id"])
	}

	// bad request shape -> 400, not proxied
	resp, _ = http.Post(gw.URL+"/", "application/json", strings.NewReader(`{"title":"t"}`))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad body status = %d, want 400", resp.StatusCode)
	}

	// report proxy
	resp, _ = http.Get(gw.URL + "/gateway/reports/6f9619ff-8b86-d011-b42d-00c04fc964ff")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("report proxy status = %d", resp.StatusCode)
	}
	if gotPath != "/reports/6f9619ff-8b86-d011-b42d-00c04fc964ff" {
		t.Errorf("report proxied path = %q", gotPath)
	}
	resp, _ = http.Get(gw.URL + "/gateway/reports/not-a-uuid")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed task_id status = %d, want 400", resp.StatusCode)
	}
}
