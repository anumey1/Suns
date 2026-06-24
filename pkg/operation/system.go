package operation

import (
	"context"
	"errors"
	"sync"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// ErrPrivilegeRequired means a privileged system action was requested but no
// privilege-capable runner is wired (or elevation was declined). Ops translate
// it into a skipped receipt rather than a hard failure, so a workflow that loses
// privilege degrades gracefully (§6.1).
var ErrPrivilegeRequired = errors.New("operation: privileged system action requires elevation")

// RunResult is the trimmed outcome of a SystemRunner.Run, decoupling the
// operation package from syscmd's concrete Result type at the seam boundary.
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// SystemRunner executes an allowlisted external command for the operation layer,
// elevating when privileged is true. It is the single injection seam through
// which ServiceUnloadOp and ReceiptForgetOp reach launchctl/pkgutil — both for
// unprivileged execution-time validation reads and for the privileged mutation —
// so the ops stay pure value types (mirroring ProcessKiller and Trasher).
type SystemRunner interface {
	Run(ctx context.Context, privileged bool, name string, args ...string) (RunResult, error)
}

// defaultSystemRunner runs unprivileged commands through the hardened syscmd
// allowlist and refuses privileged ones until a privilege-capable runner is
// injected. This keeps discovery and validation working with no elevation while
// making accidental unelevated mutation impossible.
type defaultSystemRunner struct{ r *syscmd.Runner }

func (d defaultSystemRunner) Run(ctx context.Context, privileged bool, name string, args ...string) (RunResult, error) {
	if privileged {
		return RunResult{}, ErrPrivilegeRequired
	}
	res, err := d.r.Run(ctx, name, args...)
	return RunResult{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode}, err
}

var (
	systemRunnerMu sync.RWMutex
	systemRunner   SystemRunner = defaultSystemRunner{r: syscmd.New()}
)

// UseSystemRunner injects the runner used by subsequent ServiceUnloadOp and
// ReceiptForgetOp executions. The app wires a privilege-capable runner (backed by
// the chokepoint) at startup; tests inject a fake; the default handles
// unprivileged reads and refuses privileged mutation.
func UseSystemRunner(s SystemRunner) {
	systemRunnerMu.Lock()
	defer systemRunnerMu.Unlock()
	if s == nil {
		s = defaultSystemRunner{r: syscmd.New()}
	}
	systemRunner = s
}

func getSystemRunner() SystemRunner {
	systemRunnerMu.RLock()
	defer systemRunnerMu.RUnlock()
	return systemRunner
}
