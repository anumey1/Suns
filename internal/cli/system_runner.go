package cli

import (
	"context"
	"errors"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/privilege"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// elevatingRunner is the production operation.SystemRunner: it runs unprivileged
// commands through the hardened syscmd allowlist and routes privileged ones
// through the single privilege chokepoint (§6). A declined or expired sudo ticket
// is mapped to operation.ErrPrivilegeRequired so the dependent op records a
// graceful skip rather than aborting the workflow (§6.1).
type elevatingRunner struct {
	plain *syscmd.Runner
	choke *privilege.Chokepoint
}

func newElevatingRunner() elevatingRunner {
	return elevatingRunner{
		plain: syscmd.New(),
		choke: privilege.New(privilege.TerminalPrompter{}),
	}
}

func (e elevatingRunner) Run(ctx context.Context, privileged bool, name string, args ...string) (operation.RunResult, error) {
	if !privileged {
		res, err := e.plain.Run(ctx, name, args...)
		return operation.RunResult{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode}, err
	}
	res, err := e.choke.Run(ctx, name, args...)
	out := operation.RunResult{Stdout: res.Stdout, Stderr: res.Stderr, ExitCode: res.ExitCode}
	if errors.Is(err, privilege.ErrCanceled) || errors.Is(err, privilege.ErrExpired) || errors.Is(err, privilege.ErrActionNotAllowed) {
		return out, operation.ErrPrivilegeRequired
	}
	return out, err
}
