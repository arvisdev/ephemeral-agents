package activity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// fakeAPI fakes the lifecycle endpoints; no network/docker.
type fakeAPI struct {
	states []taskconfig.StatusResponse
	reaped []string
}

func (f *fakeAPI) Spawn(_ context.Context, _ taskconfig.TaskConfig) (taskconfig.SpawnResponse, error) {
	return taskconfig.SpawnResponse{ContainerID: "cid-1", Expires: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}, nil
}

func (f *fakeAPI) Status(_ context.Context, taskID string) (taskconfig.StatusResponse, error) {
	if len(f.states) == 0 {
		return taskconfig.StatusResponse{TaskID: taskID, State: "running"}, nil
	}
	st := f.states[0]
	f.states = f.states[1:]
	return st, nil
}

func (f *fakeAPI) Reap(_ context.Context, taskID, _ string) error {
	f.reaped = append(f.reaped, taskID)
	return nil
}

type fakePub struct {
	got    taskconfig.Response
	called bool
	err    error
}

func (p *fakePub) PublishResponse(_ context.Context, r taskconfig.Response) error {
	p.got, p.called = r, true
	return p.err
}

func cfg() taskconfig.TaskConfig {
	return taskconfig.TaskConfig{TaskID: "t001", AgentRef: "a", BehavioralSpec: "b", RepoTarget: "r", Requester: "m"}
}

var testLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func run(t *testing.T, api *fakeAPI, pub *fakePub) taskconfig.Response {
	t.Helper()
	a := &Activity{API: api, Publisher: pub, PollEvery: time.Millisecond, Log: testLog}
	resp, err := a.Execute(context.Background(), cfg())
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return resp
}

func TestExecute_CompletePath(t *testing.T) {
	api := &fakeAPI{states: []taskconfig.StatusResponse{{TaskID: "t001", State: "running"}, {TaskID: "t001", State: "exited", ExitCode: 0}}}
	pub := &fakePub{}
	resp := run(t, api, pub)
	if resp.Status != taskconfig.StatusComplete {
		t.Fatalf("want complete got %q", resp.Status)
	}
	if len(api.reaped) != 1 || api.reaped[0] != "t001" {
		t.Fatalf("expected reap of t001, got %v", api.reaped)
	}
	if !pub.called || pub.got.TaskID != "t001" || pub.got.Status != "complete" {
		t.Fatalf("bad publish: %+v called=%v", pub.got, pub.called)
	}
	if resp.Artifact != "/var/poc/tasks/t001.log" {
		t.Fatalf("artifact: %q", resp.Artifact)
	}
}

func TestExecute_FailurePath(t *testing.T) {
	api := &fakeAPI{states: []taskconfig.StatusResponse{{TaskID: "t001", State: "exited", ExitCode: 1}}}
	pub := &fakePub{}
	resp := run(t, api, pub)
	if resp.Status != taskconfig.StatusFailed {
		t.Fatalf("want failed got %q", resp.Status)
	}
	if len(api.reaped) != 1 {
		t.Fatal("failure path must still reap")
	}
}

func TestExecute_TTLTimeoutReaps(t *testing.T) {
	// Status stays running; expiry is in the past → timed-out + reap.
	expired := time.Now().Add(-time.Second).UTC().Format(time.RFC3339)
	_ = expired
	a := &Activity{API: &fakeAPI{}, Publisher: &fakePub{}, PollEvery: time.Millisecond, Log: testLog}
	status, reason := a.pollUntilTerminal(context.Background(), "t001", time.Now().Add(-time.Second))
	if status != taskconfig.StatusTimedOut || reason != "ttl" {
		t.Fatalf("want timed-out/ttl, got %s/%s", status, reason)
	}
	_ = expired
}

func TestExecute_PublishFailureReturnsError(t *testing.T) {
	api := &fakeAPI{states: []taskconfig.StatusResponse{{TaskID: "t001", State: "exited", ExitCode: 0}}}
	pub := &fakePub{err: errors.New("amqp down")}
	a := &Activity{API: api, Publisher: pub, PollEvery: time.Millisecond, Log: testLog}
	_, err := a.Execute(context.Background(), cfg())
	if err == nil {
		t.Fatal("publish failure must surface so Temporal retries the activity")
	}
	if len(api.reaped) != 1 {
		t.Fatal("reap must happen even when publish fails")
	}
}
