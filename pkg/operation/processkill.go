package operation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// ErrNeedsElevation means a process is owned by root or another user and the
// current killer cannot validate-and-signal it without elevation (§4.7).
var ErrNeedsElevation = errors.New("operation: process kill requires elevation")

// ProcessKillOp is the concrete ProcessKill operation (§4.2, §12.8). It is a
// pure value type so plan.Seal copies it without aliasing.
type ProcessKillOp struct {
	PID        int
	Name       string
	Expect     identity.ProcessIdent // identity recorded at plan time (PID reuse defense)
	Signal     int                   // syscall.Signal value (SIGTERM/SIGKILL)
	Privileged bool                  // owned by root/another user → must be delegated
}

var _ Operation = ProcessKillOp{}

func (o ProcessKillOp) Kind() OpKind { return KindProcessKill }

// Reversibility is always Irreversible: a killed process is gone, regardless of
// the deletion axis (which is inert for non-file operations, §4.3).
func (o ProcessKillOp) Reversibility() Reversibility { return Irreversible }

func (o ProcessKillOp) Describe() Preview {
	return Preview{
		Kind:          KindProcessKill,
		Reversibility: Irreversible,
		Line:          fmt.Sprintf("PID %d  %s  (%s)", o.PID, o.Name, signalName(o.Signal)),
	}
}

// ValidateAtPlan confirms the process exists at discovery time.
func (o ProcessKillOp) ValidateAtPlan(ctx context.Context) error {
	_, _, _, err := procctl.Current(o.PID)
	return err
}

// ValidateAtExec re-reads the process identity and refuses if the PID now
// refers to a different process (birth time / exec path changed) — the PID
// reuse defense (§4.7).
func (o ProcessKillOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	cur, _, _, err := procctl.Current(o.PID)
	if err != nil {
		return Identity{}, err
	}
	if !cur.Birth.Equal(o.Expect.Birth) || cur.Exec != o.Expect.Exec {
		return Identity{}, fmt.Errorf("%w: PID %d", identity.ErrIdentityMismatch, o.PID)
	}
	return Identity{Kind: KindProcessKill, Process: cur}, nil
}

// Execute delegates to the injected ProcessKiller, which validates-and-signals
// atomically (own-user directly, root/other-user under elevation). The deletion
// mode is inert here.
func (o ProcessKillOp) Execute(ctx context.Context, _ Mode, _ Identity) (Receipt, error) {
	r := Receipt{Kind: KindProcessKill, Time: time.Now()}
	err := getProcessKiller().Kill(ctx, ProcessKillRequest{
		PID: o.PID, Expect: o.Expect, Signal: o.Signal, Privileged: o.Privileged,
	})
	switch {
	case err == nil:
		r.Fate, r.Status = "killed", "ok"
	case errors.Is(err, ErrNeedsElevation):
		r.Fate, r.Status = "skipped", "skipped:no-privilege"
	case errors.Is(err, identity.ErrIdentityMismatch):
		r.Fate, r.Status = "skipped", "skipped:identity"
	default:
		r.Fate, r.Status, r.Err = "skipped", "failed", err
	}
	return r, nil
}

func (o ProcessKillOp) HistoryRecord(r Receipt) HistoryEntry {
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindProcessKill,
		Reversible: Irreversible,
		Status:     r.Status,
		PID:        o.PID,
		Name:       o.Name,
		Birth:      o.Expect.Birth.Format(time.RFC3339),
		Exec:       o.Expect.Exec,
		Signal:     signalName(o.Signal),
	}
}

// ProcessKillRequest is what a ProcessKiller acts on.
type ProcessKillRequest struct {
	PID        int
	Expect     identity.ProcessIdent
	Signal     int
	Privileged bool
}

// ProcessKiller validates-and-signals a process. Implementations may elevate
// (the privilege chokepoint) for root/other-user targets.
type ProcessKiller interface {
	Kill(ctx context.Context, req ProcessKillRequest) error
}

// ownUserKiller is the default: it kills own-user processes directly and
// refuses privileged ones (until a privilege-capable killer is injected).
type ownUserKiller struct{}

func (ownUserKiller) Kill(_ context.Context, req ProcessKillRequest) error {
	if req.Privileged {
		return ErrNeedsElevation
	}
	return procctl.ValidateAndSignal(req.Expect, syscall.Signal(req.Signal))
}

var (
	killerMu sync.RWMutex
	killer   ProcessKiller = ownUserKiller{}
)

// UseProcessKiller injects the killer used by subsequent ProcessKillOp.Execute
// calls. The app wires a privilege-capable killer at startup; tests and the
// own-user fast path use the default.
func UseProcessKiller(k ProcessKiller) {
	killerMu.Lock()
	defer killerMu.Unlock()
	if k == nil {
		k = ownUserKiller{}
	}
	killer = k
}

func getProcessKiller() ProcessKiller {
	killerMu.RLock()
	defer killerMu.RUnlock()
	return killer
}

func signalName(sig int) string {
	switch syscall.Signal(sig) {
	case syscall.SIGTERM:
		return "SIGTERM"
	case syscall.SIGKILL:
		return "SIGKILL"
	case syscall.SIGINT:
		return "SIGINT"
	case syscall.SIGHUP:
		return "SIGHUP"
	default:
		return fmt.Sprintf("sig%d", sig)
	}
}
