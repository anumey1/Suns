package audit

import (
	"context"
	"testing"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// A representative `log show --style json` capture: one success and three
// failures by "bob" within a minute (a burst), plus a denied escalation.
const logFixture = `[
  {"timestamp":"2026-06-24 09:00:00.000000-0700","process":"sudo","eventMessage":"alice : TTY=ttys000 ; PWD=/Users/alice ; USER=root ; COMMAND=/usr/bin/whoami"},
  {"timestamp":"2026-06-24 09:01:00.000000-0700","process":"sudo","eventMessage":"bob : 1 incorrect password attempt ; TTY=ttys001"},
  {"timestamp":"2026-06-24 09:01:20.000000-0700","process":"sudo","eventMessage":"bob : 1 incorrect password attempt ; TTY=ttys001"},
  {"timestamp":"2026-06-24 09:01:40.000000-0700","process":"sudo","eventMessage":"bob : 1 incorrect password attempt ; TTY=ttys001"},
  {"timestamp":"2026-06-24 09:05:00.000000-0700","process":"sudo","eventMessage":"carol : user NOT in the sudoers file. This incident will be reported."}
]`

type fakeLogRunner struct{ out string }

func (f fakeLogRunner) Run(_ context.Context, _ string, _ ...string) (syscmd.Result, error) {
	return syscmd.Result{Stdout: []byte(f.out)}, nil
}

func TestAuthLog_ClassifiesAndCountsFailures(t *testing.T) {
	rep, err := AuthLog(context.Background(), fakeLogRunner{out: logFixture}, LogOptions{})
	if err != nil {
		t.Fatalf("AuthLog: %v", err)
	}
	if len(rep.Events) != 5 {
		t.Fatalf("events = %d, want 5", len(rep.Events))
	}
	// 3 incorrect-password failures + 1 denied = 4.
	if rep.Failures != 4 {
		t.Errorf("failures = %d, want 4", rep.Failures)
	}
	if rep.Events[0].Outcome != "success" || rep.Events[0].User != "alice" {
		t.Errorf("first event = %+v, want alice/success", rep.Events[0])
	}
	if rep.Window != "1d" {
		t.Errorf("default window = %q, want 1d", rep.Window)
	}
}

func TestAuthLog_DetectsRapidBurst(t *testing.T) {
	rep, _ := AuthLog(context.Background(), fakeLogRunner{out: logFixture}, LogOptions{})
	if len(rep.Bursts) != 1 {
		t.Fatalf("bursts = %d, want 1 (bob)", len(rep.Bursts))
	}
	b := rep.Bursts[0]
	if b.User != "bob" || b.Count != 3 {
		t.Errorf("burst = %+v, want bob×3", b)
	}
}

func TestAuthLog_DeniedClassification(t *testing.T) {
	rep, _ := AuthLog(context.Background(), fakeLogRunner{out: logFixture}, LogOptions{})
	var carol AuthEvent
	for _, e := range rep.Events {
		if e.User == "carol" {
			carol = e
		}
	}
	if carol.Outcome != "denied" {
		t.Errorf("carol outcome = %q, want denied", carol.Outcome)
	}
}

func TestAuthLog_NonJSONDegrades(t *testing.T) {
	rep, err := AuthLog(context.Background(), fakeLogRunner{out: "log: predicate error"}, LogOptions{Since: "6h"})
	if err != nil {
		t.Fatalf("should not error on non-JSON: %v", err)
	}
	if len(rep.Events) != 0 || rep.Window != "6h" {
		t.Errorf("expected empty report with window 6h, got %+v", rep)
	}
}

func TestClassifyAuth(t *testing.T) {
	cases := map[string]string{
		"alice : TTY=ttys000 ; COMMAND=/bin/ls":  "success",
		"bob : 2 incorrect password attempts":    "failure",
		"eve : authentication failure":           "failure",
		"mallory : user NOT in the sudoers file": "denied",
		"trent : command not allowed":            "denied",
		"random unrelated message":               "info",
	}
	for msg, want := range cases {
		if got := classifyAuth(msg); got != want {
			t.Errorf("classifyAuth(%q) = %q, want %q", msg, got, want)
		}
	}
}
