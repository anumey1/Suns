package maintain

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// Runner is the unprivileged executor for git. Production uses syscmd.New();
// tests inject a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Repo is one discovered repository and its assessed state, for the pre-action
// listing (§12.17: "list each with estimated savings before acting").
type Repo struct {
	Path         string `json:"path"`
	SavingsBytes int64  `json:"savings_bytes"`
	Clean        bool   `json:"clean"`
	Reason       string `json:"reason,omitempty"` // why it will be skipped, when not clean
}

// Result is the outcome of discovery.
type Result struct {
	Repos []Repo                `json:"repos"`
	Ops   []operation.Operation `json:"-"` // one per clean repo
}

// Options controls the maintenance flags applied to the emitted ops.
type Options struct {
	Aggressive bool
	PruneNow   bool
}

// Discover walks the roots for git repositories, assesses each, and builds a
// RepoMaintenanceOp for every clean one. It is read-only and honors ctx
// cancellation. A repository directory is not descended into beyond locating its
// .git, so a working tree is assessed once.
func Discover(ctx context.Context, r Runner, roots []string, opts Options) (Result, error) {
	var res Result
	seen := map[string]bool{}

	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil || d == nil {
				return nil // tolerate unreadable subtrees
			}
			if !d.IsDir() || d.Name() != ".git" {
				return nil
			}
			repo := filepath.Dir(p)
			if !seen[repo] {
				seen[repo] = true
				res.add(ctx, r, repo, opts)
			}
			return filepath.SkipDir // don't descend into the .git directory
		})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func (res *Result) add(ctx context.Context, r Runner, repo string, opts Options) {
	rp := Repo{Path: repo, SavingsBytes: estimateSavings(ctx, r, repo)}
	rp.Clean, rp.Reason = assessClean(ctx, r, repo)
	res.Repos = append(res.Repos, rp)
	if rp.Clean {
		res.Ops = append(res.Ops, operation.RepoMaintenanceOp{
			Repo:       repo,
			Aggressive: opts.Aggressive,
			PruneNow:   opts.PruneNow,
		})
	}
}

// estimateSavings reads `git count-objects -v` and estimates reclaimable bytes as
// the loose-object size plus the garbage size (both reported in KiB) — what a gc
// would pack and prune. It degrades to 0 on any trouble.
func estimateSavings(ctx context.Context, r Runner, repo string) int64 {
	res, err := r.Run(ctx, "git", "-C", repo, "count-objects", "-v")
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	var sizeKiB, garbageKiB int64
	for _, ln := range strings.Split(string(res.Stdout), "\n") {
		key, val, ok := strings.Cut(ln, ":")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(val), 10, 64)
		switch strings.TrimSpace(key) {
		case "size":
			sizeKiB = n
		case "size-garbage":
			garbageKiB = n
		}
	}
	return (sizeKiB + garbageKiB) * 1024
}

// assessClean reports whether a repository is safe to collect: no uncommitted
// changes and no merge/rebase/cherry-pick/revert/bisect in progress. Anything it
// cannot verify is reported not-clean with a reason, so the repo is listed but
// not collected.
func assessClean(ctx context.Context, r Runner, repo string) (bool, string) {
	res, err := r.Run(ctx, "git", "-C", repo, "status", "--porcelain")
	if err != nil || res.ExitCode != 0 {
		return false, "could not read git status"
	}
	if len(bytes.TrimSpace(res.Stdout)) != 0 {
		return false, "uncommitted changes"
	}

	dirRes, err := r.Run(ctx, "git", "-C", repo, "rev-parse", "--absolute-git-dir")
	if err != nil || dirRes.ExitCode != 0 {
		return false, "could not resolve git dir"
	}
	gitDir := strings.TrimSpace(string(dirRes.Stdout))
	for marker, label := range inProgressMarkers {
		if _, err := os.Lstat(filepath.Join(gitDir, marker)); err == nil {
			return false, label
		}
	}
	return true, ""
}

// inProgressMarkers maps a git-dir marker to the operation it indicates.
var inProgressMarkers = map[string]string{
	"MERGE_HEAD":       "merge in progress",
	"rebase-merge":     "rebase in progress",
	"rebase-apply":     "rebase in progress",
	"CHERRY_PICK_HEAD": "cherry-pick in progress",
	"REVERT_HEAD":      "revert in progress",
	"BISECT_LOG":       "bisect in progress",
}
