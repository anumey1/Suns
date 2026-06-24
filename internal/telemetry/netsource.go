package telemetry

import (
	"bufio"
	"context"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// NetSource is the supervised, EXPERIMENTAL per-process network stream (§3.3,
// §7.5). It consumes a long-lived `nettop -P -x -l 0` capture, differences
// consecutive frames (the byte columns are cumulative), and holds the latest
// top-N talkers plus a health state — read by the Poller when merging snapshots.
//
// gopsutil has no reliable per-PID network on Darwin, so this scrapes nettop and
// degrades cleanly: a stall/EOF marks the source non-live and the supervisor
// relaunches, rather than ever showing frozen numbers as live.
type NetSource struct {
	mu     sync.RWMutex
	latest []NetProc
	state  SourceState
	topN   int
}

// nettopFrameTime is the per-row timestamp format in `nettop -x` output.
const nettopFrameTime = "15:04:05.000000"

// NewNetSource returns a source in the Unavailable state until its stream
// produces two frames to difference. topN ≤ 0 defaults to 5.
func NewNetSource(topN int) *NetSource {
	if topN <= 0 {
		topN = 5
	}
	return &NetSource{topN: topN, state: SourceState{Health: HealthUnavailable, Reason: "not started"}}
}

// Latest returns the most recent top-N talkers and the source health.
func (ns *NetSource) Latest() ([]NetProc, SourceState) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()
	return ns.latest, ns.state
}

func (ns *NetSource) set(procs []NetProc) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.latest = procs
	ns.state = SourceState{Health: HealthLive, LastSample: time.Now()}
}

func (ns *NetSource) setHealth(h Health, reason string) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.state.Health = h
	ns.state.Reason = reason
}

// netRow is one process's cumulative counters within a single nettop frame.
type netRow struct {
	ts       time.Time
	in, out  uint64
	name     string
	pid      int
}

// run consumes one nettop stream from r, emitting a fresh top-N each time a frame
// completes. A read stall (no line within deadline) marks the source stalled and
// returns so the supervisor can relaunch it (§7.3). Lines are pumped through a
// goroutine so the deadline applies to the read, not just to ctx.
func (ns *NetSource) run(ctx context.Context, r io.Reader, deadline time.Duration) error {
	// The reader goroutine pushes every line, then closes the channel — so a
	// closed channel (EOF) is only observed after all buffered lines are drained,
	// never racing them. readErr is published before close, so it is visible once
	// the closed channel is seen.
	lines := make(chan string, 64)
	var readErr error
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			select {
			case lines <- sc.Text():
			case <-ctx.Done():
				return
			}
		}
		readErr = sc.Err()
	}()

	inCol, outCol := -1, -1
	var prev, cur map[string]netRow
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	for {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(deadline)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			ns.setHealth(HealthStalled, "nettop stalled mid-frame")
			return nil
		case ln, ok := <-lines:
			if !ok {
				return readErr
			}
			if isNettopHeader(ln) {
				// A header opens a new frame; whatever is in cur just completed.
				if len(cur) > 0 {
					if prev != nil {
						ns.set(diffNetFrames(prev, cur, ns.topN))
					}
					prev = cur
				}
				cur = map[string]netRow{}
				inCol, outCol = nettopByteCols(ln)
				continue
			}
			if cur == nil || inCol < 0 || outCol < 0 {
				continue
			}
			if label, row, ok := parseNetRow(ln, inCol, outCol); ok {
				cur[label] = row
			}
		}
	}
}

// isNettopHeader reports whether a line is the CSV column header that opens each
// nettop frame.
func isNettopHeader(ln string) bool {
	return strings.HasPrefix(ln, "time,")
}

// nettopByteCols finds the bytes_in / bytes_out column indices from a header.
func nettopByteCols(header string) (int, int) {
	in, out := -1, -1
	for i, h := range strings.Split(header, ",") {
		switch strings.TrimSpace(h) {
		case "bytes_in":
			in = i
		case "bytes_out":
			out = i
		}
	}
	return in, out
}

// parseNetRow parses one data row into its label and cumulative counters,
// degrading to ok=false on anything unexpected (§13.1).
func parseNetRow(ln string, inCol, outCol int) (string, netRow, bool) {
	cols := strings.Split(ln, ",")
	if len(cols) <= inCol || len(cols) <= outCol {
		return "", netRow{}, false
	}
	ts, label := cols[0], cols[1]
	if label == "" || ts == "" || ts[0] < '0' || ts[0] > '9' {
		return "", netRow{}, false
	}
	t, err := time.Parse(nettopFrameTime, ts)
	if err != nil {
		return "", netRow{}, false
	}
	name, pid := splitNettopLabel(label)
	return label, netRow{
		ts:   t,
		in:   parseNetUint(cols[inCol]),
		out:  parseNetUint(cols[outCol]),
		name: name,
		pid:  pid,
	}, true
}

// diffNetFrames differences two consecutive frames into per-process rates, sorted
// by total throughput, capped at topN.
func diffNetFrames(prev, cur map[string]netRow, topN int) []NetProc {
	var out []NetProc
	for label, c := range cur {
		p, ok := prev[label]
		if !ok {
			continue
		}
		dt := c.ts.Sub(p.ts).Seconds()
		if dt <= 0 {
			continue
		}
		rx := float64(satSubU(c.in, p.in)) / dt
		tx := float64(satSubU(c.out, p.out)) / dt
		if rx == 0 && tx == 0 {
			continue
		}
		out = append(out, NetProc{Name: c.name, PID: c.pid, RxBytesPerSec: rx, TxBytesPerSec: tx})
	}
	sort.Slice(out, func(i, j int) bool {
		return (out[i].RxBytesPerSec + out[i].TxBytesPerSec) > (out[j].RxBytesPerSec + out[j].TxBytesPerSec)
	})
	if len(out) > topN {
		out = out[:topN]
	}
	return out
}

// splitNettopLabel splits a nettop "name.pid" label on its last dot.
func splitNettopLabel(label string) (string, int) {
	i := strings.LastIndex(label, ".")
	if i < 0 {
		return label, 0
	}
	pid, _ := strconv.Atoi(label[i+1:])
	return label[:i], pid
}

func parseNetUint(s string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return n
}

// satSubU avoids underflow when a counter resets between frames.
func satSubU(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}

// Supervise runs the source under a watchdog: launch → decode → on stall/EOF mark
// the state and relaunch with exponential backoff, until ctx is cancelled (§7.4).
// nettop is unprivileged, so a launch failure is rare; it is surfaced as
// Unavailable and retried rather than failing hard.
func (ns *NetSource) Supervise(ctx context.Context, launch Launcher, deadline time.Duration) {
	backoff := 500 * time.Millisecond
	const maxBackoff = 10 * time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		ns.setHealth(HealthRestarting, "starting nettop")
		rc, err := launch(ctx)
		if err != nil {
			ns.setHealth(HealthUnavailable, "nettop unavailable: "+err.Error())
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, maxBackoff)
			continue
		}
		runErr := ns.run(ctx, rc, deadline)
		_ = rc.Close() // terminates the subprocess
		if ctx.Err() != nil {
			return
		}
		if runErr == nil {
			ns.setHealth(HealthUnavailable, "nettop exited")
		}
		if !sleepCtx(ctx, backoff) {
			return
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}
