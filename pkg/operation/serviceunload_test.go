package operation_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
)

// fakeSysRunner is a programmable SystemRunner for op tests.
type fakeSysRunner struct {
	mu      sync.Mutex
	calls   [][]string
	handler func(privileged bool, name string, args []string) (operation.RunResult, error)
}

func (f *fakeSysRunner) Run(_ context.Context, privileged bool, name string, args ...string) (operation.RunResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, append([]string{name}, args...))
	f.mu.Unlock()
	return f.handler(privileged, name, args)
}

func (f *fakeSysRunner) lastCall() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1]
}

func useRunner(t *testing.T, h func(bool, string, []string) (operation.RunResult, error)) *fakeSysRunner {
	t.Helper()
	f := &fakeSysRunner{handler: h}
	operation.UseSystemRunner(f)
	t.Cleanup(func() { operation.UseSystemRunner(nil) })
	return f
}

func tempPlist(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "com.example.agent.plist")
	if err := os.WriteFile(p, []byte("<plist/>"), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}
	return p
}

func TestServiceUnload_Success(t *testing.T) {
	plist := tempPlist(t)
	f := useRunner(t, func(_ bool, _ string, _ []string) (operation.RunResult, error) {
		return operation.RunResult{ExitCode: 0}, nil
	})
	op := operation.ServiceUnloadOp{Domain: "gui/501", Label: "com.example.agent", Plist: plist}

	id, err := op.ValidateAtExec(context.Background())
	if err != nil {
		t.Fatalf("ValidateAtExec: %v", err)
	}
	if id.Service.Domain != "gui/501" || id.Service.Label != "com.example.agent" || id.Service.Plist != plist {
		t.Errorf("identity = %+v", id.Service)
	}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, id)
	if r.Fate != "unloaded" || r.Status != "ok" {
		t.Errorf("fate/status = %q/%q, want unloaded/ok", r.Fate, r.Status)
	}
	if got := f.lastCall(); len(got) != 3 || got[0] != "launchctl" || got[1] != "bootout" || got[2] != "gui/501/com.example.agent" {
		t.Errorf("bootout call = %v", got)
	}
}

func TestServiceUnload_AlreadyNotLoaded(t *testing.T) {
	plist := tempPlist(t)
	useRunner(t, func(_ bool, _ string, _ []string) (operation.RunResult, error) {
		return operation.RunResult{ExitCode: 1, Stderr: []byte("Boot-out failed: 3: No such process")}, nil
	})
	op := operation.ServiceUnloadOp{Domain: "gui/501", Label: "x", Plist: plist}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Fate != "unloaded" || r.Status != "ok:not-loaded" {
		t.Errorf("fate/status = %q/%q, want unloaded/ok:not-loaded", r.Fate, r.Status)
	}
}

func TestServiceUnload_NoPrivilegeSkips(t *testing.T) {
	plist := tempPlist(t)
	useRunner(t, func(privileged bool, _ string, _ []string) (operation.RunResult, error) {
		if privileged {
			return operation.RunResult{}, operation.ErrPrivilegeRequired
		}
		return operation.RunResult{}, nil
	})
	op := operation.ServiceUnloadOp{Domain: "system", Label: "x", Plist: plist, Privileged: true}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Fate != "skipped" || r.Status != "skipped:no-privilege" {
		t.Errorf("fate/status = %q/%q, want skipped/skipped:no-privilege", r.Fate, r.Status)
	}
}

func TestServiceUnload_MissingPlistFailsValidation(t *testing.T) {
	op := operation.ServiceUnloadOp{Domain: "gui/501", Label: "x", Plist: filepath.Join(t.TempDir(), "gone.plist")}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Fatal("want error when defining plist is gone")
	}
}

func TestServiceUnload_HistoryRecord(t *testing.T) {
	op := operation.ServiceUnloadOp{Domain: "system", Label: "com.x", Plist: "/Library/LaunchDaemons/com.x.plist"}
	e := op.HistoryRecord(operation.Receipt{Status: "ok"})
	if e.Op != operation.KindServiceUnload || e.Reversible != operation.Recoverable {
		t.Errorf("op/rev = %v/%v", e.Op, e.Reversible)
	}
	if e.Domain != "system" || e.Label != "com.x" || e.Plist == "" {
		t.Errorf("history fields = %+v", e)
	}
}
