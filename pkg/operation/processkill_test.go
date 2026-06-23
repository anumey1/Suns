package operation_test

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// spawnSleep starts a child `sleep` we own, so kill tests never touch anything
// but our own process.
func spawnSleep(t *testing.T) (*exec.Cmd, identity.ProcessIdent) {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})
	// Give gopsutil a moment to see it.
	var id identity.ProcessIdent
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, _, _, err := procctl.Current(cmd.Process.Pid)
		if err == nil && got.Exec != "" {
			id = got
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if id.PID == 0 {
		id, _, _, _ = procctl.Current(cmd.Process.Pid)
		id.PID = cmd.Process.Pid
	}
	return cmd, id
}

func TestProcessKillOp_KillsOwnProcess(t *testing.T) {
	cmd, id := spawnSleep(t)
	op := operation.ProcessKillOp{PID: id.PID, Name: "sleep", Expect: id, Signal: int(syscall.SIGKILL)}

	if _, err := op.ValidateAtExec(context.Background()); err != nil {
		t.Fatalf("ValidateAtExec on live process: %v", err)
	}
	r, err := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if r.Fate != "killed" {
		t.Fatalf("Fate = %q (status %q), want killed", r.Fate, r.Status)
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done: // process exited (was killed)
	case <-time.After(2 * time.Second):
		t.Fatal("process was not killed")
	}

	if rec := op.HistoryRecord(r); rec.Op != operation.KindProcessKill || rec.Reversible != operation.Irreversible {
		t.Fatalf("bad history record: %+v", rec)
	}
}

// A stale identity (wrong birth time) must be refused without signalling — the
// PID-reuse defense (§4.7).
func TestProcessKillOp_RefusesStaleIdentity(t *testing.T) {
	cmd, id := spawnSleep(t)
	stale := id
	stale.Birth = id.Birth.Add(-time.Hour) // pretend a different, older process

	op := operation.ProcessKillOp{PID: id.PID, Name: "sleep", Expect: stale, Signal: int(syscall.SIGKILL)}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Fatal("ValidateAtExec accepted a stale identity")
	}

	// Execute via the killer must also refuse and NOT kill the live process.
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Fate == "killed" {
		t.Fatal("killed a process whose identity did not match")
	}
	// The process should still be alive.
	if _, _, _, err := procctl.Current(cmd.Process.Pid); err != nil {
		t.Fatalf("process unexpectedly gone: %v", err)
	}
}

// Without an injected privilege-capable killer, a privileged target is skipped
// (no-privilege), never force-killed.
func TestProcessKillOp_PrivilegedSkippedByDefault(t *testing.T) {
	cmd, id := spawnSleep(t)
	op := operation.ProcessKillOp{PID: id.PID, Name: "sleep", Expect: id, Signal: int(syscall.SIGKILL), Privileged: true}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Status != "skipped:no-privilege" {
		t.Fatalf("status = %q, want skipped:no-privilege", r.Status)
	}
	if _, _, _, err := procctl.Current(cmd.Process.Pid); err != nil {
		t.Fatalf("process unexpectedly killed: %v", err)
	}
}
