package operation

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/anumey1/Suns/pkg/safety/firmlink"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/fsdelete"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/anumey1/Suns/pkg/trash"
)

// FileDeleteOp is the concrete FileDelete operation (§4.2). It is a pure value
// type — all fields are values, so plan.Seal copies it without retaining any
// reference into live state (§4.5).
type FileDeleteOp struct {
	Path     string             // absolute target path
	Size     int64              // reclaimable bytes, for the preview total
	Category string             // safe-cache category / source (for the history record)
	Expected identity.FileIdent // identity recorded at plan time (§4.7)
}

var _ Operation = FileDeleteOp{}

func (o FileDeleteOp) Kind() OpKind { return KindFileDelete }

// Reversibility reports the intrinsic best case for a file delete (Reversible,
// via trash). The EFFECTIVE reversibility depends on the active deletion mode
// (obliterate → Irreversible) and is resolved by the gate, which knows the mode
// (§4.3); the operation itself does not carry the mode.
func (o FileDeleteOp) Reversibility() Reversibility { return Reversible }

func (o FileDeleteOp) Describe() Preview {
	return Preview{
		Kind:          KindFileDelete,
		Reversibility: Reversible,
		Line:          o.Path,
		Bytes:         o.Size,
	}
}

// ValidateAtPlan confirms the target is off the deny floor and currently exists.
func (o FileDeleteOp) ValidateAtPlan(ctx context.Context) error {
	if err := floor.Check(o.Path); err != nil {
		return err
	}
	if _, err := os.Lstat(o.Path); err != nil {
		return err
	}
	return nil
}

// ValidateAtExec re-checks the deny floor and re-verifies the target's tiered
// identity against what was recorded at plan time, permitting a device change
// only across a known firmlink boundary (§4.6, §4.7). It returns the confirmed
// Identity that Execute requires.
func (o FileDeleteOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	if err := floor.Check(o.Path); err != nil {
		return Identity{}, err
	}
	cur, err := identity.ComputeFile(o.Path, identity.DefaultLargeThreshold)
	if err != nil {
		return Identity{}, err
	}
	if err := identity.VerifyFile(o.Expected, cur, firmlink.IsBoundary(o.Path)); err != nil {
		return Identity{}, err
	}
	return Identity{Kind: KindFileDelete, File: cur}, nil
}

// Execute trashes (default) or obliterates the target according to mode. In
// trash mode the approved root is moved atomically by pkg/trash; in obliterate
// mode the fd-anchored deleter is used with the confirmed identity (§4.4, §4.6).
func (o FileDeleteOp) Execute(ctx context.Context, mode Mode, id Identity) (Receipt, error) {
	r := Receipt{Kind: KindFileDelete, Time: time.Now()}
	switch mode {
	case ModeTrash:
		tr, err := getTrasher()
		if err != nil {
			r.Status, r.Err = "failed", err
			return r, err
		}
		res := tr.Trash(ctx, o.Path)
		if res.Skipped {
			r.Fate, r.Status = "skipped", "skipped:"+res.Reason
			return r, nil
		}
		r.Fate, r.Status, r.TrashPath = "trashed", "ok", res.TrashPath
		return r, nil

	case ModeObliterate:
		res, err := fsdelete.Obliterate(o.Path, id.File)
		if err != nil {
			r.Status, r.Err = "failed", err
			return r, err
		}
		if res.Removed == 0 && len(res.Skipped) > 0 {
			r.Fate, r.Status = "skipped", "skipped:identity-or-swap"
			return r, nil
		}
		r.Fate, r.Status = "obliterated", "ok"
		return r, nil

	default:
		err := fmt.Errorf("operation: unknown mode %d", mode)
		r.Status, r.Err = "failed", err
		return r, err
	}
}

// HistoryRecord produces the typed file_delete history entry (§4.10). The
// reversibility recorded reflects the actual fate: trashed → reversible,
// obliterated → irreversible.
func (o FileDeleteOp) HistoryRecord(r Receipt) HistoryEntry {
	rev := Reversible
	if r.Fate == "obliterated" {
		rev = Irreversible
	}
	expected := o.Expected
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindFileDelete,
		Reversible: rev,
		Status:     r.Status,
		Path:       o.Path,
		Size:       o.Size,
		Fate:       r.Fate,
		TrashPath:  r.TrashPath,
		OrigPath:   o.Path,
		Identity:   &expected,
	}
}

// Trash mechanism wiring.
//
// FileDeleteOp obtains its Trasher from a package-level default so the operation
// can execute itself (the interface fixes Execute's signature) while keeping
// FileDeleteOp a pure value. UseTrasher lets the Phase 1 executor inject a
// single Trasher shared across a batch (so the volume-scoped circuit breaker
// spans the whole run) and lets tests target a temporary Trash directory.
var (
	trasherMu      sync.Mutex
	trasherShared  *trash.Trasher
	trasherInitErr error
	trasherOnce    sync.Once
)

// UseTrasher injects the Trasher used by subsequent FileDeleteOp.Execute calls.
func UseTrasher(t *trash.Trasher) {
	trasherMu.Lock()
	defer trasherMu.Unlock()
	trasherShared = t
}

func getTrasher() (*trash.Trasher, error) {
	trasherMu.Lock()
	defer trasherMu.Unlock()
	if trasherShared != nil {
		return trasherShared, nil
	}
	trasherOnce.Do(func() { trasherShared, trasherInitErr = trash.New() })
	return trasherShared, trasherInitErr
}
