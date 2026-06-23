package cli

import (
	"errors"
	"os"
	"strconv"
	"syscall"
	"time"

	"github.com/anumey1/Suns/internal/procadmin"
	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/spf13/cobra"
)

// newKillProcCmd is the hidden machine subcommand the privileged killer
// re-invokes under sudo: it performs the atomic validate-and-kill AS ROOT in a
// single process (§4.7). It is not for human use; it speaks in exit codes.
func newKillProcCmd() *cobra.Command {
	return &cobra.Command{
		Use:    procadmin.KillProcSubcommand + " <pid> <birthUnixNano> <exec> <signum>",
		Hidden: true,
		Args:   cobra.ExactArgs(4),
		RunE: func(_ *cobra.Command, args []string) error {
			pid, _ := strconv.Atoi(args[0])
			nano, _ := strconv.ParseInt(args[1], 10, 64)
			exe := args[2]
			signum, _ := strconv.Atoi(args[3])

			expect := identity.ProcessIdent{PID: pid, Birth: time.Unix(0, nano), Exec: exe}
			err := procctl.ValidateAndSignal(expect, syscall.Signal(signum))
			switch {
			case err == nil:
				os.Exit(0)
			case errors.Is(err, identity.ErrIdentityMismatch):
				os.Exit(procadmin.ExitMismatch)
			default:
				os.Exit(procadmin.ExitGone)
			}
			return nil
		},
	}
}
