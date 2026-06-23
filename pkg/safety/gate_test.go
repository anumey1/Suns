package safety_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/anumey1/Suns/pkg/trash"
)

// fakeFlush is a non-file operation used to assert that the deletion axis is
// inert for non-file kinds (§4.3): it must always read Irreversible regardless
// of the deletion mode.
type fakeFlush struct{}

func (fakeFlush) Kind() operation.OpKind { return operation.KindDNSFlush }
func (fakeFlush) Describe() operation.Preview {
	return operation.Preview{Kind: operation.KindDNSFlush, Line: "flush DNS cache"}
}
func (fakeFlush) Reversibility() operation.Reversibility { return operation.Irreversible }
func (fakeFlush) ValidateAtPlan(context.Context) error   { return nil }
func (fakeFlush) ValidateAtExec(context.Context) (operation.Identity, error) {
	return operation.Identity{Kind: operation.KindDNSFlush}, nil
}
func (fakeFlush) Execute(context.Context, operation.Mode, operation.Identity) (operation.Receipt, error) {
	return operation.Receipt{Kind: operation.KindDNSFlush, Fate: "flushed", Status: "ok"}, nil
}
func (fakeFlush) HistoryRecord(operation.Receipt) operation.HistoryEntry {
	return operation.HistoryEntry{Op: operation.KindDNSFlush, Reversible: operation.Irreversible}
}

func TestBuildGroups_DeletionAxisOnlyAffectsFileDelete(t *testing.T) {
	ops := []operation.Operation{
		operation.FileDeleteOp{Path: "/x", Size: 10},
		fakeFlush{},
	}
	p := plan.New(ops).Seal()

	// Trash mode: FileDelete is Reversible; the flush stays Irreversible.
	for _, g := range safety.BuildGroups(p, operation.ModeTrash) {
		switch g.Kind {
		case operation.KindFileDelete:
			if g.Reversibility != operation.Reversible {
				t.Errorf("FileDelete in trash mode = %v, want Reversible", g.Reversibility)
			}
		case operation.KindDNSFlush:
			if g.Reversibility != operation.Irreversible {
				t.Errorf("DNSFlush = %v, want Irreversible regardless of mode", g.Reversibility)
			}
		}
	}

	// Obliterate mode: FileDelete becomes Irreversible; the flush is unchanged.
	for _, g := range safety.BuildGroups(p, operation.ModeObliterate) {
		if g.Kind == operation.KindFileDelete && g.Reversibility != operation.Irreversible {
			t.Errorf("FileDelete in obliterate mode = %v, want Irreversible", g.Reversibility)
		}
		if g.Kind == operation.KindDNSFlush && g.Reversibility != operation.Irreversible {
			t.Errorf("DNSFlush changed with mode: %v", g.Reversibility)
		}
	}
}

// Full spine flow: validate → execute → typed history entry stamped with the
// plan ID. A vanished target is skipped, not executed (§4.7, §6.1).
func TestExecute_FullFlowWithSkip(t *testing.T) {
	root := t.TempDir()
	tr, err := trash.NewWithDir(filepath.Join(root, "Trash"))
	if err != nil {
		t.Fatal(err)
	}
	operation.UseTrasher(tr)

	good := filepath.Join(root, "good.bin")
	if err := os.WriteFile(good, []byte("keepable"), 0o644); err != nil {
		t.Fatal(err)
	}
	goodID, err := identity.ComputeFile(good, identity.DefaultLargeThreshold)
	if err != nil {
		t.Fatal(err)
	}

	// A planned target that no longer exists at execution time.
	gone := filepath.Join(root, "gone.bin")
	goneID := identity.FileIdent{Tier: identity.TierSmall, Size: 3, Hash: "deadbeef"}

	p := plan.New([]operation.Operation{
		operation.FileDeleteOp{Path: good, Size: 8, Expected: goodID},
		operation.FileDeleteOp{Path: gone, Size: 3, Expected: goneID},
	}).Seal()

	results := safety.Execute(context.Background(), p, operation.ModeTrash)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	var trashed, skipped int
	for _, r := range results {
		if r.Entry.Plan != p.ID {
			t.Errorf("history entry not stamped with plan ID: %q != %q", r.Entry.Plan, p.ID)
		}
		switch r.Receipt.Fate {
		case "trashed":
			trashed++
		default:
			if r.Skipped {
				skipped++
			}
		}
	}
	if trashed != 1 || skipped != 1 {
		t.Fatalf("trashed=%d skipped=%d, want 1/1", trashed, skipped)
	}
	if _, err := os.Lstat(good); !os.IsNotExist(err) {
		t.Errorf("good target not trashed: %v", err)
	}
}
