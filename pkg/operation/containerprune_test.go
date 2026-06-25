package operation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
)

// fakePruner records the includeVolumes flag and returns a programmable outcome.
type fakePruner struct {
	sawVolumes bool
	stats      operation.PruneStats
	err        error
}

func (f *fakePruner) Prune(_ context.Context, includeVolumes bool) (operation.PruneStats, error) {
	f.sawVolumes = includeVolumes
	return f.stats, f.err
}

func usefakePruner(t *testing.T, p operation.ContainerPruner) {
	t.Helper()
	operation.UseContainerPruner(p)
	t.Cleanup(func() { operation.UseContainerPruner(nil) })
}

func TestContainerPrune_Success(t *testing.T) {
	fp := &fakePruner{stats: operation.PruneStats{ReclaimedBytes: 1 << 30}}
	usefakePruner(t, fp)

	op := operation.ContainerPruneOp{Endpoint: "unix:///x.sock", EstimatedBytes: 2 << 30, IncludeVolumes: true}
	id, err := op.ValidateAtExec(context.Background())
	if err != nil || id.Kind != operation.KindContainerPrune {
		t.Fatalf("ValidateAtExec = %v, %v", id, err)
	}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, id)
	if r.Fate != "pruned" || r.Status != "ok" {
		t.Errorf("fate/status = %q/%q, want pruned/ok", r.Fate, r.Status)
	}
	if !fp.sawVolumes {
		t.Error("IncludeVolumes must be passed through to the pruner")
	}
}

func TestContainerPrune_NoEngineSkips(t *testing.T) {
	// The default pruner (no injection) refuses → clean skip.
	operation.UseContainerPruner(nil)
	op := operation.ContainerPruneOp{Endpoint: "none"}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Fate != "skipped" || r.Status != "skipped:no-engine" {
		t.Errorf("fate/status = %q/%q, want skipped/skipped:no-engine", r.Fate, r.Status)
	}
}

func TestContainerPrune_FailureRecorded(t *testing.T) {
	usefakePruner(t, &fakePruner{err: errors.New("daemon exploded")})
	op := operation.ContainerPruneOp{Endpoint: "x"}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if r.Status != "failed" || r.Err == nil {
		t.Errorf("status/err = %q/%v, want failed/non-nil", r.Status, r.Err)
	}
}

func TestContainerPrune_DescribeAndHistory(t *testing.T) {
	op := operation.ContainerPruneOp{Endpoint: "unix:///x", EstimatedBytes: 100, IncludeVolumes: true}
	if op.Reversibility() != operation.Irreversible {
		t.Error("container prune must be Irreversible")
	}
	pv := op.Describe()
	if pv.Bytes != 100 || pv.Reversibility != operation.Irreversible {
		t.Errorf("preview = %+v", pv)
	}
	e := op.HistoryRecord(operation.Receipt{Status: "ok", Fate: "pruned"})
	if e.Op != operation.KindContainerPrune || e.Size != 100 || e.Cmd != "docker system prune -a --volumes" {
		t.Errorf("history = %+v", e)
	}
}
