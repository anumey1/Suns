package cli

import (
	"fmt"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/uninstaller"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/trash"
	"github.com/spf13/cobra"
)

// newNukeCmd builds `suns nuke <app>` — the precision uninstaller.
func newNukeCmd() *cobra.Command {
	var (
		dryRun    bool
		yes       bool
		deathstar bool
		jarjar    string
	)
	cmd := &cobra.Command{
		Use:   "nuke <app>",
		Short: "Precision-uninstall an app: bundle, support files, launch agents, receipts",
		Long: `Nuke traces an application by its bundle identifier and previews a frozen plan
that removes the bundle, its ~/Library support files, related launch agents, and
(for .pkg-installed apps) its sole-owned payload — then forgets the installer
receipt. Files claimed by more than one package are RETAINED as shared
dependencies so unrelated apps are never bricked.

Safe by default: it moves files to the Trash and shows the gate before acting.
Forgetting receipts and unloading system daemons require an admin password (asked
once, at execution). This is not a "complete uninstall" — see the scope notes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			res, err := uninstaller.Plan(ctx, args[0], uninstaller.Options{})
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "Application: %s\n", res.App)
			fmt.Fprintf(out, "Bundle ID:   %s\n", res.BundleID)
			if len(res.Ops) == 0 {
				fmt.Fprintln(out, "Nothing to remove was found.")
				return nil
			}

			p := plan.New(res.Ops).Seal()
			groups := safety.BuildGroups(p, mode)
			fmt.Fprint(out, safety.Render(groups, 30))

			if len(res.Retained) > 0 {
				fmt.Fprintln(out, "Retained (shared dependencies — claimed by other packages):")
				for _, r := range res.Retained {
					fmt.Fprintf(out, "    · %s\n", r)
				}
			}
			fmt.Fprintln(out, "Scope — nuke does NOT comprehensively handle:")
			for _, b := range res.Bounds {
				fmt.Fprintf(out, "    · %s\n", b)
			}
			fmt.Fprintf(out, "Deletion mode: %s\n", state.DeletionMode())

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}

			immediate := state.ConfirmMode() || yes
			if !immediate {
				if !confirm(cmd.InOrStdin(), out, "Nuke it?") {
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
			var reclaimed int64
			for _, r := range safety.Execute(ctx, p, mode) {
				if appendErr := log.Append(r.Entry); appendErr != nil {
					fmt.Fprintf(out, "warning: could not record history: %v\n", appendErr)
				}
				switch r.Receipt.Fate {
				case "", "skipped":
					skipped++
				default:
					done++
					if r.Op.Kind() == operation.KindFileDelete {
						reclaimed += r.Op.Describe().Bytes
					}
				}
			}
			fmt.Fprintf(out, "Done: %d actions · %s reclaimed", done, humanBytes(reclaimed))
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
