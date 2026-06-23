// Package plan defines the frozen, value-sealed execution plan (§4.5).
//
// The gate confirms this exact sealed plan (complete totals, not the paged UI
// view) and the executor consumes only this sealed plan, so structural drift
// between preview and execution is impossible.
package plan

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"reflect"
	"time"

	"github.com/anumey1/Suns/pkg/operation"
)

// Plan is a set of operations assembled after discovery. After Seal it cannot
// change even if the source state mutates (immutability is enforced by test,
// §15).
type Plan struct {
	ID        string                // unique, time-sortable; recorded in the operation history
	Ops       []operation.Operation // value-sealed after Seal()
	CreatedAt time.Time

	sealed bool
}

// New creates an unsealed plan from the complete operation set (not the paged
// UI view). Callers Seal it before showing the gate.
func New(ops []operation.Operation) *Plan {
	return &Plan{
		ID:        NewID(),
		Ops:       ops,
		CreatedAt: time.Now(),
	}
}

// Seal deep-copies every concrete operation into a pointer-free VALUE form that
// retains no references into scanner buffers, SessionState, or caches. Concrete
// operation types are deliberately designed as value types, so value-copying
// each interface element (dereferencing any pointer wrapper) is sufficient:
// after Seal the plan cannot change even if the source state mutates (§4.5).
//
// Seal also re-slices Ops into a freshly allocated slice, so growing or
// reordering the caller's original slice cannot affect the sealed plan.
func (p *Plan) Seal() *Plan {
	sealed := make([]operation.Operation, len(p.Ops))
	for i, op := range p.Ops {
		sealed[i] = valueCopy(op)
	}
	p.Ops = sealed
	p.sealed = true
	return p
}

// Sealed reports whether Seal has been called.
func (p *Plan) Sealed() bool { return p.sealed }

// valueCopy returns op as a pointer-free value. If op wraps a pointer to a
// concrete struct, it is dereferenced into a fresh value copy so that later
// mutation through the original pointer cannot change the sealed plan. The
// concrete value type must itself implement Operation (value receivers) — the
// design contract for operation kinds (§4.2, §4.5).
func valueCopy(op operation.Operation) operation.Operation {
	v := reflect.ValueOf(op)
	if v.Kind() != reflect.Pointer {
		return op // already a value
	}
	if v.IsNil() {
		return op
	}
	fresh := reflect.New(v.Elem().Type()).Elem()
	fresh.Set(v.Elem()) // copy the pointed-to struct by value
	if got, ok := fresh.Interface().(operation.Operation); ok {
		return got
	}
	// The value form does not satisfy Operation (pointer-receiver methods): the
	// op violates the value-type contract. Fail loud rather than seal an
	// aliased, mutable operation.
	panic(fmt.Sprintf("plan: operation %T is pointer-backed and its value form does not implement Operation; "+
		"operation kinds must be pointer-free value types (§4.5)", op))
}

// NewID returns a unique, lexicographically time-sortable identifier. It is a
// 48-bit millisecond timestamp prefix followed by 80 random bits, hex-encoded —
// ULID-shaped ordering without an external dependency. (A canonical ULID
// library can be substituted later without changing callers.)
func NewID() string {
	ms := uint64(time.Now().UnixMilli())
	var buf [16]byte
	buf[0] = byte(ms >> 40)
	buf[1] = byte(ms >> 32)
	buf[2] = byte(ms >> 24)
	buf[3] = byte(ms >> 16)
	buf[4] = byte(ms >> 8)
	buf[5] = byte(ms)
	_, _ = rand.Read(buf[6:])
	return hex.EncodeToString(buf[:])
}
