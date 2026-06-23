package restore_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anumey1/Suns/internal/restore"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// trashed simulates a completed FileDelete: it records identity, moves the file
// into a Trash dir, and returns the history entry restore would consume.
func trashed(t *testing.T, root, rel string, content []byte) operation.HistoryEntry {
	t.Helper()
	orig := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(orig), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orig, content, 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := identity.ComputeFile(orig, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	trashDir := filepath.Join(root, "Trash")
	if err := os.MkdirAll(trashDir, 0o700); err != nil {
		t.Fatal(err)
	}
	trashPath := filepath.Join(trashDir, filepath.Base(orig))
	if err := os.Rename(orig, trashPath); err != nil {
		t.Fatal(err)
	}
	return operation.HistoryEntry{
		Op: operation.KindFileDelete, Reversible: operation.Reversible, Fate: "trashed",
		OrigPath: orig, TrashPath: trashPath, Identity: &id, Size: int64(len(content)),
	}
}

func TestRestore_RoundTrip(t *testing.T) {
	root := t.TempDir()
	e := trashed(t, root, "proj/cache.txt", []byte("precious"))

	o := restore.Restore(e)
	if !o.Restored {
		t.Fatalf("not restored: %s", o.Reason)
	}
	if o.Path != e.OrigPath {
		t.Fatalf("restored to %q, want original %q", o.Path, e.OrigPath)
	}
	got, err := os.ReadFile(e.OrigPath)
	if err != nil || string(got) != "precious" {
		t.Fatalf("content not restored: %q err=%v", got, err)
	}
	if _, err := os.Lstat(e.TrashPath); !os.IsNotExist(err) {
		t.Fatalf("trash entry still present: %v", err)
	}
}

func TestRestore_CollisionRestoresAlongside(t *testing.T) {
	root := t.TempDir()
	e := trashed(t, root, "proj/cache.txt", []byte("original"))

	// Re-occupy the original path.
	if err := os.WriteFile(e.OrigPath, []byte("occupying"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := restore.Restore(e)
	if !o.Restored {
		t.Fatalf("not restored: %s", o.Reason)
	}
	if o.Path == e.OrigPath {
		t.Fatal("restore overwrote the occupied original path")
	}
	if got, _ := os.ReadFile(e.OrigPath); string(got) != "occupying" {
		t.Fatalf("occupant was overwritten: %q", got)
	}
	if got, _ := os.ReadFile(o.Path); string(got) != "original" {
		t.Fatalf("alongside copy wrong: %q", got)
	}
}

func TestRestore_RefusesTamperedTrashEntry(t *testing.T) {
	root := t.TempDir()
	e := trashed(t, root, "proj/cache.txt", []byte("original"))

	// Tamper with the trashed object after deletion.
	if err := os.WriteFile(e.TrashPath, []byte("tampered-and-longer"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := restore.Restore(e)
	if o.Restored {
		t.Fatal("restored a tampered trash entry")
	}
	if _, err := os.Lstat(e.OrigPath); !os.IsNotExist(err) {
		t.Fatalf("tampered entry was put back at original path: %v", err)
	}
}

func TestRestore_RecreatesMissingParent(t *testing.T) {
	root := t.TempDir()
	e := trashed(t, root, "proj/nested/cache.txt", []byte("data"))

	// Remove the now-empty parent directory.
	if err := os.RemoveAll(filepath.Join(root, "proj")); err != nil {
		t.Fatal(err)
	}
	o := restore.Restore(e)
	if !o.Restored {
		t.Fatalf("not restored: %s", o.Reason)
	}
	if got, _ := os.ReadFile(e.OrigPath); string(got) != "data" {
		t.Fatalf("content after parent recreate: %q", got)
	}
}

func TestCandidates_FiltersToReversibleTrashed(t *testing.T) {
	entries := []operation.HistoryEntry{
		{Op: operation.KindFileDelete, Reversible: operation.Reversible, Fate: "trashed", OrigPath: "/a", TrashPath: "/t/a"},
		{Op: operation.KindFileDelete, Reversible: operation.Irreversible, Fate: "obliterated", OrigPath: "/b"},
		{Op: operation.KindProcessKill, Reversible: operation.Irreversible},
		{Op: operation.KindFileDelete, Reversible: operation.Reversible, Fate: "skipped", OrigPath: "/c"},
	}
	got := restore.Candidates(entries)
	if len(got) != 1 || got[0].OrigPath != "/a" {
		t.Fatalf("Candidates = %+v, want only the trashed reversible entry", got)
	}
}
