package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/purge"
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
		scheduled    bool
	)
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Curated, safe cleanup of dev caches (safe-cache allowlist)",
		Long: `Clean discovers targets from the curated safe-cache allowlist, previews a
frozen plan, and (after you confirm) trashes them. Safe by default: it moves
files to the Trash and shows the gate before acting. Nothing outside the
allowlist is ever touched, and the deny floor can never be bypassed.

--scheduled runs the most-constrained unattended mode (used by the launchd agent
that "suns schedule" installs): locked to the allowlist, deletion forced to
trash, all interactive flags and config ignored, and a scheduled_run record
written to history.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if scheduled {
				return runScheduledClean(ctx, out)
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
	cmd.Flags().BoolVar(&scheduled, "scheduled", false, "constrained unattended mode for the launchd agent (forces trash, no prompts)")
	cmd.AddCommand(newCleanEmptyDirsCmd())
	cmd.AddCommand(newCleanBrokenSymlinksCmd())
	return cmd
}

// newCleanEmptyDirsCmd builds `suns clean empty-dirs <path>` — the path-scoped
// Empty Directory Purger (§12.19). It always confirms the named scope before
// acting, independent of confirm_mode (only --yes, an explicit acknowledgement,
// bypasses it).
func newCleanEmptyDirsCmd() *cobra.Command {
	var (
		dryRun bool
		yes    bool
		jarjar string
	)
	cmd := &cobra.Command{
		Use:   "empty-dirs <path>",
		Short: "Purge empty directories under a path (.DS_Store-only counts as empty)",
		Long: `Empty-dirs removes empty directories under the named path, cascading upward so a
directory emptied by removing its children is caught in the same pass. A
directory whose only content is a .DS_Store is treated as empty and removed with
it. The named path itself is never removed. You are always asked to confirm the
scope before anything is trashed.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := purge.EmptyDirs(cmd.Context(), args[0], purge.Options{})
			if err != nil {
				return err
			}
			return runPurge(cmd, res, jarjar, dryRun, yes,
				fmt.Sprintf("Purge %d empty dirs under %s?", len(res.Ops), args[0]),
				"empty directories")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the frozen plan and exit without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the scope confirmation")
	cmd.Flags().StringVar(&jarjar, "jarjar", "trash", "deletion mode: trash|obliterate")
	return cmd
}

