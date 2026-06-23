package cli

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/anumey1/Suns/internal/restore"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	var (
		all bool
		yes bool
	)
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Undo trashed files from the operation history (identity-checked)",
		Long: `Restore reverses Reversible file_delete records from the operation history,
verifying each trashed object against its recorded identity before moving it
back to its original path. With no flags it lists what can be restored; use
--all to restore everything eligible.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			logPath, err := history.DefaultPath()
			if err != nil {
				return err
			}
			entries, err := history.ReadAll(logPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					fmt.Fprintln(out, "No operation history yet — nothing to restore.")
					return nil
				}
				return err
			}

			cands := restore.Candidates(entries)
			if len(cands) == 0 {
				fmt.Fprintln(out, "Nothing to restore.")
				return nil
			}

			if !all {
				fmt.Fprintf(out, "%d restorable item(s):\n", len(cands))
				for i, e := range cands {
					fmt.Fprintf(out, "  %2d. %s  (%s)\n", i+1, e.OrigPath, humanBytes(e.Size))
				}
				fmt.Fprintln(out, "\nRun 'suns restore --all' to restore them.")
				return nil
			}

			if !yes {
				if !confirm(cmd.InOrStdin(), out, fmt.Sprintf("Restore %d item(s)?", len(cands))) {
					fmt.Fprintln(out, "Aborted.")
					return nil
				}
			}

			var restored, skipped int
			for _, e := range cands {
				o := restore.Restore(e)
				if o.Restored {
					restored++
					if o.Path != e.OrigPath {
						fmt.Fprintf(out, "  restored (renamed): %s\n", o.Path)
					} else {
						fmt.Fprintf(out, "  restored: %s\n", o.Path)
					}
				} else {
					skipped++
					fmt.Fprintf(out, "  skipped:  %s — %s\n", e.OrigPath, o.Reason)
				}
			}
			fmt.Fprintf(out, "Restored %d · skipped %d\n", restored, skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "restore all eligible items (otherwise just list)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}
