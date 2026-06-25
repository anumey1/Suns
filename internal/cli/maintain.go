package cli

import (
	"fmt"

	"github.com/anumey1/Suns/internal/maintain"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/syscmd"
	"github.com/spf13/cobra"
)

// newMaintainCmd builds `suns maintain [path...]` — the de-fanged Git Repository
// Garbage Collector (§12.17). It lists every discovered repo with its estimated
// savings and cleanliness BEFORE acting, gates the batch, and re-confirms each
// repo's cleanliness at execution. The default is a plain `git gc` (two-week
// reflog window); --aggressive and --prune-now are explicit opt-ins.
func newMaintainCmd() *cobra.Command {
	var (
		dryRun     bool
		yes        bool
		aggressive bool
		pruneNow   bool
	)
	cmd := &cobra.Command{
		Use:   "maintain [path...]",
		Short: "Safe git gc across repositories (skips dirty/in-progress repos)",
		Long: `Maintain discovers git repositories under the given paths (default: current
directory), estimates the space each would reclaim, and runs a safe garbage
collection on the clean ones. Repositories with uncommitted changes or an
in-progress merge/rebase are listed and skipped, never collected.

The default is a plain "git gc" with git's normal two-week reflog grace window.
--aggressive and --prune-now drop that safety margin and can permanently discard
recently-abandoned work; they are opt-in and warned about per repo.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			roots := args
			if len(roots) == 0 {
				roots = []string{"."}
			}

			res, err := maintain.Discover(ctx, syscmd.New(), roots, maintain.Options{
				Aggressive: aggressive,
				PruneNow:   pruneNow,
			})
			if err != nil {
				return err
			}
			if len(res.Repos) == 0 {
				fmt.Fprintln(out, "No git repositories found.")
				return nil
			}

			// List every repo with savings + status BEFORE acting (§12.17).
			fmt.Fprintf(out, "Repositories (%d):\n", len(res.Repos))
			var willAct int
			for _, r := range res.Repos {
				if r.Clean {
					willAct++
					fmt.Fprintf(out, "  ✓ %-50s ~%s\n", r.Path, humanBytes(r.SavingsBytes))
				} else {
					fmt.Fprintf(out, "  ⤫ %-50s skip: %s\n", r.Path, r.Reason)
				}
			}

			if aggressive || pruneNow {
				fmt.Fprintln(out, "\n⚠  --aggressive/--prune-now drop git's reflog grace window; recently")
				fmt.Fprintln(out, "   dropped stashes, reset commits, and abandoned branches may be lost")
				fmt.Fprintln(out, "   permanently in every repo above.")
			}

			if len(res.Ops) == 0 {
				fmt.Fprintln(out, "\nNothing to collect — no clean repositories.")
				return nil
			}

			p := plan.New(res.Ops).Seal()
			groups := safety.BuildGroups(p, operation.ModeTrash)
			fmt.Fprint(out, "\n")
			fmt.Fprint(out, safety.Render(groups, 30))

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}
			if !yes {
				if !confirm(cmd.InOrStdin(), out, fmt.Sprintf("Run %s on %d clean repos?", gcLabel(aggressive, pruneNow), willAct)) {
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

			var done, skipped int
			for _, r := range safety.Execute(ctx, p, operation.ModeTrash) {
				if appendErr := log.Append(r.Entry); appendErr != nil {
					fmt.Fprintf(out, "warning: could not record history: %v\n", appendErr)
				}
				if r.Receipt.Fate == "collected" {
					done++
				} else {
					skipped++
				}
			}
			fmt.Fprintf(out, "Collected %d repos", done)
			if skipped > 0 {
				fmt.Fprintf(out, " · %d skipped (became dirty or failed)", skipped)
			}
			fmt.Fprintln(out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list repos and the frozen plan without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "bypass the confirmation gate")
	cmd.Flags().BoolVar(&aggressive, "aggressive", false, "git gc --aggressive (slower; rewrites packs)")
	cmd.Flags().BoolVar(&pruneNow, "prune-now", false, "git gc --prune=now (drops the two-week reflog grace window)")
	return cmd
}

// gcLabel renders the gc form for the confirmation prompt.
func gcLabel(aggressive, pruneNow bool) string {
	s := "git gc"
	if aggressive {
		s += " --aggressive"
	}
	if pruneNow {
		s += " --prune=now"
	}
	return s
}
