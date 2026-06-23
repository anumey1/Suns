package safety

import (
	"context"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
)

// ValidatedOp pairs an operation with the outcome of its execution-time
// identity recheck (§4.7). A non-nil Err means the op must be skipped at
// execution (the target vanished, was replaced, changed identity, or is now
// floor-protected) — the executor records it as skipped rather than acting.
type ValidatedOp struct {
	Op  operation.Operation
	ID  operation.Identity
	Err error
}

// Revalidate runs ValidateAtExec on every operation in the sealed plan,
// collecting confirmed identities and per-op skip reasons. It does not execute
// anything. This is the TOCTOU / PID-reuse defense applied just before action.
func Revalidate(ctx context.Context, p *plan.Plan) []ValidatedOp {
	out := make([]ValidatedOp, 0, len(p.Ops))
	for _, op := range p.Ops {
		id, err := op.ValidateAtExec(ctx)
		out = append(out, ValidatedOp{Op: op, ID: id, Err: err})
	}
	return out
}

// ExecResult is the outcome of executing one operation, including the typed
// history entry ready to be appended to the operation history.
type ExecResult struct {
	Op      operation.Operation
	Receipt operation.Receipt
	Entry   operation.HistoryEntry
	Skipped bool
	Err     error
}

// Execute revalidates and then runs each operation in the sealed plan under the
// given deletion mode, stamping each history entry with the plan ID. An op that
// fails revalidation is skipped (recorded, not executed); the rest proceed, so
// a single bad target never aborts the whole batch (§6.1 partial-batch
// semantics generalize here). Callers append each ExecResult.Entry to the
// history log and render the receipts.
func Execute(ctx context.Context, p *plan.Plan, deletion operation.Mode) []ExecResult {
	results := make([]ExecResult, 0, len(p.Ops))
	for _, vo := range Revalidate(ctx, p) {
		if vo.Err != nil {
			entry := vo.Op.HistoryRecord(operation.Receipt{Kind: vo.Op.Kind(), Fate: "skipped", Status: "skipped:revalidation"})
			entry.Plan = p.ID
			results = append(results, ExecResult{Op: vo.Op, Skipped: true, Err: vo.Err, Entry: entry})
			continue
		}
		receipt, err := vo.Op.Execute(ctx, deletion, vo.ID)
		entry := vo.Op.HistoryRecord(receipt)
		entry.Plan = p.ID
		results = append(results, ExecResult{
			Op:      vo.Op,
			Receipt: receipt,
			Entry:   entry,
			Skipped: receipt.Fate == "skipped",
			Err:     err,
		})
	}
	return results
}
