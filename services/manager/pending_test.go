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
	"os"
	"path/filepath"
	"testing"
)

// The pending store must survive a manager restart: add() persists the
// originating request as <reports_dir>/<task_id>.pending, newPending()
// reloads them at boot, take() removes the file with the entry.
func TestPendingSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	p1 := newPending(dir)
	req := TaskRequest{Title: "t", AgentCapability: "dev", TaskBrief: "b"}
	p1.add("11111111-1111-4111-8111-111111111111", req)

	if _, err := os.Stat(filepath.Join(dir, "11111111-1111-4111-8111-111111111111.pending")); err != nil {
		t.Fatalf("pending file not persisted: %v", err)
	}

	// "Restart": a fresh pending over the same directory.
	p2 := newPending(dir)
	got, ok := p2.take("11111111-1111-4111-8111-111111111111")
	if !ok || got.TaskBrief != "b" {
		t.Fatalf("reloaded pending mismatch: ok=%v req=%+v", ok, got)
	}
	if _, err := os.Stat(filepath.Join(dir, "11111111-1111-4111-8111-111111111111.pending")); !os.IsNotExist(err) {
		t.Fatalf("pending file not removed on take: %v", err)
	}
}

// An unparsable .pending file is dropped, not fatal.
func TestPendingSkipsUnparsable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-a-uuid.pending"), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newPending(dir)
	if len(p.waiting) != 0 {
		t.Fatalf("expected empty pending, got %d", len(p.waiting))
	}
}
