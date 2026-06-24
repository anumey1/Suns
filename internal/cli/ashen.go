package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/dedup"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/trash"
	"github.com/spf13/cobra"
)

// newAshenCmd builds `suns ashen` — the hash-based duplicate finder.
//
// Naming: ash is largely carbon, and burnt "Carbon Copies" come to rest in an
// ashen state. Duplicate files are carbon copies; ashen burns the redundant ones
// down to a single keeper. The `dedup` alias preserves the spec name (§12.1).
func newAshenCmd() *cobra.Command {
	var (
		dryRun    bool
		yes       bool
		deathstar bool
		jarjar    string
		minSize   int64
	)
	cmd := &cobra.Command{
		Use:     "ashen [path...]",
		Aliases: []string{"dedup"},
		Short:   "Burn duplicate \"carbon copies\" down to one keeper (hash-based dedup)",
		Long: `Ashen finds byte-identical duplicate files under the given paths (default: the
current directory) and previews a frozen plan that keeps one copy per group and
trashes the rest. Safe by default: it moves duplicates to the Trash and shows the
gate before acting.

Detection is exact (size → 4 KB head hash → full SHA-256). Hardlinks are never
offered, bundles (.app, .rtfd, …) are treated atomically, and symlinks are never
followed. The keeper is chosen automatically (preferring user-document over
cache/download/temp locations); review the keep/burn split below before
confirming — interactive keeper adjustment is a TUI feature.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			roots, err := resolveRoots(args)
			if err != nil {
				return err
			}

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

			for _, r := range roots {
				fmt.Fprintf(out, "Scanning %s\n", r)
			}
			report, err := dedup.Find(ctx, roots, dedup.Options{MinSize: minSize})
			if err != nil {
				return err
			}
			if len(report.Ops) == 0 {
				fmt.Fprintln(out, "No duplicates found.")
				return nil
			}

			renderGroups(out, report)

			p := plan.New(report.Ops).Seal()
			groups := safety.BuildGroups(p, mode)
			fmt.Fprint(out, safety.Render(groups, 20))
			fmt.Fprintf(out, "Deletion mode: %s\n", state.DeletionMode())
			if report.CloneCaveat {
				fmt.Fprintln(out, "Note: space freed may be lower than shown for any files that share APFS clone blocks.")
			}

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}

			immediate := state.ConfirmMode() || yes
			if !immediate {
				if !confirm(cmd.InOrStdin(), out, "Burn the duplicates?") {
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
			fmt.Fprintf(out, "%s %d duplicates · %s reclaimed", verb, done, humanBytes(reclaimed))
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
	cmd.Flags().Int64Var(&minSize, "min-size", 1, "ignore files smaller than this many bytes")
	return cmd
}

// resolveRoots cleans the path arguments to absolute paths, defaulting to the
// current working directory when none are given. Each must exist.
func resolveRoots(args []string) ([]string, error) {
	if len(args) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		args = []string{cwd}
	}
	roots := make([]string, 0, len(args))
	for _, a := range args {
		abs, err := filepath.Abs(a)
		if err != nil {
			return nil, err
		}
		if _, err := os.Stat(abs); err != nil {
			return nil, fmt.Errorf("ashen: %w", err)
		}
		roots = append(roots, filepath.Clean(abs))
	}
	return roots, nil
}

// renderGroups prints the keep/burn split per duplicate group before the gate.
func renderGroups(out io.Writer, report dedup.Report) {
	fmt.Fprintf(out, "Found %d duplicate group(s):\n", len(report.Groups))
	for _, g := range report.Groups {
		fmt.Fprintf(out, "  · %s each, %d copies\n", humanBytes(g.Size), len(g.Deletable)+1)
		fmt.Fprintf(out, "    keep 🟢 %s\n", g.Keeper)
		for _, d := range g.Deletable {
			fmt.Fprintf(out, "    burn 🔥 %s\n", d)
		}
		if g.XattrDiffer {
			fmt.Fprintln(out, "    (note: copies differ only in cosmetic metadata — quarantine/tags/where-from)")
		}
	}
	fmt.Fprintln(out)
}
