package purge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// opPaths returns the target paths of the emitted FileDeleteOps.
func opPaths(res Result) map[string]bool {
	m := map[string]bool{}
	for _, f := range res.Findings {
		m[f.Path] = true
	}
	return m
}

func TestEmptyDirs_Cascade(t *testing.T) {
	root := t.TempDir()
	// a/b/c all empty → maximal root is a (one op subsumes the subtree).
	mkdir(t, filepath.Join(root, "a", "b", "c"))
	// keep/ holds a real file → not empty, never emitted.
	mkdir(t, filepath.Join(root, "keep"))
	touch(t, filepath.Join(root, "keep", "file.txt"))

	res, err := EmptyDirs(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := opPaths(res)
	if !got[filepath.Join(root, "a")] {
		t.Errorf("expected maximal empty root 'a' to be emitted; got %v", got)
	}
	if got[filepath.Join(root, "a", "b")] || got[filepath.Join(root, "a", "b", "c")] {
		t.Errorf("nested dirs must be subsumed by 'a', not emitted separately: %v", got)
	}
	if got[filepath.Join(root, "keep")] {
		t.Errorf("non-empty 'keep' must not be emitted")
	}
	if got[root] {
		t.Errorf("the named root itself must never be emitted")
	}
}

func TestEmptyDirs_DSStoreCountsAsEmpty(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "onlyds")
	mkdir(t, d)
	touch(t, filepath.Join(d, ".DS_Store"))

	res, err := EmptyDirs(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !opPaths(res)[d] {
		t.Errorf("a dir whose only content is .DS_Store must count as empty; got %v", opPaths(res))
	}
}

func TestEmptyDirs_RealFileBlocks(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "hasfile")
	mkdir(t, d)
	touch(t, filepath.Join(d, "keep.txt"))

	res, err := EmptyDirs(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ops) != 0 {
		t.Errorf("a dir with a real file is not empty; got %v", opPaths(res))
	}
}

func TestEmptyDirs_SymlinkBlocks(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "haslink")
	mkdir(t, d)
	if err := os.Symlink("/nowhere", filepath.Join(d, "link")); err != nil {
		t.Fatal(err)
	}
	res, err := EmptyDirs(context.Background(), root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ops) != 0 {
		t.Errorf("a dir containing a symlink must not count as empty; got %v", opPaths(res))
	}
}

func TestBrokenSymlinks_DetectsDangling(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "real.txt")
	touch(t, target)

	good := filepath.Join(root, "good.link")
	if err := os.Symlink(target, good); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(root, "bad.link")
	if err := os.Symlink(filepath.Join(root, "missing.txt"), bad); err != nil {
		t.Fatal(err)
	}

	res, err := BrokenSymlinks(context.Background(), []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := opPaths(res)
	if !got[bad] {
		t.Errorf("dangling symlink must be flagged; got %v", got)
	}
	if got[good] {
		t.Errorf("valid symlink must NOT be flagged; got %v", got)
	}
	if got[target] {
		t.Errorf("the real target file must never be flagged; got %v", got)
	}
}

func TestBrokenSymlinks_NoFollowKeepsTarget(t *testing.T) {
	// A valid symlink pointing at a directory must not cause the walk to follow
	// into it (and must not be flagged).
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	mkdir(t, sub)
	touch(t, filepath.Join(sub, "inner.txt"))
	if err := os.Symlink(sub, filepath.Join(root, "dirlink")); err != nil {
		t.Fatal(err)
	}
	res, err := BrokenSymlinks(context.Background(), []string{root}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ops) != 0 {
		t.Errorf("valid dir symlink must not be flagged; got %v", opPaths(res))
	}
}
