package cli

import (
	"encoding/json"
	"fmt"

	"github.com/anumey1/Suns/internal/purge"
	"github.com/spf13/cobra"
)

// newScanCmd builds `suns scan <path...>` — the read-only filesystem-hygiene
// audit: it lists broken symlinks (§12.18) and empty directories (§12.19) under
// the given roots without ever deleting. The destroy halves live behind the gate
// as `suns clean broken-symlinks` / `suns clean empty-dirs`.
func newScanCmd() *cobra.Command {
	var (
		asJSON     bool
		brokenOnly bool
		emptyOnly  bool
	)
	cmd := &cobra.Command{
		Use:   "scan <path...>",
		Short: "Audit broken symlinks and empty directories under paths (read-only)",
		Long: `Scan walks the given paths and reports filesystem hygiene findings — dangling
symlinks (whose target no longer exists) and empty directories (including ones
that contain only a .DS_Store) — without deleting anything. Use
"suns clean broken-symlinks <path>" / "suns clean empty-dirs <path>" to remove
them behind the safety gate.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			type report struct {
				Broken  []purge.Finding `json:"broken_symlinks"`
				Empty   []purge.Finding `json:"empty_dirs"`
				Skipped []string        `json:"skipped,omitempty"`
			}
			var rep report

			if !emptyOnly {
				res, err := purge.BrokenSymlinks(ctx, args, purge.Options{})
				if err != nil {
					return err
				}
				rep.Broken = res.Findings
				rep.Skipped = append(rep.Skipped, res.Skipped...)
			}
			if !brokenOnly {
				for _, root := range args {
					res, err := purge.EmptyDirs(ctx, root, purge.Options{})
					if err != nil {
						rep.Skipped = append(rep.Skipped, fmt.Sprintf("%s: %v", root, err))
						continue
					}
					rep.Empty = append(rep.Empty, res.Findings...)
					rep.Skipped = append(rep.Skipped, res.Skipped...)
				}
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			fmt.Fprintf(out, "Broken symlinks (%d):\n", len(rep.Broken))
			for _, f := range rep.Broken {
				fmt.Fprintf(out, "  %s  %s\n", f.Path, f.Detail)
			}
			fmt.Fprintf(out, "Empty directories (%d):\n", len(rep.Empty))
			for _, f := range rep.Empty {
				fmt.Fprintf(out, "  %s\n", f.Path)
			}
			if len(rep.Skipped) > 0 {
				fmt.Fprintf(out, "Skipped (%d unreadable/protected):\n", len(rep.Skipped))
				for _, s := range rep.Skipped {
					fmt.Fprintf(out, "  %s\n", s)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit findings as JSON")
	cmd.Flags().BoolVar(&brokenOnly, "broken-symlinks", false, "only audit broken symlinks")
	cmd.Flags().BoolVar(&emptyOnly, "empty-dirs", false, "only audit empty directories")
	return cmd
}
