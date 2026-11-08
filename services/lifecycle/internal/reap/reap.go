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

// Package reap holds the pure label-based reap decision logic used by both
// the lifecycle service's reaper loop and the API handlers. It is deliberately
// free of Docker/Temporal/RabbitMQ so the expiry decision is unit-testable.
package reap

import (
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// Docker label keys pinned by M2-Lifecycle-Installation-and-Deployment.md §1b.
// The reaper's query filter matches poc.owner=lifecycle exactly.
const (
	OwnerLabel   = "poc.owner"
	TaskLabel    = "poc.task"
	ExpiresLabel = "poc.expires"
	OwnerValue   = "lifecycle"
)

// Reasons a container is selected for reaping.
const (
	ReasonExpired   = "expired"
	ReasonNoExpiry  = "missing-expires-label"
	ReasonBadExpiry = "unparsable-expires-label"
)

// ShouldReap decides whether a POC-owned container must be reaped, purely
// from its labels and the current time. A container with a missing or
// unparsable poc.expires label is reaped immediately — a leak class, not a
// keeper (troubleshooting table, M2 deploy doc §6).
//
// Ownership is assumed already established by the label filter
// (poc.owner=lifecycle); Temporal reachability is intentionally NOT consulted
// here — when Temporal is unreachable the reaper relies on poc.expires only.
func ShouldReap(labels map[string]string, now time.Time) (bool, string) {
	raw, ok := labels[ExpiresLabel]
	if !ok || raw == "" {
		return true, ReasonNoExpiry
	}
	expires, err := taskconfig.ParseExpires(raw)
	if err != nil {
		return true, ReasonBadExpiry
	}
	if !now.Before(expires) {
		return true, ReasonExpired
	}
	return false, ""
}
