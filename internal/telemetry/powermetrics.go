package telemetry

import (
	"context"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/anumey1/Suns/pkg/plist"
	howett "howett.net/plist"
)

// pmDoc is the subset of a `powermetrics -f plist` sample we consume. The exact
// schema is version- and sampler-sensitive (§13.1), so decoding is tolerant:
// missing keys leave zero values. Field mapping is best-effort and validated
// on-device against captured output.
type pmDoc struct {
	GPU struct {
		IdleRatio float64 `plist:"idle_ratio"`
	} `plist:"gpu"`
	Processor struct {
		CombinedPowerMW float64 `plist:"combined_power"` // milliwatts
		PackageWatts    float64 `plist:"package_watts"`
	} `plist:"processor"`
	ThermalPressure string `plist:"thermal_pressure"`
}

// DecodePowerMetrics decodes one powermetrics plist document into a Power
// sample.
func DecodePowerMetrics(doc []byte) (Power, error) {
	var d pmDoc
	if _, err := howett.Unmarshal(doc, &d); err != nil {
		return Power{}, err
	}
	p := Power{
		GPUPercent:   clampPct((1 - d.GPU.IdleRatio) * 100),
		SystemWatts:  d.Processor.CombinedPowerMW / 1000,
		PackageWatts: d.Processor.PackageWatts,
		Throttled:    d.ThermalPressure != "" && !strings.EqualFold(d.ThermalPressure, "Nominal"),
	}
	return p, nil
}

func clampPct(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// PowerSource is the supervised powermetrics stream (§7.4). It holds the latest
// decoded sample and a health state, both read by the Poller when merging
// snapshots.
type PowerSource struct {
	mu     sync.RWMutex
	latest Power
	state  SourceState
}

// NewPowerSource returns a source in the Unavailable state until its stream
// produces a sample.
func NewPowerSource() *PowerSource {
	return &PowerSource{state: SourceState{Health: HealthUnavailable, Reason: "not started"}}
}

// Latest returns the most recent sample and its health.
func (ps *PowerSource) Latest() (Power, SourceState) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.latest, ps.state
}

func (ps *PowerSource) set(p Power) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.latest = p
	ps.state = SourceState{Health: HealthLive, LastSample: time.Now()}
}

func (ps *PowerSource) setHealth(h Health, reason string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.state.Health = h
	ps.state.Reason = reason
}

// run consumes one powermetrics stream from r, decoding each document with the
// deadline-guarded tokenizer; a stall marks the source stalled and returns so
// the supervisor can restart it (§7.3).
func (ps *PowerSource) run(ctx context.Context, r io.Reader, deadline time.Duration) error {
	return plist.StreamWithDeadline(ctx, r, deadline,
		func(doc []byte) {
			if p, err := DecodePowerMetrics(doc); err == nil {
				ps.set(p)
			}
		},
		func() { ps.setHealth(HealthStalled, "powermetrics stalled mid-flush") },
	)
}

// Launcher starts a powermetrics stream and returns its stdout. Closing the
// returned ReadCloser must terminate the subprocess (so a stall can be
// recovered by killing it). The app supplies a launcher that runs
// `sudo -n powermetrics -f plist …` through the privilege chokepoint.
type Launcher func(ctx context.Context) (io.ReadCloser, error)

// Supervise runs the source under a watchdog: launch → decode → on stall/EOF
// mark the state and relaunch with exponential backoff, until ctx is cancelled
// (§7.4). A launch failure (e.g. no sudo ticket) is surfaced as Unavailable and
// retried with backoff rather than failing hard.
func (ps *PowerSource) Supervise(ctx context.Context, launch Launcher, deadline time.Duration) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 10 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		ps.setHealth(HealthRestarting, "starting powermetrics")
		rc, err := launch(ctx)
		if err != nil {
			ps.setHealth(HealthUnavailable, "elevation required: "+err.Error())
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		runErr := ps.run(ctx, rc, deadline)
		_ = rc.Close() // terminates the subprocess
		if ctx.Err() != nil {
			return
		}
		if runErr == nil {
			ps.setHealth(HealthUnavailable, "powermetrics exited")
		}
		if !sleepCtx(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

func nextBackoff(cur, max time.Duration) time.Duration {
	cur *= 2
	if cur > max {
		return max
	}
	return cur
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
