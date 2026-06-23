package operation_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/anumey1/Suns/pkg/trash"
)

func TestFileDeleteOp_TrashMode(t *testing.T) {
	root := t.TempDir()
	tr, err := trash.NewWithDir(filepath.Join(root, "Trash"))
	if err != nil {
		t.Fatal(err)
	}
	operation.UseTrasher(tr)

	f := filepath.Join(root, "cache.bin")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := identity.ComputeFile(f, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.FileDeleteOp{Path: f, Size: 4, Expected: id}

	vid, err := op.ValidateAtExec(context.Background())
	if err != nil {
		t.Fatalf("ValidateAtExec: %v", err)
	}
	r, err := op.Execute(context.Background(), operation.ModeTrash, vid)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if r.Fate != "trashed" {
		t.Fatalf("Fate = %q, want trashed (status %q)", r.Fate, r.Status)
	}
	if _, err := os.Lstat(f); !os.IsNotExist(err) {
		t.Fatalf("original still present after trash: %v", err)
	}

	rec := op.HistoryRecord(r)
	if rec.Op != operation.KindFileDelete || rec.Reversible != operation.Reversible {
		t.Fatalf("history record wrong: %+v", rec)
	}
}

func TestFileDeleteOp_ObliterateMode(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "cache.bin")
	if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := identity.ComputeFile(f, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.FileDeleteOp{Path: f, Size: 4, Expected: id}

	vid, err := op.ValidateAtExec(context.Background())
	if err != nil {
		t.Fatalf("ValidateAtExec: %v", err)
	}
	r, err := op.Execute(context.Background(), operation.ModeObliterate, vid)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if r.Fate != "obliterated" {
		t.Fatalf("Fate = %q, want obliterated (status %q)", r.Fate, r.Status)
	}
	if _, err := os.Lstat(f); !os.IsNotExist(err) {
		t.Fatalf("file survived obliterate: %v", err)
	}
	if rec := op.HistoryRecord(r); rec.Reversible != operation.Irreversible {
		t.Fatalf("obliterate must record irreversible, got %v", rec.Reversible)
	}
}

func TestFileDeleteOp_RefusesIdentityMismatch(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "config")
	if err := os.WriteFile(f, []byte("original"), 0o644); err != nil {
		t.Fatal(err)
	}
	id, err := identity.ComputeFile(f, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.FileDeleteOp{Path: f, Expected: id}

	// Replace content after planning.
	if err := os.WriteFile(f, []byte("REPLACED-LONGER-CONTENT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Fatal("ValidateAtExec accepted a target whose identity changed")
	}
}

// The deny floor is rechecked at execution-validation time and cannot be
// bypassed (§4.7, §5.3).
func TestFileDeleteOp_FloorRecheckedAtExec(t *testing.T) {
	op := operation.FileDeleteOp{Path: "/System/Library/x"}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Fatal("ValidateAtExec did not reject a floor-protected path")
	}
}
