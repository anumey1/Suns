package cli

import (
	"fmt"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/orphans"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/trash"
	"github.com/spf13/cobra"
)

// newOrphansCmd builds `suns orphans` — the orphaned launch-agent purge.
func newOrphansCmd() *cobra.Command {
	var (
		dryRun    bool
		yes       bool
		deathstar bool
		jarjar    string
	)
	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Find and remove launch agents whose executable is gone",
		Long: `Orphans scans your launchd directories for jobs whose program no longer exists
on disk, and previews a frozen plan that boots each one out and trashes its
plist. Conservative by design: Apple-managed jobs are skipped, and jobs launched
via shell scripts or relative/PATH-resolved programs are left alone because their
real binary cannot be resolved.

Safe by default: it moves plists to the Trash and shows the gate before acting.
This reports, it does not guarantee — see the scope notes.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			ov := config.Overrides{}
			if deathstar {
				on := true
				ov.ConfirmMode = &on
			}
			if cmd.Flags().Changed("jarjar") {
				ov.DeletionMode = &jarjar
			}
			state, err := config.Load(ov)
			if err != nil {
				return err
			}
			mode := opMode(state.DeletionMode())

			report, err := orphans.Find(ctx, orphans.Options{})
			if err != nil {
				return err
			}
			if len(report.Ops) == 0 {
				fmt.Fprintln(out, "No orphaned launch agents found.")
				return nil
			}

			fmt.Fprintf(out, "Found %d orphaned launch agent(s):\n", len(report.Orphans))
			for _, o := range report.Orphans {
				fmt.Fprintf(out, "    · %s\n      label %s · missing exec %s\n", o.Plist, o.Label, o.MissingExec)
			}
			fmt.Fprintln(out)

			p := plan.New(report.Ops).Seal()
			groups := safety.BuildGroups(p, mode)
			fmt.Fprint(out, safety.Render(groups, 30))
			fmt.Fprintln(out, "Scope — orphans reports, it does not guarantee:")
			for _, b := range report.Bounds {
				fmt.Fprintf(out, "    · %s\n", b)
			}
			fmt.Fprintf(out, "Deletion mode: %s\n", state.DeletionMode())

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}

			immediate := state.ConfirmMode() || yes
			if !immediate {
				if !confirm(cmd.InOrStdin(), out, "Purge these orphans?") {
					fmt.Fprintln(out, "Aborted. Nothing was changed.")
					return nil
				}
			}

			logPath, err := history.DefaultPath()
			if err != nil {
				return err
			}
			log, err := history.Open(logPath)
			if err != nil {
				return err
			}

			tr, err := trash.New()
			if err != nil {
				return err
			}
			operation.UseTrasher(tr)

			var done, skipped int
			for _, r := range safety.Execute(ctx, p, mode) {
				if appendErr := log.Append(r.Entry); appendErr != nil {
					fmt.Fprintf(out, "warning: could not record history: %v\n", appendErr)
				}
				switch r.Receipt.Fate {
				case "", "skipped":
					skipped++
				default:
					done++
				}
			}
			fmt.Fprintf(out, "Done: %d actions", done)
			if skipped > 0 {
				fmt.Fprintf(out, " · %d skipped", skipped)
			}
			fmt.Fprintln(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the frozen plan and exit without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "bypass the confirmation gate")
	cmd.Flags().BoolVar(&deathstar, "deathstar", false, "confirm_mode on: execute immediately (no gate)")
	cmd.Flags().StringVar(&jarjar, "jarjar", "trash", "deletion mode: trash|obliterate")
	return cmd
}
