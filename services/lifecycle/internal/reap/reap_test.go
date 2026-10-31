package reap

import (
	"testing"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

func now() time.Time { return time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC) }

func TestShouldReap_NotExpired(t *testing.T) {
	labels := map[string]string{
		OwnerLabel:   OwnerValue,
		TaskLabel:    "t001",
		ExpiresLabel: now().Add(30 * time.Second).UTC().Format(time.RFC3339),
	}
	if ok, reason := ShouldReap(labels, now()); ok {
		t.Fatalf("expected keep, got reap (reason %q)", reason)
	}
}

func TestShouldReap_Expired(t *testing.T) {
	labels := map[string]string{
		OwnerLabel:   OwnerValue,
		ExpiresLabel: now().Add(-1 * time.Second).UTC().Format(time.RFC3339),
	}
	ok, reason := ShouldReap(labels, now())
	if !ok || reason != ReasonExpired {
		t.Fatalf("expected expired reap, got ok=%v reason=%q", ok, reason)
	}
}

func TestShouldReap_ExactExpiryMomentReaps(t *testing.T) {
	// now == expires must reap (reaper is authoritative; no grace window).
	stamp := now().UTC().Format(time.RFC3339)
	labels := map[string]string{OwnerLabel: OwnerValue, ExpiresLabel: stamp}
	ok, reason := ShouldReap(labels, now())
	if !ok || reason != ReasonExpired {
		t.Fatalf("expected reap at exact expiry, got ok=%v reason=%q", ok, reason)
	}
}

func TestShouldReap_MissingExpiresLabel(t *testing.T) {
	ok, reason := ShouldReap(map[string]string{OwnerLabel: OwnerValue}, now())
	if !ok || reason != ReasonNoExpiry {
		t.Fatalf("expected missing-label reap, got ok=%v reason=%q", ok, reason)
	}
}

func TestShouldReap_UnparsableExpiresLabel(t *testing.T) {
	labels := map[string]string{OwnerLabel: OwnerValue, ExpiresLabel: "not-a-time"}
	ok, reason := ShouldReap(labels, now())
	if !ok || reason != ReasonBadExpiry {
		t.Fatalf("expected bad-label reap, got ok=%v reason=%q", ok, reason)
	}
}

func TestParseExpires_RoundTrip(t *testing.T) {
	want := now().Add(900 * time.Second).UTC()
	got, err := taskconfig.ParseExpires(want.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !got.Equal(want) {
		t.Fatalf("round trip: want %v got %v", want, got)
	}
}
