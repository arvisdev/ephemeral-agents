package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestParseReadQuery(t *testing.T) {
	t.Run("valid minimal", func(t *testing.T) {
		q, err := ParseReadQuery(url.Values{"task_id": {"t1"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if q.TaskID != "t1" || q.Scope != ScopeSession || q.Limit != 5 {
			t.Errorf("got %+v", q)
		}
	})

	t.Run("defaults and k bounds", func(t *testing.T) {
		q, err := ParseReadQuery(url.Values{"task_id": {"t1"}, "k": {"10"}})
		if err != nil || q.Limit != 10 {
			t.Errorf("got %+v err=%v", q, err)
		}
		_, err = ParseReadQuery(url.Values{"task_id": {"t1"}, "k": {"0"}})
		if err == nil || !strings.Contains(err.Error(), "k must be") {
			t.Errorf("k=0: want bounded error, got %v", err)
		}
		_, err = ParseReadQuery(url.Values{"task_id": {"t1"}, "k": {"51"}})
		if err == nil {
			t.Error("k=51: want error")
		}
		// k=0 falls back to the default 5 (a missing param reads as "0").
		q, err = ParseReadQuery(url.Values{"task_id": {"t1"}, "k": {""}})
		if err != nil || q.Limit != 5 {
			t.Errorf("empty k: got %+v err=%v", q, err)
		}
	})

	t.Run("requires task_id or q", func(t *testing.T) {
		_, err := ParseReadQuery(url.Values{})
		if err == nil || !strings.Contains(err.Error(), "task_id or q") {
			t.Errorf("want required-params error, got %v", err)
		}
	})

	t.Run("invalid scope rejected", func(t *testing.T) {
		_, err := ParseReadQuery(url.Values{"task_id": {"t1"}, "scope": {"global"}})
		if err == nil {
			t.Error("want scope error")
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		q, err := ParseReadQuery(url.Values{"task_id": {"  t1  "}, "q": {" use X "}})
		if err != nil || q.TaskID != "t1" || q.Query != "use X" {
			t.Errorf("got %+v err=%v", q, err)
		}
	})
}

func TestParseWrite(t *testing.T) {
	valid := WriteRequest{TaskID: "t1", AgentRef: "agents/coder", Scope: "cross-task",
		Body: EntryBody{Summary: "use X for Y"}}

	t.Run("valid", func(t *testing.T) {
		got, err := ParseWrite(valid)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Scope != ScopeCrossTask {
			t.Errorf("scope = %q", got.Scope)
		}
	})

	t.Run("empty scope defaults to session", func(t *testing.T) {
		req := valid
		req.Scope = ""
		got, err := ParseWrite(req)
		if err != nil || got.Scope != ScopeSession {
			t.Errorf("got %+v err=%v", got, err)
		}
	})

	for name, mutate := range map[string]func(*WriteRequest){
		"missing task_id":  func(r *WriteRequest) { r.TaskID = "" },
		"blank task_id":    func(r *WriteRequest) { r.TaskID = "   " },
		"missing agent":    func(r *WriteRequest) { r.AgentRef = "" },
		"missing summary":  func(r *WriteRequest) { r.Body.Summary = "" },
		"invalid scope":    func(r *WriteRequest) { r.Scope = "global" },
		"scope whitespace": func(r *WriteRequest) { r.Scope = " session " },
	} {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			_, err := ParseWrite(req)
			if err == nil {
				t.Errorf("%s: want error", name)
			}
		})
	}
}
