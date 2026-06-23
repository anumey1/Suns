package cli

import (
	"fmt"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/scanner"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/trash"
	"github.com/spf13/cobra"
)

func newCleanCmd() *cobra.Command {
	var (
		dryRun       bool
		yes          bool
		deathstar    bool
		jarjar       string
		includeOptIn bool
	)
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Curated, safe cleanup of dev caches (safe-cache allowlist)",
		Long: `Clean discovers targets from the curated safe-cache allowlist, previews a
frozen plan, and (after you confirm) trashes them. Safe by default: it moves
files to the Trash and shows the gate before acting. Nothing outside the
allowlist is ever touched, and the deny floor can never be bypassed.`,
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

			manifest, err := scanner.LoadSafeCacheManifest()
			if err != nil {
				return err
			}
			res, err := scanner.Discover(ctx, manifest.Targets, scanner.Options{IncludeOptIn: includeOptIn})
			if err != nil {
				return err
			}
			if len(res.Ops) == 0 {
				fmt.Fprintln(out, "Nothing to clean — no allowlisted caches present.")
				return nil
			}

			p := plan.New(res.Ops).Seal()
			groups := safety.BuildGroups(p, mode)
			fmt.Fprint(out, safety.Render(groups, 20))
			fmt.Fprintf(out, "Deletion mode: %s\n", state.DeletionMode())

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}

			// confirm_mode off (default) shows the gate and requires approval;
			// --deathstar (confirm_mode on) or --yes execute immediately (§4.3).
			immediate := state.ConfirmMode() || yes
			if !immediate {
				if !confirm(cmd.InOrStdin(), out, "Proceed?") {
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

			// One Trasher for the whole batch so the volume-scoped circuit
			// breaker spans the run (§4.4).
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
				case "trashed", "obliterated":
					done++
					reclaimed += r.Op.Describe().Bytes
				default:
					skipped++
				}
			}

			verb := "Trashed"
			if mode == operation.ModeObliterate {
				verb = "Obliterated"
			}
			fmt.Fprintf(out, "%s %d items · %s reclaimed", verb, done, humanBytes(reclaimed))
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
	cmd.Flags().BoolVar(&includeOptIn, "include-optin", false, "include expensive opt-in caches (e.g. iOS DeviceSupport)")
	return cmd
}

// opMode maps the deletion-mode machine key to the operation.Mode.
func opMode(deletionMode string) operation.Mode {
	if deletionMode == config.DeletionObliterate {
		return operation.ModeObliterate
	}
	return operation.ModeTrash
}
