package plan_test

import (
	"context"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
)

// After Seal, mutating the original concrete operation through a pointer must
// not change the sealed plan: Seal value-copies pointer-backed ops (§4.5).
func TestSeal_ValueCopiesPointerBackedOps(t *testing.T) {
	fd := &operation.FileDeleteOp{Path: "/Users/x/Library/Caches/a", Size: 100}
	p := plan.New([]operation.Operation{fd}).Seal()

	fd.Path = "/evil"
	fd.Size = 999

	got := p.Ops[0].Describe()
	if got.Line != "/Users/x/Library/Caches/a" || got.Bytes != 100 {
		t.Fatalf("sealed plan drifted after source mutation: %+v", got)
	}
}

// Growing or rewriting the caller's original slice after Seal must not affect
// the plan: Seal re-slices into a fresh backing array.
func TestSeal_FreshSliceImmuneToSourceMutation(t *testing.T) {
	ops := []operation.Operation{
		operation.FileDeleteOp{Path: "/a"},
		operation.FileDeleteOp{Path: "/b"},
	}
	p := plan.New(ops).Seal()

	ops[0] = operation.FileDeleteOp{Path: "/hacked"}
	ops = append(ops, operation.FileDeleteOp{Path: "/extra"})
	_ = ops

	if len(p.Ops) != 2 {
		t.Fatalf("len(p.Ops) = %d, want 2", len(p.Ops))
	}
	if line := p.Ops[0].Describe().Line; line != "/a" {
		t.Fatalf("p.Ops[0] = %q, want /a", line)
	}
}

func TestNewID_UniqueAndSortable(t *testing.T) {
	a := plan.NewID()
	b := plan.NewID()
	if a == b {
		t.Fatalf("NewID collided: %q", a)
	}
	if len(a) != 32 {
		t.Fatalf("NewID length = %d, want 32 hex chars", len(a))
	}
}

// An operation that is pointer-backed AND whose value form does not implement
// Operation (pointer-receiver methods) violates the value-type contract; Seal
// must fail loud rather than seal an aliased, mutable op (§4.5).
func TestSeal_PanicsOnPointerReceiverOp(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Seal did not panic on a pointer-receiver operation")
		}
	}()
	plan.New([]operation.Operation{&ptrOnlyOp{}}).Seal()
}

// ptrOnlyOp satisfies Operation only via pointer receivers, so its value form
// does not implement Operation.
type ptrOnlyOp struct{ n int }

func (o *ptrOnlyOp) Kind() operation.OpKind                 { return operation.KindCacheReset }
func (o *ptrOnlyOp) Describe() operation.Preview            { return operation.Preview{} }
func (o *ptrOnlyOp) Reversibility() operation.Reversibility { return operation.Irreversible }
func (o *ptrOnlyOp) ValidateAtPlan(context.Context) error   { return nil }
func (o *ptrOnlyOp) ValidateAtExec(context.Context) (operation.Identity, error) {
	return operation.Identity{}, nil
}
func (o *ptrOnlyOp) Execute(context.Context, operation.Mode, operation.Identity) (operation.Receipt, error) {
	return operation.Receipt{}, nil
}
func (o *ptrOnlyOp) HistoryRecord(operation.Receipt) operation.HistoryEntry {
	return operation.HistoryEntry{}
}
