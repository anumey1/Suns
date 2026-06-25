package operation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anumey1/Suns/pkg/safety/floor"
)

// RepoMaintenanceOp runs safe garbage collection on one git repository (§12.17).
// It is a pure value type so plan.Seal copies it without aliasing.
//
// Reversibility is Recoverable (🟡): a plain `git gc` packs loose objects and
// prunes only beyond the default two-week reflog window, so recently-dropped work
// remains recoverable. The dangerous knobs — `--aggressive` and `--prune=now` —
// are explicit opt-ins (the original "gc --aggressive --prune=now across all
// repos" can permanently drop stashes, reset commits, and abandoned branches).
//
// The critical safety property is enforced at execution time: ValidateAtExec
// re-confirms the working tree is clean and no merge/rebase is in progress
// IMMEDIATELY before acting, so a repo that became dirty after planning is
// skipped rather than collected.
type RepoMaintenanceOp struct {
	Repo       string // absolute repository working-tree path
	Aggressive bool   // git gc --aggressive (opt-in)
	PruneNow   bool   // git gc --prune=now (opt-in; drops the reflog grace window)
}

var _ Operation = RepoMaintenanceOp{}

func (o RepoMaintenanceOp) Kind() OpKind { return KindRepoMaintenance }

func (o RepoMaintenanceOp) Reversibility() Reversibility { return Recoverable }

func (o RepoMaintenanceOp) Describe() Preview {
	return Preview{
		Kind:          KindRepoMaintenance,
		Reversibility: Recoverable,
		Line:          o.gcCommand() + "  (" + o.Repo + ")",
	}
}

// ValidateAtPlan confirms the target is off the deny floor and is a git
// repository at discovery time.
func (o RepoMaintenanceOp) ValidateAtPlan(ctx context.Context) error {
	if err := floor.Check(o.Repo); err != nil {
		return err
	}
	if _, err := os.Lstat(filepath.Join(o.Repo, ".git")); err != nil {
		return fmt.Errorf("not a git repository: %s", o.Repo)
	}
	return nil
}

// ValidateAtExec re-confirms, immediately before acting, that the repo is off the
// floor and its working tree is clean with no merge/rebase in progress. A dirty
// or mid-operation repo returns an error, so safety.Execute skips it (recorded,
// never collected) — the TOCTOU defense for repository GC (§12.17).
func (o RepoMaintenanceOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	if err := floor.Check(o.Repo); err != nil {
		return Identity{}, err
	}
	if clean, reason := repoClean(ctx, o.Repo); !clean {
		return Identity{}, fmt.Errorf("repository not clean: %s", reason)
	}
	return Identity{Kind: KindRepoMaintenance}, nil
}

// Execute runs `git -C <repo> gc [flags]` through the injected runner
// (unprivileged). The deletion mode is inert for this op.
func (o RepoMaintenanceOp) Execute(ctx context.Context, _ Mode, _ Identity) (Receipt, error) {
	r := Receipt{Kind: KindRepoMaintenance, Time: time.Now()}
	args := append([]string{"-C", o.Repo, "gc"}, o.gcFlags()...)
	res, err := getSystemRunner().Run(ctx, false, "git", args...)
	switch {
	case err == nil && res.ExitCode == 0:
		r.Fate, r.Status = "collected", "ok"
	default:
		r.Fate, r.Status, r.Err = "skipped", "failed", err
	}
	return r, nil
}

func (o RepoMaintenanceOp) HistoryRecord(r Receipt) HistoryEntry {
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindRepoMaintenance,
		Reversible: Recoverable,
		Status:     r.Status,
		Fate:       r.Fate,
		Path:       o.Repo,
		Cmd:        o.gcCommand(),
	}
}

func (o RepoMaintenanceOp) gcFlags() []string {
	var f []string
	if o.Aggressive {
		f = append(f, "--aggressive")
	}
	if o.PruneNow {
		f = append(f, "--prune=now")
	}
	return f
}

func (o RepoMaintenanceOp) gcCommand() string {
	return strings.TrimSpace("git gc " + strings.Join(o.gcFlags(), " "))
}

// repoClean reports whether a repository has no uncommitted changes and no
// in-progress merge/rebase/cherry-pick/revert/bisect. It uses the injected runner
// for the porcelain status and reads the in-progress marker files directly from
// the resolved git directory. Any failure to determine state is treated as "not
// clean" so an unverifiable repo is skipped, never collected.
func repoClean(ctx context.Context, repo string) (bool, string) {
	runner := getSystemRunner()

	res, err := runner.Run(ctx, false, "git", "-C", repo, "status", "--porcelain")
	if err != nil || res.ExitCode != 0 {
		return false, "could not read git status"
	}
	if len(bytes.TrimSpace(res.Stdout)) != 0 {
		return false, "uncommitted changes"
	}

	dirRes, err := runner.Run(ctx, false, "git", "-C", repo, "rev-parse", "--absolute-git-dir")
	if err != nil || dirRes.ExitCode != 0 {
		return false, "could not resolve git dir"
	}
	gitDir := strings.TrimSpace(string(dirRes.Stdout))
	for marker, label := range map[string]string{
		"MERGE_HEAD":       "merge in progress",
		"rebase-merge":     "rebase in progress",
		"rebase-apply":     "rebase in progress",
		"CHERRY_PICK_HEAD": "cherry-pick in progress",
		"REVERT_HEAD":      "revert in progress",
		"BISECT_LOG":       "bisect in progress",
	} {
		if _, err := os.Lstat(filepath.Join(gitDir, marker)); err == nil {
			return false, label
		}
	}
	return true, ""
}
