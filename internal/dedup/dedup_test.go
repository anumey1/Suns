package dedup

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/anumey1/Suns/pkg/operation"
)

// write creates a file at dir/name with the given content, making parent dirs.
func write(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// offered returns the set of all paths the report would touch (keepers excluded
// — only the deletable members and their ops).
func offered(rep Report) map[string]bool {
	set := map[string]bool{}
	for _, op := range rep.Ops {
		set[op.(operation.FileDeleteOp).Path] = true
	}
	return set
}

func find(t *testing.T, roots ...string) Report {
	t.Helper()
	rep, err := Find(context.Background(), roots, Options{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	return rep
}

func TestFind_TrueDuplicates(t *testing.T) {
	dir := t.TempDir()
	// >4 KB so all three passes (size, head, full) are exercised.
	dup := bytes.Repeat([]byte("carbon-copy-"), 500)
	write(t, dir, "a.bin", dup)
	write(t, dir, "b.bin", dup)
	write(t, dir, "c.bin", dup)
	write(t, dir, "unique.bin", append([]byte("different"), dup...))

	rep := find(t, dir)
	if len(rep.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(rep.Groups))
	}
	g := rep.Groups[0]
	if len(g.Deletable) != 2 {
		t.Fatalf("want 2 deletable, got %d (%v)", len(g.Deletable), g.Deletable)
	}
	if len(rep.Ops) != 2 {
		t.Fatalf("want 2 ops, got %d", len(rep.Ops))
	}
	if rep.ReclaimableEst != int64(len(dup))*2 {
		t.Errorf("reclaimable = %d, want %d", rep.ReclaimableEst, len(dup)*2)
	}
	if !rep.CloneCaveat {
		t.Errorf("CloneCaveat should be set when deletions are present")
	}
	if offered(rep)[filepath.Join(dir, "unique.bin")] {
		t.Errorf("unique file must not be offered")
	}
}

func TestFind_SameSizeDifferentContent(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "x.bin", bytes.Repeat([]byte("A"), 8192))
	write(t, dir, "y.bin", bytes.Repeat([]byte("B"), 8192)) // same size, different bytes
	rep := find(t, dir)
	if len(rep.Groups) != 0 {
		t.Fatalf("want 0 groups for distinct content, got %d", len(rep.Groups))
	}
}

func TestFind_HardlinkNeverOffered(t *testing.T) {
	dir := t.TempDir()
	orig := write(t, dir, "orig.bin", bytes.Repeat([]byte("x"), 5000))
	if err := os.Link(orig, filepath.Join(dir, "hard.bin")); err != nil {
		t.Skipf("hardlink unsupported: %v", err)
	}
	rep := find(t, dir)
	// orig and hard are the same inode → collapsed to one candidate → no duplicate.
	if len(rep.Groups) != 0 || len(rep.Ops) != 0 {
		t.Fatalf("hardlinks must not be offered: groups=%d ops=%d", len(rep.Groups), len(rep.Ops))
	}
}

func TestFind_BundleAtomic(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("bundle"), 1000)
	// Identical files INSIDE a bundle must never be deduped.
	write(t, dir, "Foo.app/Contents/a.bin", content)
	write(t, dir, "Foo.app/Contents/b.bin", content)
	// Identical files OUTSIDE the bundle are normal duplicates.
	write(t, dir, "top1.bin", content)
	write(t, dir, "top2.bin", content)

	rep := find(t, dir)
	off := offered(rep)
	for p := range off {
		if strings.Contains(p, "Foo.app"+string(os.PathSeparator)) {
			t.Errorf("bundle interior offered: %s", p)
		}
	}
	// The two top-level copies form exactly one group with one deletable.
	if len(rep.Groups) != 1 || len(rep.Ops) != 1 {
		t.Fatalf("want top-level dup only: groups=%d ops=%d", len(rep.Groups), len(rep.Ops))
	}
}

func TestFind_KeeperHeuristic(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("keep-me"), 1000)
	cache := write(t, dir, "Caches/x.bin", content)
	doc := write(t, dir, "Documents/x.bin", content)
	_ = cache

	rep := find(t, dir)
	if len(rep.Groups) != 1 {
		t.Fatalf("want 1 group, got %d", len(rep.Groups))
	}
	g := rep.Groups[0]
	if g.Keeper != doc {
		t.Errorf("keeper = %s, want the Documents copy %s", g.Keeper, doc)
	}
	if len(g.Deletable) != 1 || !strings.Contains(g.Deletable[0], "Caches") {
		t.Errorf("the Caches copy should be the deletable one, got %v", g.Deletable)
	}
}

func TestFind_XattrNormalization(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("quarantined"), 1000)
	a := write(t, dir, "a.bin", content)
	write(t, dir, "b.bin", content)
	// Tag one copy with a cosmetic quarantine xattr.
	if err := unix.Setxattr(a, "com.apple.quarantine", []byte("0081;deadbeef;Safari;"), 0); err != nil {
		t.Skipf("xattr unsupported on this fs: %v", err)
	}
	rep := find(t, dir)
	if len(rep.Groups) != 1 {
		t.Fatalf("cosmetic xattr must not break duplicate detection: groups=%d", len(rep.Groups))
	}
	if !rep.Groups[0].XattrDiffer {
		t.Errorf("XattrDiffer should flag the cosmetic difference")
	}
}

func TestFind_CloneNotExcluded(t *testing.T) {
	dir := t.TempDir()
	content := bytes.Repeat([]byte("clone-lineage"), 1000)
	orig := write(t, dir, "orig.bin", content)
	clone := filepath.Join(dir, "clone.bin")
	if err := unix.Clonefile(orig, clone, 0); err != nil {
		t.Skipf("clonefile unsupported on this fs (need APFS): %v", err)
	}
	// A clone shares blocks but is a real duplicate by content — it must NOT be
	// excluded (the key APFS correction, §12.1).
	rep := find(t, dir)
	if len(rep.Groups) != 1 || len(rep.Ops) != 1 {
		t.Fatalf("APFS clone wrongly excluded: groups=%d ops=%d", len(rep.Groups), len(rep.Ops))
	}
}

func TestFind_SymlinkNeverFollowed(t *testing.T) {
	dir := t.TempDir()
	target := write(t, dir, "target.bin", bytes.Repeat([]byte("t"), 5000))
	if err := os.Symlink(target, filepath.Join(dir, "link.bin")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	rep := find(t, dir)
	// The symlink is not followed, so only one candidate exists → no duplicate.
	if len(rep.Groups) != 0 || len(rep.Ops) != 0 {
		t.Fatalf("symlink must not be followed/offered: groups=%d ops=%d", len(rep.Groups), len(rep.Ops))
	}
}

func TestFind_MinSizeAndZeroByteSkipped(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "empty1.bin", nil)
	write(t, dir, "empty2.bin", nil)
	rep := find(t, dir)
	if len(rep.Ops) != 0 {
		t.Fatalf("zero-byte files must never be offered: ops=%d", len(rep.Ops))
	}
}
