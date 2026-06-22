// Package plan defines the frozen, value-sealed execution plan (§4.5).
//
// The gate confirms this exact sealed plan (complete totals, not the paged UI
// view) and the executor consumes only this sealed plan, so structural drift
// between preview and execution is impossible.
package plan

import (
	"time"

	"github.com/anumey1/Suns/pkg/operation"
)

// Plan is a set of operations assembled after discovery. After Seal it cannot
// change even if the source state mutates (immutability is enforced by test,
// §15).
type Plan struct {
	ID        string                // a ULID, recorded in the operation history
	Ops       []operation.Operation // VALUE-SEALED after Seal()
	CreatedAt time.Time

	sealed bool
}

// Seal deep-copies every concrete operation into a pointer-free value form
// that retains no references into scanner buffers, SessionState, or caches.
// After Seal the plan cannot change even if the source state mutates (§4.5).
func (p *Plan) Seal() *Plan {
	// TODO(phase0): deep-copy each concrete op into pointer-free value form.
	p.sealed = true
	return p
}

// Sealed reports whether Seal has been called.
func (p *Plan) Sealed() bool { return p.sealed }
