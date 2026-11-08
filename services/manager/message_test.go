package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func testAgent() AgentIndexEntry {
	return AgentIndexEntry{
		Ref: "dev-agent", Name: "Dev Agent", Role: "dev",
		Folder: "agents/dev-agent", Skills: []string{"go", "git"},
		SpecRef: "agents/dev-agent/behavioral-spec.md",
	}
}

func testReq() TaskRequest {
	return TaskRequest{Title: "add health endpoint", AgentCapability: "dev", TaskBrief: "add /healthz"}
}

// --- ttl_s rule: max(3 × B1 boot-time p50, brief estimate) -----------------

func TestComputeTTLS(t *testing.T) {
	cases := []struct {
		name         string
		bootP50, est int64
		want         int64
	}{
		{"estimate dominates", 60, 1800, 1800},
		{"3x p50 dominates", 60, 100, 180},
		{"equal", 300, 900, 900},
		{"zero estimate", 45, 0, 135},
		{"floor at 1 (schema minimum)", 0, 0, 1},
		{"negative p50 clamped", -5, 10, 10},
	}
	for _, tc := range cases {
		if got := ComputeTTLS(tc.bootP50, tc.est); got != tc.want {
			t.Errorf("%s: ComputeTTLS(%d,%d) = %d, want %d", tc.name, tc.bootP50, tc.est, got, tc.want)
		}
	}
}

// --- schema conformance (schemas/message-v1.json) ---------------------------

