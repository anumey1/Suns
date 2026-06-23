// Package cli holds the Cobra command definitions (§8, §9).
//
// The organizing principle is the separation of destructive from read-only
// commands: only destructive commands route through the safety gate and emit
// typed operations into a frozen plan; read-only commands never prompt and
// never delete.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/procadmin"
	"github.com/anumey1/Suns/internal/tui"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/spf13/cobra"
)

// Version is the build version, overridable via -ldflags at release time.
var Version = "0.0.0-dev"

// NewRootCmd builds the command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "suns",
		Short: "Suns — safe-by-default macOS system utility (Burn It All)",
		Long: `Suns — Super User Nutcase Sessions (Burn It All)

A safety-obsessed macOS cleanup, uninstall, audit, and telemetry utility.
Safe by default: a first run with no flags can never permanently destroy data.

The interactive TUI (run with no subcommand) and the get-coffee dashboard land
in a later Phase 1 slice; this build ships the headless safety core.`,
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// No subcommand → launch the interactive TUI (§9.3).
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := config.Load(config.Overrides{})
			if err != nil {
				return err
			}
			return tui.Run(cmd.Context(), state)
		},
	}
	// Wire the privilege-capable process killer for ProcessKill operations
	// (own-user direct; root/other-user delegated under sudo, §4.7).
	operation.UseProcessKiller(procadmin.New())

	root.AddCommand(newDoctorCmd())
	root.AddCommand(newCleanCmd())
	root.AddCommand(newRestoreCmd())
	root.AddCommand(newGetCoffeeCmd())
	root.AddCommand(newPsCmd())
	root.AddCommand(newKillProcCmd())
	return root
}

// Execute runs the CLI and returns a process exit code.
func Execute(ctx context.Context) int {
	root := NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "suns:", err)
		return 1
	}
	return 0
}

// confirm prompts the operator with a [y/N] question, defaulting to No. It is
// the CLI embodiment of the safety gate's confirmation step (§4.3); the TUI
// uses a modal for the same decision.
func confirm(in io.Reader, out io.Writer, question string) bool {
	fmt.Fprintf(out, "%s [y/N]: ", question)
	sc := bufio.NewScanner(in)
	if !sc.Scan() {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(sc.Text()))
	return answer == "y" || answer == "yes"
}

// humanBytes formats a byte count for human-facing summaries.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
