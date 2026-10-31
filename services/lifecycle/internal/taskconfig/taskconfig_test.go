package taskconfig

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestValidate_OK(t *testing.T) {
	cfg := TaskConfig{TaskID: "t001", AgentRef: "repo/agents/x", BehavioralSpec: "be nice", RepoTarget: "repo/y", Requester: "manager"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidate_MissingFields(t *testing.T) {
	for _, tc := range []TaskConfig{
		{},
		{AgentRef: "a", BehavioralSpec: "b", RepoTarget: "r", Requester: "m"},
		{TaskID: "t", BehavioralSpec: "b", RepoTarget: "r", Requester: "m"},
		{TaskID: "t", AgentRef: "a", RepoTarget: "r", Requester: "m"},
		{TaskID: "t", AgentRef: "a", BehavioralSpec: "b", Requester: "m"},
	} {
		if err := tc.Validate(); err == nil {
			t.Fatalf("expected error for %+v", tc)
		}
	}
}

func TestEffectiveTTL_MessageWins(t *testing.T) {
	cfg := TaskConfig{TTLS: 15}
	if got := cfg.EffectiveTTL(900 * time.Second); got != 15*time.Second {
		t.Fatalf("want 15s got %v", got)
	}
}

func TestEffectiveTTL_DefaultWhenAbsent(t *testing.T) {
	cfg := TaskConfig{}
	if got := cfg.EffectiveTTL(900 * time.Second); got != 900*time.Second {
		t.Fatalf("want 900s got %v", got)
	}
	cfg.TTLS = -3 // invalid: fall back to default
	if got := cfg.EffectiveTTL(900 * time.Second); got != 900*time.Second {
		t.Fatalf("negative ttl_s must fall back to default, got %v", got)
	}
}

func TestExpiryTime(t *testing.T) {
	start := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	cfg := TaskConfig{TTLS: 30}
	want := start.Add(30 * time.Second)
	if got := cfg.ExpiryTime(900*time.Second, start); !got.Equal(want) {
		t.Fatalf("want %v got %v", want, got)
	}
}

func TestResponsePayloadShape(t *testing.T) {
	cfg := TaskConfig{TaskID: "t001", AgentRef: "a", BehavioralSpec: "b", RepoTarget: "r", Requester: "m"}
	resp := cfg.Response(StatusComplete, "/var/poc/tasks/t001.log")
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(m) != 3 {
		t.Fatalf("payload must have exactly task_id, status, artifact; got %v", m)
	}
	for _, k := range []string{"task_id", "status", "artifact"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing key %q in %s", k, b)
		}
	}
	if resp.TaskID != "t001" || resp.Status != "complete" {
		t.Fatalf("bad values: %+v", resp)
	}
}

func TestResponseStatuses(t *testing.T) {
	if StatusComplete != "complete" || StatusFailed != "failed" || StatusTimedOut != "timed-out" {
		t.Fatal("status strings are wire contract; do not change")
	}
}

func TestParseExpires_Error(t *testing.T) {
	if _, err := ParseExpires("yesterday"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ParseExpires(""); !errors.Is(err, err) {
		t.Fatal("unreachable")
	}
}
