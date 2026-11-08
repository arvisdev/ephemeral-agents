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
	"fmt"
	"testing"
)

// validMessage matches schemas/message-v1.json exactly.
var validMessage = map[string]interface{}{
	"task_id":         "3f2b8c1e-9d4a-4b7e-8c2f-1a5d6e7f8a90",
	"agent_ref":       "git@github.com:acme/agents.git#agents/reviewer",
	"behavioral_spec": "Review PRs for memory-safety issues",
	"repo_target":     "git@github.com:acme/target.git",
	"requester":       "manager@vm-xa",
}

func mustJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func TestValidateValidMessage(t *testing.T) {
	if reason := ValidateTaskMessage(mustJSON(t, validMessage)); reason != "" {
		t.Fatalf("expected valid, got reason: %s", reason)
	}
}

func TestValidateValidMessageWithTTL(t *testing.T) {
	m := map[string]interface{}{}
	for k, v := range validMessage {
		m[k] = v
	}
	m["ttl_s"] = float64(900)
	if reason := ValidateTaskMessage(mustJSON(t, m)); reason != "" {
		t.Fatalf("expected valid with ttl_s, got reason: %s", reason)
	}
}

func TestValidateRejectsBadJSON(t *testing.T) {
	if reason := ValidateTaskMessage([]byte("{not json")); reason == "" {
		t.Fatal("expected invalid JSON to be rejected")
	}
}

func TestValidateRejectsMissingRequired(t *testing.T) {
	for _, field := range []string{"task_id", "agent_ref", "behavioral_spec", "repo_target", "requester"} {
		m := map[string]interface{}{}
		for k, v := range validMessage {
			m[k] = v
		}
		delete(m, field)
		reason := ValidateTaskMessage(mustJSON(t, m))
		if reason == "" {
			t.Errorf("expected missing %q to be rejected", field)
		}
	}
}

func TestValidateRejectsNonUUIDTaskID(t *testing.T) {
	m := map[string]interface{}{}
	for k, v := range validMessage {
		m[k] = v
	}
	m["task_id"] = "not-a-uuid"
	if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
		t.Fatal("expected non-UUID task_id to be rejected")
	}
}

func TestValidateRejectsTTLBelowMinimum(t *testing.T) {
	for _, ttl := range []interface{}{float64(0), float64(-1)} {
		m := map[string]interface{}{}
		for k, v := range validMessage {
			m[k] = v
		}
		m["ttl_s"] = ttl
		if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
			t.Errorf("expected ttl_s=%v to be rejected (minimum 1)", ttl)
		}
	}
}

func TestValidateRejectsNonIntegerTTL(t *testing.T) {
	m := map[string]interface{}{}
	for k, v := range validMessage {
		m[k] = v
	}
	m["ttl_s"] = "soon" // wrong type: string, schema says integer
	if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
		t.Fatal("expected string ttl_s to be rejected")
	}
}

func TestValidateRejectsAdditionalProperties(t *testing.T) {
	m := map[string]interface{}{}
	for k, v := range validMessage {
		m[k] = v
	}
	m["extra"] = "nope"
	if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
		t.Fatal("expected additional property to be rejected (additionalProperties=false)")
	}
}

func TestValidateRejectsWrongTypes(t *testing.T) {
	for _, field := range []string{"agent_ref", "behavioral_spec", "repo_target", "requester"} {
		m := map[string]interface{}{}
		for k, v := range validMessage {
			m[k] = v
		}
		m[field] = 42 // schema: string
		if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
			t.Errorf("expected non-string %q to be rejected", field)
		}
	}
}

func TestValidateRejectsNullRequiredField(t *testing.T) {
	m := map[string]interface{}{}
	for k, v := range validMessage {
		m[k] = v
	}
	m["requester"] = nil
	if reason := ValidateTaskMessage(mustJSON(t, m)); reason == "" {
		t.Fatal("expected null required field to be rejected")
	}
}

// UUID format cross-check: all 5 versions' shape (variant nibble) via hex decode.
func Example_validateUUIDFormat() {
	reason := ValidateTaskMessage([]byte(`{"task_id":"00000000-0000-4000-8000-000000000000","agent_ref":"a","behavioral_spec":"b","repo_target":"c","requester":"d"}`))
	fmt.Println(reason == "")
	// Output: true
}
