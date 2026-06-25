package operation

import (
	"context"
	"errors"
	"sync"
)

// PruneStats reports what a container-engine prune reclaimed.
type PruneStats struct {
	ReclaimedBytes int64
}

// ErrContainerEngineUnavailable means no running container engine was wired, so a
// ContainerPruneOp is skipped rather than failing hard (§6.1). The CLI detects
// engine presence before planning, so this is the defensive fallback.
var ErrContainerEngineUnavailable = errors.New("operation: no container engine available")

// ContainerPruner reclaims space from a container engine (Docker / Colima /
// OrbStack) by pruning unused images, containers, networks and, optionally,
// volumes. It is the injection seam through which ContainerPruneOp reaches the
// discovered Docker CLI, keeping the op a pure value type (mirroring Trasher,
// ProcessKiller, and SystemRunner).
type ContainerPruner interface {
	Prune(ctx context.Context, includeVolumes bool) (PruneStats, error)
}

// unavailablePruner is the default: it refuses, so an unconfigured executor never
// silently does nothing-looking-like-success.
type unavailablePruner struct{}

func (unavailablePruner) Prune(context.Context, bool) (PruneStats, error) {
	return PruneStats{}, ErrContainerEngineUnavailable
}

var (
	containerPrunerMu sync.RWMutex
	containerPruner   ContainerPruner = unavailablePruner{}
)

// UseContainerPruner injects the pruner used by subsequent ContainerPruneOp
// executions. The docker-prune command wires a CLI-backed pruner (with the
// discovered binary) just before executing, mirroring how clean/nuke wire the
// Trasher; tests inject a fake; the default refuses.
func UseContainerPruner(p ContainerPruner) {
	containerPrunerMu.Lock()
	defer containerPrunerMu.Unlock()
	if p == nil {
		p = unavailablePruner{}
	}
	containerPruner = p
}

func getContainerPruner() ContainerPruner {
	containerPrunerMu.RLock()
	defer containerPrunerMu.RUnlock()
	return containerPruner
}
