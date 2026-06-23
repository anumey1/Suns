// Package procadmin provides the privilege-capable ProcessKiller wired into the
// app at startup (§4.7, §6.4).
//
// Own-user processes are validated-and-signalled directly (no elevation).
// Root/other-user processes are delegated to a re-invocation of suns under
// sudo — `sudo -n suns __killproc …` — which performs the atomic
// validate-and-kill AS ROOT in a single process, so there is no TOCTOU between
// the privileged identity read and the signal. It relies on a cached sudo
// ticket (acquired interactively beforehand via the chokepoint / the TUI's `e`
// elevate); with no ticket, `sudo -n` fails and Kill returns ErrNeedsElevation.
package procadmin

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

const sudoPath = "/usr/bin/sudo"

// KillProcSubcommand is the hidden CLI subcommand name that performs the
// privileged validate-and-kill. Exit codes: 0 ok; ExitMismatch identity
// changed; ExitGone target vanished; anything else ⇒ treat as no-ticket.
const KillProcSubcommand = "__killproc"

const (
	ExitMismatch = 10
	ExitGone     = 11
)

// Killer is the privilege-capable ProcessKiller.
type Killer struct{ self string }

// New returns a Killer that re-invokes the current executable under sudo for
// privileged targets.
func New() *Killer {
	self, _ := os.Executable()
	return &Killer{self: self}
}

// Kill implements operation.ProcessKiller.
func (k *Killer) Kill(ctx context.Context, req operation.ProcessKillRequest) error {
	if !req.Privileged {
		return procctl.ValidateAndSignal(req.Expect, syscall.Signal(req.Signal))
	}
	if k.self == "" {
		return operation.ErrNeedsElevation
	}
	args := []string{
		"-n", k.self, KillProcSubcommand,
		strconv.Itoa(req.PID),
		strconv.FormatInt(req.Expect.Birth.UnixNano(), 10),
		req.Expect.Exec,
		strconv.Itoa(req.Signal),
	}
	cmd := exec.CommandContext(ctx, sudoPath, args...)
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			switch ee.ExitCode() {
			case ExitMismatch:
				return identity.ErrIdentityMismatch
			case ExitGone:
				return operation.ErrNeedsElevation // target gone; surface as skip
			}
		}
		// sudo -n with no cached ticket exits 1: needs interactive elevation.
		return operation.ErrNeedsElevation
	}
	return nil
}
