package trash

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTrash_RenamesIntoTrashDir(t *testing.T) {
	root := t.TempDir()
	trashDir := filepath.Join(root, "Trash")
	tr, err := NewWithDir(trashDir)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "file.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := tr.Trash(context.Background(), src)
	if res.Skipped {
		t.Fatalf("unexpected skip: %s", res.Reason)
	}
	if res.Method != MethodRename {
		t.Fatalf("Method = %q, want %q (same-volume fallback)", res.Method, MethodRename)
	}
	if _, err := os.Lstat(src); !os.IsNotExist(err) {
		t.Fatalf("source still present after trash: %v", err)
	}
	got, err := os.ReadFile(res.TrashPath)
	if err != nil || string(got) != "payload" {
		t.Fatalf("trashed content mismatch: %q err=%v", got, err)
	}
}

func TestTrash_CollisionGetsUniqueName(t *testing.T) {
	root := t.TempDir()
	trashDir := filepath.Join(root, "Trash")
	tr, err := NewWithDir(trashDir)
	if err != nil {
		t.Fatal(err)
	}
	write := func(p string) {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := filepath.Join(root, "dup.txt")
	b := filepath.Join(root, "sub", "dup.txt")
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		t.Fatal(err)
	}
	write(a)
	write(b)

	r1 := tr.Trash(context.Background(), a)
	r2 := tr.Trash(context.Background(), b)
	if r1.Skipped || r2.Skipped {
		t.Fatalf("unexpected skip: %v / %v", r1.Reason, r2.Reason)
	}
	if r1.TrashPath == r2.TrashPath {
		t.Fatalf("collision not disambiguated: both went to %q", r1.TrashPath)
	}
}

// copyThenUnlink is the cross-volume fallback. We exercise its logic directly
// (a real EXDEV needs a second mount) and assert it copies the content and
// removes the source — never deleting the source without a successful copy.
func TestCopyThenUnlink_PreservesContentRemovesSource(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "tree")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "f.txt"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(root, "Trash", "tree")
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := copyThenUnlink(src, dst); err != nil {
		t.Fatalf("copyThenUnlink: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "sub", "f.txt"))
	if err != nil || string(got) != "deep" {
		t.Fatalf("copied content mismatch: %q err=%v", got, err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source not removed: %v", err)
	}
}

func TestBreaker_TripsThenDegradesAndResets(t *testing.T) {
	b := newBreaker(3)
	if !b.allow() {
		t.Fatal("fresh breaker should allow")
	}
	b.recordTimeout()
	b.recordTimeout()
	if !b.allow() {
		t.Fatal("breaker tripped before threshold")
	}
	b.recordTimeout() // third → trips
	if b.allow() {
		t.Fatal("breaker should be tripped at threshold")
	}
	b.recordSuccess()
	if !b.allow() {
		t.Fatal("breaker should re-close after a success")
	}
}
