package operation

import (
	"context"
	"errors"
	"time"
)

// ContainerPruneOp reclaims space from a container engine by pruning all unused
// images, stopped containers, networks and, when IncludeVolumes is set, unused
// volumes (§12.15). It is a pure value type so plan.Seal copies it without
// aliasing; the discovered engine binary is reached through the injected
// ContainerPruner, not held by the op.
//
// Reversibility is Irreversible (🔴): pruned images must be re-pulled or rebuilt
// and pruned volumes are gone. The Jarjar deletion Mode axis is inert.
type ContainerPruneOp struct {
	Endpoint       string // discovered engine endpoint/binary, for the preview + history
	EstimatedBytes int64  // reclaimable estimate from `docker system df`
	IncludeVolumes bool   // also prune unused volumes (more destructive)
}

var _ Operation = ContainerPruneOp{}

func (o ContainerPruneOp) Kind() OpKind { return KindContainerPrune }

func (o ContainerPruneOp) Reversibility() Reversibility { return Irreversible }

func (o ContainerPruneOp) Describe() Preview {
	return Preview{
		Kind:          KindContainerPrune,
		Reversibility: Irreversible,
		Line:          o.command() + "  (" + o.Endpoint + ")",
		Bytes:         o.EstimatedBytes,
	}
}

// ValidateAtPlan has nothing to check; engine presence is established by the CLI
// before planning.
func (o ContainerPruneOp) ValidateAtPlan(ctx context.Context) error { return nil }

// ValidateAtExec returns the trivial prune identity; there is no per-target
// identity to drift.
func (o ContainerPruneOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	return Identity{Kind: KindContainerPrune}, nil
}

// Execute runs the prune through the injected ContainerPruner. A missing engine
// is a clean skip rather than a failure (§6.1).
func (o ContainerPruneOp) Execute(ctx context.Context, _ Mode, _ Identity) (Receipt, error) {
	r := Receipt{Kind: KindContainerPrune, Time: time.Now()}
	_, err := getContainerPruner().Prune(ctx, o.IncludeVolumes)
	switch {
	case err == nil:
		r.Fate, r.Status = "pruned", "ok"
	case errors.Is(err, ErrContainerEngineUnavailable):
		r.Fate, r.Status = "skipped", "skipped:no-engine"
	default:
		r.Fate, r.Status, r.Err = "skipped", "failed", err
	}
	return r, nil
}

func (o ContainerPruneOp) HistoryRecord(r Receipt) HistoryEntry {
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindContainerPrune,
		Reversible: Irreversible,
		Status:     r.Status,
		Fate:       r.Fate,
		Size:       o.EstimatedBytes,
		Cmd:        o.command(),
		Path:       o.Endpoint,
	}
}

func (o ContainerPruneOp) command() string {
	c := "docker system prune -a"
	if o.IncludeVolumes {
		c += " --volumes"
	}
	return c
}
