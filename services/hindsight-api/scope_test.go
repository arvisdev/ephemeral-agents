package main

import "testing"

func TestNormalizeScope(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{"", ScopeSession, false},     // empty defaults to private
		{"session", "session", false}, // exact
		{"cross-task", "cross-task", false},
		{"session ", "session", true}, // not normalized silently: caller must trim
		{"global", "", true},
		{"SESSION", "", true},
		{"cross_task", "", true}, // the scope-rule mismatch trap from §6
	}
	for _, c := range cases {
		got, err := NormalizeScope(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeScope(%q): want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizeScope(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizeScope(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeScopeErrorMentionsValidValues(t *testing.T) {
	_, err := NormalizeScope("global")
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{"session", "cross-task", "global"} {
		if !contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestCrossTaskVisible(t *testing.T) {
	cases := []struct {
		write, read string
		want        bool
	}{
		// The G2 property: task 1's cross-task write is visible to task 2.
		{ScopeCrossTask, ScopeCrossTask, true},
		// Session writes stay private, even against a cross-task reader.
		{ScopeSession, ScopeCrossTask, false},
		{ScopeCrossTask, ScopeSession, false},
		{ScopeSession, ScopeSession, false},
	}
	for _, c := range cases {
		if got := CrossTaskVisible(c.write, c.read); got != c.want {
			t.Errorf("CrossTaskVisible(%q,%q) = %v, want %v", c.write, c.read, got, c.want)
		}
	}
}

func TestPlanRead(t *testing.T) {
	cases := []struct {
		scope, query     string
		wantExact, wantX bool
	}{
		{ScopeSession, "", true, false},
		{ScopeSession, "hello", true, false}, // session readers never hit the shared pool
		{ScopeCrossTask, "", true, false},    // no query: exact rows only
		{ScopeCrossTask, "hello", true, true},
	}
	for _, c := range cases {
		p := PlanRead(c.scope, c.query)
		if p.ExactScope != c.wantExact || p.CrossTask != c.wantX {
			t.Errorf("PlanRead(%q,%q) = %+v, want exact=%v cross=%v",
				c.scope, c.query, p, c.wantExact, c.wantX)
		}
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
