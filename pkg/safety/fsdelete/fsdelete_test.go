package fsdelete

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anumey1/Suns/pkg/safety/identity"
)

func mkTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	victim := filepath.Join(root, "victim")
	must(os.MkdirAll(filepath.Join(victim, "a", "b"), 0o755))
	must(os.WriteFile(filepath.Join(victim, "f1.txt"), []byte("one"), 0o644))
	must(os.WriteFile(filepath.Join(victim, "a", "f2.txt"), []byte("two"), 0o644))
	must(os.WriteFile(filepath.Join(victim, "a", "b", "f3.txt"), []byte("three"), 0o644))
	return root
}

func TestObliterate_RemovesTreeCompletely(t *testing.T) {
	root := mkTree(t)
	victim := filepath.Join(root, "victim")

	id, err := identity.ComputeFile(victim, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Obliterate(victim, id)
	if err != nil {
		t.Fatalf("Obliterate: %v", err)
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("unexpected skips: %v", res.Skipped)
	}
	if _, err := os.Lstat(victim); !os.IsNotExist(err) {
		t.Fatalf("victim still exists: %v", err)
	}
	if res.Removed == 0 {
		t.Fatalf("Removed = 0, want > 0")
	}
}

// The deletion must never follow a symlink out of the intended subtree. A
// symlink standing where the walker descends is unlinked as a link, never
// traversed — the same O_NOFOLLOW openat mechanism that defeats a concurrent
// directory-to-symlink swap (§4.6).
func TestObliterate_NeverEscapesViaSymlink(t *testing.T) {
	root := mkTree(t)
	victim := filepath.Join(root, "victim")

	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(outside, "DO_NOT_DELETE")
	if err := os.WriteFile(sentinel, []byte("precious"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Replace victim/a/b with a symlink pointing at outside.
	if err := os.RemoveAll(filepath.Join(victim, "a", "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(victim, "a", "b")); err != nil {
		t.Fatal(err)
	}

	id, err := identity.ComputeFile(victim, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Obliterate(victim, id); err != nil {
		t.Fatalf("Obliterate: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel outside the subtree was destroyed: %v", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside directory was destroyed: %v", err)
	}
}

// A target whose identity changed after planning is refused (skipped), not
// deleted (§4.7).
func TestObliterate_RefusesIdentityMismatch(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "config.txt")
	if err := os.WriteFile(f, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := identity.ComputeFile(f, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	// Swap the content (and size) after planning.
	if err := os.WriteFile(f, []byte("REPLACED-CONTENT-DIFFERENT-SIZE"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Obliterate(f, id)
	if err != nil {
		t.Fatalf("Obliterate: %v", err)
	}
	if len(res.Skipped) == 0 {
		t.Fatalf("expected the modified target to be skipped, got Removed=%d", res.Removed)
	}
	if _, err := os.Stat(f); err != nil {
		t.Fatalf("modified target was deleted despite identity mismatch: %v", err)
	}
}
