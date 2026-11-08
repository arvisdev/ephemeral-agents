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

// Scoping rules (M4 delivery plan WP-4.1, install doc §1b): an entry is
// written with scope 'session' (private to the run/task) or 'cross-task'
// (visible to later tasks). These pure functions are the single source of
// truth for that contract and are unit-tested in scope_test.go.

const (
	ScopeSession   = "session"
	ScopeCrossTask = "cross-task"
)

// NormalizeScope validates + normalizes a caller-supplied scope string.
// Empty scope defaults to "session" (the safest private default).
func NormalizeScope(s string) (string, error) {
	switch s {
	case "":
		return ScopeSession, nil
	case ScopeSession, ScopeCrossTask:
		return s, nil
	default:
		return "", &ScopeError{Got: s}
	}
}

// ScopeError reports an invalid scope value.
type ScopeError struct{ Got string }

func (e *ScopeError) Error() string {
	return "invalid scope \"" + e.Got + "\" (must be \"session\" or \"cross-task\")"
}

// CrossTaskVisible reports whether an entry written with writeScope is
// visible to a reader reading in readScope from a different task.
//
// Rule: only cross-task entries cross task boundaries. A reader asking for
// 'cross-task' sees every cross-task entry; a reader asking for 'session'
// only ever sees its own task's rows (handled by the exact-scope query path).
func CrossTaskVisible(writeScope, readScope string) bool {
	return writeScope == ScopeCrossTask && readScope == ScopeCrossTask
}

// ReadPlan describes which store queries a read request fans out to.
type ReadPlan struct {
	ExactScope bool   // query own task_id+scope rows
	CrossTask  bool   // vector-similarity over cross-task rows
	Reason     string // why, for logs/evidence
}

// PlanRead decides the fan-out for a read request (pure, unit-tested).
// session reads: exact rows only — other tasks' session rows are private.
// cross-task reads: exact rows + similarity over the shared pool.
func PlanRead(scope, query string) ReadPlan {
	if scope == ScopeSession {
		return ReadPlan{ExactScope: true, Reason: "session scope is private to the task"}
	}
	if query == "" {
		return ReadPlan{ExactScope: true, Reason: "no query text: exact rows only"}
	}
	return ReadPlan{ExactScope: true, CrossTask: true, Reason: "cross-task read includes shared pool similarity"}
}