func TestComposeTaskMessageSchemaConformance(t *testing.T) {
	msg, err := ComposeTaskMessage(testReq(), testAgent(),
		"https://github.com/owner/agent-roster.git",
		"https://github.com/owner/project.git", "desktop-engineer", 60, 100)
	if err != nil {
		t.Fatalf("ComposeTaskMessage: %v", err)
	}
	if err := msg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	// required fields present + typed per the schema
	if !uuidRe.MatchString(msg.TaskID) {
		t.Errorf("task_id %q not a UUID", msg.TaskID)
	}
	if msg.AgentRef == "" || msg.BehavioralSpec == "" || msg.RepoTarget == "" || msg.Requester == "" {
		t.Errorf("a required field is empty: %+v", msg)
	}
	// agent_ref = repo location + repo folder (Note 1)
	if !strings.HasPrefix(msg.AgentRef, "https://github.com/owner/agent-roster.git#") ||
		!strings.HasSuffix(msg.AgentRef, "agents/dev-agent") {
		t.Errorf("agent_ref = %q, want repo#folder", msg.AgentRef)
	}
	// ttl_s written by the manager: max(3*60, 100) = 180
	if msg.TTLS == nil || *msg.TTLS != 180 {
		t.Errorf("ttl_s = %v, want 180 (manager sole writer)", msg.TTLS)
	}
	// MarshalStrict output must contain exactly the schema's allowed keys
	b, err := msg.MarshalStrict()
	if err != nil {
		t.Fatalf("MarshalStrict: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		t.Fatalf("wire format not JSON: %v", err)
	}
	for _, k := range []string{"task_id", "agent_ref", "behavioral_spec", "repo_target", "requester", "ttl_s"} {
		if _, ok := probe[k]; !ok {
			t.Errorf("wire format missing %q: %s", k, b)
		}
	}
	if len(probe) != 6 {
		t.Errorf("wire format has %d keys, want exactly the 6 schema properties: %s", len(probe), b)
	}
	// unique task_ids across compositions (idempotency key)
	msg2, _ := ComposeTaskMessage(testReq(), testAgent(), "r", "rt", "rq", 1, 1)
	if msg2.TaskID == msg.TaskID {
		t.Error("two compositions produced the same task_id")
	}
}

func TestComposeTaskMessageRejections(t *testing.T) {
	if _, err := ComposeTaskMessage(testReq(), AgentIndexEntry{Role: "dev"}, "repo", "rt", "rq", 1, 1); err == nil {
		t.Error("entry without folder accepted")
	}
	if _, err := ComposeTaskMessage(testReq(), testAgent(), "", "rt", "rq", 1, 1); err == nil {
		t.Error("empty catalog repo accepted")
	}
	if _, err := ComposeTaskMessage(testReq(), testAgent(), "repo", "", "rq", 1, 1); err == nil {
		t.Error("empty repo_target accepted")
	}
	if _, err := ComposeTaskMessage(testReq(), testAgent(), "repo", "rt", "", 1, 1); err == nil {
		t.Error("empty requester accepted")
	}
}

// --- agent index search ------------------------------------------------------

func TestMatchAgent(t *testing.T) {
	index := []AgentIndexEntry{
		{Ref: "dev-agent", Role: "dev", Folder: "agents/dev-agent"},
		{Ref: "review-agent", Role: "reviewer", Folder: "agents/review-agent"},
	}
	got, err := MatchAgent(index, "dev")
	if err != nil || got.Ref != "dev-agent" {
		t.Errorf("MatchAgent(dev) = %+v, %v", got, err)
	}
	if _, err := MatchAgent(index, "qa"); err == nil {
		t.Error("unknown capability accepted")
	}
	if _, err := MatchAgent(index, ""); err == nil {
		t.Error("empty capability accepted")
	}
	if got, err := MatchAgent(index, "review-agent"); err != nil || got.Folder != "agents/review-agent" {
		t.Errorf("exact ref match failed: %+v, %v", got, err)
	}
}

func TestParseAgentIndex(t *testing.T) {
	idx, err := ParseAgentIndex([]byte(`[
		{"ref":"dev-agent","name":"Dev Agent","role":"dev","folder":"agents/dev-agent","skills":["go"],"spec_ref":"agents/dev-agent/behavioral-spec.md"}
	]`))
	if err != nil || len(idx) != 1 || idx[0].Role != "dev" {
		t.Fatalf("ParseAgentIndex = %+v, %v", idx, err)
	}
	if _, err := ParseAgentIndex([]byte(`not json`)); err == nil {
		t.Error("malformed index accepted")
	}
	if _, err := ParseAgentIndex([]byte(`[]`)); err == nil {
		t.Error("empty index accepted")
	}
}

// --- response filtering (agent.tasks.response) -------------------------------

func TestResponseMatches(t *testing.T) {
	const id = "6f9619ff-8b86-d011-b42d-00c04fc964ff"
	if !ResponseMatches(TaskResponse{TaskID: id, Status: "success", Artifact: "task/" + id}, id) {
		t.Error("matching response filtered out")
	}
	for _, resp := range []TaskResponse{
		{TaskID: "00000000-0000-0000-0000-000000000000", Status: "success"},
		{TaskID: "", Status: "success"},
	} {
		if ResponseMatches(resp, id) {
			t.Errorf("response %+v should be ignored", resp)
		}
	}
}

func TestBuildReport(t *testing.T) {
	const id = "6f9619ff-8b86-d011-b42d-00c04fc964ff"
	r, err := BuildReport(TaskResponse{TaskID: id, Status: "success", Artifact: "task/" + id + "@abc123"}, "")
	if err != nil {
		t.Fatalf("BuildReport: %v", err)
	}
	if r.Status != "success" || r.Artifact != "task/"+id+"@abc123" || r.TaskID != id {
		t.Errorf("report = %+v", r)
	}
	if r.CompletedAt == "" {
		t.Error("completed_at missing")
	}
	// failure: human-visible reason (M5 §5)
	r, err = BuildReport(TaskResponse{TaskID: id, Status: "failure"}, "hydration source down")
	if err != nil {
		t.Fatalf("BuildReport failure: %v", err)
	}
	if r.Status != "failure" || r.Reason != "hydration source down" {
		t.Errorf("failure report = %+v", r)
	}
	r, _ = BuildReport(TaskResponse{TaskID: id, Status: "failure"}, "")
	if r.Reason == "" {
		t.Error("failure without worker reason must still carry a human-visible reason")
	}
	if _, err := BuildReport(TaskResponse{TaskID: id, Status: "weird"}, ""); err == nil {
		t.Error("unknown status accepted")
	}
	if _, err := BuildReport(TaskResponse{}, ""); err == nil {
		t.Error("response without task_id accepted")
	}
	// report marshals to the /reports/<task_id>.json shape
	b, _ := json.Marshal(r)
	var probe map[string]any
	json.Unmarshal(b, &probe)
	for _, k := range []string{"task_id", "status", "outcome", "reason", "completed_at"} {
		if _, ok := probe[k]; !ok {
			t.Errorf("report JSON missing %q: %s", k, b)
		}
	}
}
