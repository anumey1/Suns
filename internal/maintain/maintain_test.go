package maintain

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// fakeRunner emulates git for a set of repos keyed by working-tree path.
type fakeRunner struct {
	status map[string]string // repo → porcelain output
	gitDir map[string]string // repo → absolute git dir
}

func (f fakeRunner) Run(_ context.Context, _ string, args ...string) (syscmd.Result, error) {
	repo := argAfter(args, "-C")
	switch {
	case has(args, "count-objects"):
		return syscmd.Result{Stdout: []byte("count: 5\nsize: 100\nsize-garbage: 20\n"), ExitCode: 0}, nil
	case has(args, "status"):
		return syscmd.Result{Stdout: []byte(f.status[repo]), ExitCode: 0}, nil
	case has(args, "rev-parse"):
		return syscmd.Result{Stdout: []byte(f.gitDir[repo] + "\n"), ExitCode: 0}, nil
	}
	return syscmd.Result{}, nil
}

func has(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func argAfter(ss []string, flag string) string {
	for i, s := range ss {
		if s == flag && i+1 < len(ss) {
			return ss[i+1]
		}
	}
	return ""
}

func mkRepo(t *testing.T, root, name string) string {
	t.Helper()
	repo := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestDiscover_ClassifiesRepos(t *testing.T) {
	root := t.TempDir()
	clean := mkRepo(t, root, "clean")
	dirty := mkRepo(t, root, "dirty")
	merging := mkRepo(t, root, "merging")
	// merging has an in-progress merge marker in its git dir.
	if err := os.WriteFile(filepath.Join(merging, ".git", "MERGE_HEAD"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	fr := fakeRunner{
		status: map[string]string{clean: "", dirty: " M file.go\n", merging: ""},
		gitDir: map[string]string{
			clean:   filepath.Join(clean, ".git"),
			dirty:   filepath.Join(dirty, ".git"),
			merging: filepath.Join(merging, ".git"),
		},
	}

	res, err := Discover(context.Background(), fr, []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Repos) != 3 {
		t.Fatalf("want 3 repos discovered, got %d", len(res.Repos))
	}

	byPath := map[string]Repo{}
	for _, r := range res.Repos {
		byPath[r.Path] = r
	}
	if !byPath[clean].Clean {
		t.Errorf("clean repo should be Clean")
	}
	if byPath[clean].SavingsBytes != (100+20)*1024 {
		t.Errorf("savings = %d, want %d", byPath[clean].SavingsBytes, (100+20)*1024)
	}
	if byPath[dirty].Clean || byPath[dirty].Reason != "uncommitted changes" {
		t.Errorf("dirty repo = %+v, want not-clean/uncommitted", byPath[dirty])
	}
	if byPath[merging].Clean || byPath[merging].Reason != "merge in progress" {
		t.Errorf("merging repo = %+v, want not-clean/merge", byPath[merging])
	}

	// Exactly one op, for the clean repo only.
	if len(res.Ops) != 1 {
		t.Fatalf("want 1 op (clean only), got %d", len(res.Ops))
	}
}

func TestDiscover_AppliesOptInFlags(t *testing.T) {
	root := t.TempDir()
	clean := mkRepo(t, root, "clean")
	fr := fakeRunner{
		status: map[string]string{clean: ""},
		gitDir: map[string]string{clean: filepath.Join(clean, ".git")},
	}
	res, err := Discover(context.Background(), fr, []string{root}, Options{Aggressive: true, PruneNow: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ops) != 1 {
		t.Fatalf("want 1 op, got %d", len(res.Ops))
	}
	// The op's preview must reflect the opt-in flags.
	line := res.Ops[0].Describe().Line
	if !strings.Contains(line, "--aggressive") || !strings.Contains(line, "--prune=now") {
		t.Errorf("op describe = %q, want aggressive+prune", line)
	}
}
