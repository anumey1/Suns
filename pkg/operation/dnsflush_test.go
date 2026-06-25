package operation_test

import (
	"context"
	"strings"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
)

// flushCall records one runner invocation for order/privilege assertions.
type flushCall struct {
	privileged bool
	line       string
}

func TestDNSFlush_Success(t *testing.T) {
	var calls []flushCall
	useRunner(t, func(privileged bool, name string, args []string) (operation.RunResult, error) {
		calls = append(calls, flushCall{privileged, strings.Join(append([]string{name}, args...), " ")})
		return operation.RunResult{ExitCode: 0}, nil
	})

	op := operation.DNSFlushOp{}
	if err := op.ValidateAtPlan(context.Background()); err != nil {
		t.Fatalf("ValidateAtPlan: %v", err)
	}
	id, err := op.ValidateAtExec(context.Background())
	if err != nil || id.Kind != operation.KindDNSFlush {
		t.Fatalf("ValidateAtExec = %v, %v", id, err)
	}

	r, _ := op.Execute(context.Background(), operation.ModeTrash, id)
	if r.Fate != "flushed" || r.Status != "ok" {
		t.Errorf("fate/status = %q/%q, want flushed/ok", r.Fate, r.Status)
	}

	// Both commands must run, in order, and both elevated.
	if len(calls) != 2 {
		t.Fatalf("want 2 calls, got %d: %+v", len(calls), calls)
	}
	if calls[0].line != "dscacheutil -flushcache" {
		t.Errorf("first call = %q", calls[0].line)
	}
	if calls[1].line != "killall -HUP mDNSResponder" {
		t.Errorf("second call = %q", calls[1].line)
	}
	for i, c := range calls {
		if !c.privileged {
			t.Errorf("call %d (%q) must be privileged", i, c.line)
		}
	}
}

func TestDNSFlush_NoPrivilegeSkips(t *testing.T) {
	var calls int
	useRunner(t, func(_ bool, _ string, _ []string) (operation.RunResult, error) {
		calls++
		return operation.RunResult{}, operation.ErrPrivilegeRequired
	})

	r, _ := op0().Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Fate != "skipped" || r.Status != "skipped:no-privilege" {
		t.Errorf("fate/status = %q/%q, want skipped/skipped:no-privilege", r.Fate, r.Status)
	}
	// The first command failing on privilege must short-circuit the second.
	if calls != 1 {
		t.Errorf("want 1 call before short-circuit, got %d", calls)
	}
}

func TestDNSFlush_SecondCommandFailureIsFailure(t *testing.T) {
	useRunner(t, func(_ bool, name string, _ []string) (operation.RunResult, error) {
		if name == "killall" {
			return operation.RunResult{ExitCode: 1}, nil // non-zero exit, no error
		}
		return operation.RunResult{ExitCode: 0}, nil
	})
	r, _ := op0().Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Status != "failed" {
		t.Errorf("status = %q, want failed", r.Status)
	}
}

func TestDNSFlush_ModeIsInert(t *testing.T) {
	// Reversibility must read Irreversible regardless of deletion mode.
	if (operation.DNSFlushOp{}).Reversibility() != operation.Irreversible {
		t.Error("DNSFlushOp must be Irreversible")
	}
	if (operation.DNSFlushOp{}).Describe().Reversibility != operation.Irreversible {
		t.Error("Describe must report Irreversible")
	}
}

func TestDNSFlush_HistoryRecord(t *testing.T) {
	e := operation.DNSFlushOp{}.HistoryRecord(operation.Receipt{Status: "ok", Fate: "flushed"})
	if e.Op != operation.KindDNSFlush || e.Reversible != operation.Irreversible {
		t.Errorf("op/rev = %v/%v", e.Op, e.Reversible)
	}
	if e.Cmd == "" || e.Fate != "flushed" {
		t.Errorf("history fields = %+v", e)
	}
}

func op0() operation.DNSFlushOp { return operation.DNSFlushOp{} }
