package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestRing_BoundedOldestToNewest(t *testing.T) {
	r := NewRing(3)
	for i := 1; i <= 5; i++ {
		r.Push(float64(i))
	}
	got := r.Values()
	want := []float64{3, 4, 5} // oldest→newest, capped at 3
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Values() = %v, want %v", got, want)
		}
	}
}

func TestSample_PopulatesCheapSourcesLive(t *testing.T) {
	p := New(time.Second)
	ctx := context.Background()
	_ = p.sample(ctx) // prime
	s := p.sample(ctx)

	if s == nil {
		t.Fatal("nil snapshot")
	}
	if s.Sources[SrcCPU].Health != HealthLive {
		t.Errorf("CPU source = %v, want live", s.Sources[SrcCPU].Health)
	}
	if s.Sources[SrcMemory].Health != HealthLive {
		t.Errorf("memory source = %v, want live", s.Sources[SrcMemory].Health)
	}
	if s.Memory.Total == 0 {
		t.Error("memory total is zero")
	}
	if len(s.CPU.PerCore) == 0 {
		t.Error("no per-core CPU data")
	}
	// Heavy/root sources are honestly reported unavailable until wired.
	if s.Sources[SrcThermal].Health != HealthUnavailable {
		t.Errorf("thermal = %v, want unavailable", s.Sources[SrcThermal].Health)
	}
}

func TestRun_PublishesSnapshotAndStops(t *testing.T) {
	p := New(20 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	// Give it a few ticks to publish.
	deadline := time.After(2 * time.Second)
	for p.Snapshot() == nil {
		select {
		case <-deadline:
			t.Fatal("no snapshot published")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not stop on context cancel")
	}
}
