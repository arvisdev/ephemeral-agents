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