// newCleanBrokenSymlinksCmd builds `suns clean broken-symlinks <path...>` — the
// destroy half of the Broken Symlink Auditor (§12.18); the read-only audit half
// is `suns scan`.
func newCleanBrokenSymlinksCmd() *cobra.Command {
	var (
		dryRun bool
		yes    bool
		jarjar string
	)
	cmd := &cobra.Command{
		Use:   "broken-symlinks <path...>",
		Short: "Remove dangling symlinks (whose target no longer exists) under paths",
		Long: `Broken-symlinks removes symlinks under the named paths whose target no longer
exists. Walks are no-follow; only the dangling link is removed, never anything it
points through. You are always asked to confirm the scope before anything is
trashed.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := purge.BrokenSymlinks(cmd.Context(), args, purge.Options{})
			if err != nil {
				return err
			}
			return runPurge(cmd, res, jarjar, dryRun, yes,
				fmt.Sprintf("Remove %d broken symlinks?", len(res.Ops)),
				"broken symlinks")
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the frozen plan and exit without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the scope confirmation")
	cmd.Flags().StringVar(&jarjar, "jarjar", "trash", "deletion mode: trash|obliterate")
	return cmd
}

// runPurge renders the gate for a path-scoped purge, confirms the scope (always,
// unless --yes), then executes the FileDeleteOps through the trasher and records
// history. It is the shared tail of the empty-dirs and broken-symlinks
// subcommands, mirroring the main clean execute path.
func runPurge(cmd *cobra.Command, res purge.Result, jarjar string, dryRun, yes bool, scopeQuestion, noun string) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	if len(res.Ops) == 0 {
		fmt.Fprintf(out, "No %s found.\n", noun)
		for _, s := range res.Skipped {
			fmt.Fprintf(out, "  skipped: %s\n", s)
		}
		return nil
	}

	mode := opMode(jarjar)
	p := plan.New(res.Ops).Seal()
	groups := safety.BuildGroups(p, mode)
	fmt.Fprint(out, safety.Render(groups, 20))
	if len(res.Skipped) > 0 {
		fmt.Fprintf(out, "Skipped %d unreadable/protected entries.\n", len(res.Skipped))
	}

	if dryRun {
		fmt.Fprintln(out, "(dry run — no changes made)")
		return nil
	}

	// The scope confirmation is mandatory and independent of confirm_mode: only an
	// explicit --yes bypasses it (§12.19).
	if !yes {
		if !confirm(cmd.InOrStdin(), out, scopeQuestion) {
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
		case "trashed", "obliterated":
			done++
		default:
			skipped++
		}
	}
	verb := "Trashed"
	if mode == operation.ModeObliterate {
		verb = "Obliterated"
	}
	fmt.Fprintf(out, "%s %d %s", verb, done, noun)
	if skipped > 0 {
		fmt.Fprintf(out, " · %d skipped", skipped)
	}
	fmt.Fprintln(out)
	return nil
}

// opMode maps the deletion-mode machine key to the operation.Mode.
func opMode(deletionMode string) operation.Mode {
	if deletionMode == config.DeletionObliterate {
		return operation.ModeObliterate
	}
	return operation.ModeTrash
}

// runScheduledClean is the most-constrained unattended path (§12.20), invoked by
// the launchd agent as `suns clean --scheduled`. It ignores all interactive flags
// and config: deletion is FORCED to trash (never obliterate unattended), the
// scope is locked to the curated safe-cache allowlist, and it never prompts. The
// Trasher's pure-Go ~/.Trash fallback applies in a headless context and skips any
// target whose trashability cannot be guaranteed rather than escalating to a
// permanent delete. Every run writes a scheduled_run history record whose status
// (ok|partial|failed) is surfaced prominently — never a silent failure.
func runScheduledClean(ctx context.Context, out io.Writer) error {
	const mode = operation.ModeTrash

	logPath, err := history.DefaultPath()
	if err != nil {
		return err
	}
	log, err := history.Open(logPath)
	if err != nil {
		return err
	}

	record := func(status, summary string) {
		entry := operation.HistoryEntry{
			TS:         time.Now(),
			Op:         operation.KindScheduledRun,
			Reversible: operation.Reversible,
			Status:     status,
			Summary:    summary,
			Cmd:        "clean --scheduled",
		}
		_ = log.Append(entry)
		marker := "scheduled clean"
		if status != "ok" {
			marker = "⚠ scheduled clean " + status
		}
		fmt.Fprintf(out, "%s: %s\n", marker, summary)
	}

	manifest, err := scanner.LoadSafeCacheManifest()
	if err != nil {
		record("failed", "could not load safe-cache allowlist: "+err.Error())
		return nil
	}
	res, err := scanner.Discover(ctx, manifest.Targets, scanner.Options{IncludeOptIn: false})
	if err != nil {
		record("failed", "discovery failed: "+err.Error())
		return nil
	}
	if len(res.Ops) == 0 {
		record("ok", "nothing to clean")
		return nil
	}

	tr, err := trash.New()
	if err != nil {
		record("failed", "trash unavailable: "+err.Error())
		return nil
	}
	operation.UseTrasher(tr)

	p := plan.New(res.Ops).Seal()
	var done, skipped, failed int
	var reclaimed int64
	for _, r := range safety.Execute(ctx, p, mode) {
		_ = log.Append(r.Entry)
		switch {
		case r.Receipt.Fate == "trashed":
			done++
			reclaimed += r.Op.Describe().Bytes
		case r.Err != nil || r.Receipt.Status == "failed":
			failed++
		default:
			skipped++
		}
	}

	status := "ok"
	switch {
	case failed > 0 && done == 0:
		status = "failed"
	case failed > 0 || skipped > 0:
		status = "partial"
	}
	record(status, fmt.Sprintf("%d trashed, %d skipped, %d failed · %s reclaimed",
		done, skipped, failed, humanBytes(reclaimed)))
	return nil
}
