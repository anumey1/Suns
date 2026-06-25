package operation_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
)

// gitRepo creates a temp directory that looks like a git working tree (has a
// .git dir) and returns the work-tree path and the .git dir path.
func gitRepo(t *testing.T) (string, string) {
	t.Helper()
	work := t.TempDir()
	gitDir := filepath.Join(work, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return work, gitDir
}

// gitHandler builds a SystemRunner handler emulating git: a given porcelain
// status output and a fixed git-dir for rev-parse; gc always succeeds. It records
// the gc argv into *gcArgs.
func gitHandler(porcelain, gitDir string, gcArgs *[]string) func(bool, string, []string) (operation.RunResult, error) {
	return func(_ bool, name string, args []string) (operation.RunResult, error) {
		switch {
		case contains(args, "status"):
			return operation.RunResult{Stdout: []byte(porcelain), ExitCode: 0}, nil
		case contains(args, "rev-parse"):
			return operation.RunResult{Stdout: []byte(gitDir + "\n"), ExitCode: 0}, nil
		case contains(args, "gc"):
			if gcArgs != nil {
				*gcArgs = append([]string{name}, args...)
			}
			return operation.RunResult{ExitCode: 0}, nil
		}
		return operation.RunResult{}, nil
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func TestRepoMaintenance_CleanCollects(t *testing.T) {
	work, gitDir := gitRepo(t)
	var gcArgs []string
	useRunner(t, gitHandler("", gitDir, &gcArgs))

	op := operation.RepoMaintenanceOp{Repo: work}
	if err := op.ValidateAtPlan(context.Background()); err != nil {
		t.Fatalf("ValidateAtPlan: %v", err)
	}
	id, err := op.ValidateAtExec(context.Background())
	if err != nil || id.Kind != operation.KindRepoMaintenance {
		t.Fatalf("ValidateAtExec = %v, %v", id, err)
	}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, id)
	if r.Fate != "collected" || r.Status != "ok" {
		t.Errorf("fate/status = %q/%q, want collected/ok", r.Fate, r.Status)
	}
	// Plain gc: no aggressive/prune flags.
	joined := strings.Join(gcArgs, " ")
	if !strings.Contains(joined, "-C "+work+" gc") || strings.Contains(joined, "--aggressive") || strings.Contains(joined, "--prune") {
		t.Errorf("gc argv = %q, want plain gc in %s", joined, work)
	}
}

func TestRepoMaintenance_DirtySkippedAtExec(t *testing.T) {
	work, gitDir := gitRepo(t)
	useRunner(t, gitHandler(" M tracked.go\n", gitDir, nil))

	op := operation.RepoMaintenanceOp{Repo: work}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Error("a repo with uncommitted changes must fail ValidateAtExec")
	}
}

func TestRepoMaintenance_InProgressMergeSkipped(t *testing.T) {
	work, gitDir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(gitDir, "MERGE_HEAD"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	useRunner(t, gitHandler("", gitDir, nil))

	op := operation.RepoMaintenanceOp{Repo: work}
	_, err := op.ValidateAtExec(context.Background())
	if err == nil || !strings.Contains(err.Error(), "merge in progress") {
		t.Errorf("merge-in-progress repo must be skipped, got err=%v", err)
	}
}

func TestRepoMaintenance_OptInFlags(t *testing.T) {
	work, gitDir := gitRepo(t)
	var gcArgs []string
	useRunner(t, gitHandler("", gitDir, &gcArgs))

	op := operation.RepoMaintenanceOp{Repo: work, Aggressive: true, PruneNow: true}
	id, _ := op.ValidateAtExec(context.Background())
	if _, err := op.Execute(context.Background(), operation.ModeTrash, id); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(gcArgs, " ")
	if !strings.Contains(joined, "--aggressive") || !strings.Contains(joined, "--prune=now") {
		t.Errorf("gc argv = %q, want --aggressive --prune=now", joined)
	}
}

func TestRepoMaintenance_HistoryRecord(t *testing.T) {
	op := operation.RepoMaintenanceOp{Repo: "/x/repo", Aggressive: true}
	e := op.HistoryRecord(operation.Receipt{Status: "ok", Fate: "collected"})
	if e.Op != operation.KindRepoMaintenance || e.Reversible != operation.Recoverable {
		t.Errorf("op/rev = %v/%v", e.Op, e.Reversible)
	}
	if e.Path != "/x/repo" || !strings.Contains(e.Cmd, "--aggressive") {
		t.Errorf("history fields = %+v", e)
	}
}
