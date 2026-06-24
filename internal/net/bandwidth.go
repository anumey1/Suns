package net

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"
)

// Bandwidth detector (§12.7). The RELIABLE CORE is per-interface throughput from
// the kernel's interface counters (gopsutil net.IOCounters). Per-process
// attribution is EXPERIMENTAL: it parses two samples of a `nettop` stream and
// differences them, and degrades cleanly to "unavailable" rather than show wrong
// numbers when nettop is missing or its output is unexpected (§7.5).

// nettopTimeLayout is the per-row timestamp format in `nettop -x` output.
const nettopTimeLayout = "15:04:05.000000"

// InterfaceBW is per-interface throughput.
type InterfaceBW struct {
	Name          string  `json:"name"`
	RxBytesPerSec float64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec float64 `json:"tx_bytes_per_sec"`
}

// ProcBW is per-process throughput (experimental).
type ProcBW struct {
	Name          string  `json:"name"`
	PID           int     `json:"pid"`
	RxBytesPerSec float64 `json:"rx_bytes_per_sec"`
	TxBytesPerSec float64 `json:"tx_bytes_per_sec"`
}

// BWReport is the read-only bandwidth picture.
type BWReport struct {
	IntervalSeconds    float64       `json:"interval_seconds"`
	Interfaces         []InterfaceBW `json:"interfaces"`
	Processes          []ProcBW      `json:"processes,omitempty"`
	ProcessesAvailable bool          `json:"processes_available"` // false → nettop unavailable
}

// BWOptions controls a bandwidth sample.
type BWOptions struct {
	Interval time.Duration // sampling window; 0 → 1s
	TopN     int           // max per-process rows; 0 → 10
}

func (o BWOptions) interval() time.Duration {
	if o.Interval <= 0 {
		return time.Second
	}
	return o.Interval
}

func (o BWOptions) topN() int {
	if o.TopN <= 0 {
		return 10
	}
	return o.TopN
}

// Bandwidth measures interface throughput over one window and, in parallel,
// attempts per-process attribution via nettop. The two run concurrently so the
// whole call takes about one interval.
func Bandwidth(ctx context.Context, r Runner, opts BWOptions) (BWReport, error) {
	interval := opts.interval()

	type ntResult struct {
		procs []ProcBW
		ok    bool
	}
	ntCh := make(chan ntResult, 1)
	go func() {
		procs, ok := sampleNettop(ctx, r, opts.topN())
		ntCh <- ntResult{procs: procs, ok: ok}
	}()

	ifaces, secs := sampleInterfaces(ctx, interval)
	nt := <-ntCh

	rep := BWReport{IntervalSeconds: secs, Interfaces: ifaces, ProcessesAvailable: nt.ok}
	if nt.ok {
		rep.Processes = nt.procs
	}
	return rep, nil
}

// sampleInterfaces reads the kernel interface counters, waits one window, reads
// them again, and returns per-interface rates plus the elapsed seconds. Idle
// interfaces (no traffic in the window) are omitted.
func sampleInterfaces(ctx context.Context, interval time.Duration) ([]InterfaceBW, float64) {
	first, err := psnet.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, 0
	}
	start := time.Now()
	select {
	case <-ctx.Done():
		return nil, 0
	case <-time.After(interval):
	}
	second, err := psnet.IOCountersWithContext(ctx, true)
	if err != nil {
		return nil, 0
	}
	secs := time.Since(start).Seconds()
	if secs <= 0 {
		return nil, 0
	}

	prev := map[string]psnet.IOCountersStat{}
	for _, c := range first {
		prev[c.Name] = c
	}
	var out []InterfaceBW
	for _, c := range second {
		p, ok := prev[c.Name]
		if !ok {
			continue
		}
		rx := float64(satSub(c.BytesRecv, p.BytesRecv)) / secs
		tx := float64(satSub(c.BytesSent, p.BytesSent)) / secs
		if rx == 0 && tx == 0 {
			continue
		}
		out = append(out, InterfaceBW{Name: c.Name, RxBytesPerSec: rx, TxBytesPerSec: tx})
	}
	sort.Slice(out, func(i, j int) bool {
		return (out[i].RxBytesPerSec + out[i].TxBytesPerSec) > (out[j].RxBytesPerSec + out[j].TxBytesPerSec)
	})
	return out, secs
}

// sampleNettop runs a two-sample nettop capture and differences it. It returns
// ok=false on any trouble so the caller degrades to interface-only output.
func sampleNettop(ctx context.Context, r Runner, topN int) ([]ProcBW, bool) {
	res, err := r.Run(ctx, "nettop", "-P", "-x", "-L", "2")
	if err != nil && len(res.Stdout) == 0 {
		return nil, false
	}
	procs := parseNettop(res.Stdout)
	if len(procs) == 0 {
		return nil, false
	}
	sort.Slice(procs, func(i, j int) bool {
		return (procs[i].RxBytesPerSec + procs[i].TxBytesPerSec) > (procs[j].RxBytesPerSec + procs[j].TxBytesPerSec)
	})
	if len(procs) > topN {
		procs = procs[:topN]
	}
	return procs, true
}

// parseNettop differences the two samples in `nettop -P -x -L 2` CSV output. Each
// process appears once per sample (label in column 1 = "name.pid"); the rate is
// (sample2 − sample1) / elapsed, using the rows' own timestamps for elapsed.
func parseNettop(out []byte) []ProcBW {
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return nil
	}
	inCol, outCol := -1, -1
	for i, h := range strings.Split(lines[0], ",") {
		switch strings.TrimSpace(h) {
		case "bytes_in":
			inCol = i
		case "bytes_out":
			outCol = i
		}
	}
	if inCol < 0 || outCol < 0 {
		return nil
	}

	type acc struct {
		name      string
		pid       int
		t1, t2    time.Time
		in1, out1 uint64
		in2, out2 uint64
		seen      int
	}
	procs := map[string]*acc{}
	var order []string

	for _, ln := range lines[1:] {
		cols := strings.Split(ln, ",")
		if len(cols) <= outCol || len(cols) <= inCol {
			continue
		}
		ts, label := cols[0], cols[1]
		if label == "" || ts == "" || ts[0] < '0' || ts[0] > '9' {
			continue
		}
		t, err := time.Parse(nettopTimeLayout, ts)
		if err != nil {
			continue
		}
		in, out := parseUint(cols[inCol]), parseUint(cols[outCol])

		a := procs[label]
		if a == nil {
			name, pid := splitNamePID(label)
			a = &acc{name: name, pid: pid}
			procs[label] = a
			order = append(order, label)
		}
		if a.seen == 0 {
			a.t1, a.in1, a.out1 = t, in, out
		} else {
			a.t2, a.in2, a.out2 = t, in, out
		}
		a.seen++
	}

	var result []ProcBW
	for _, label := range order {
		a := procs[label]
		if a.seen < 2 {
			continue
		}
		dt := a.t2.Sub(a.t1).Seconds()
		if dt <= 0 {
			continue
		}
		result = append(result, ProcBW{
			Name:          a.name,
			PID:           a.pid,
			RxBytesPerSec: float64(satSub(a.in2, a.in1)) / dt,
			TxBytesPerSec: float64(satSub(a.out2, a.out1)) / dt,
		})
	}
	return result
}

// splitNamePID splits a nettop "name.pid" label on its last dot.
func splitNamePID(label string) (string, int) {
	i := strings.LastIndex(label, ".")
	if i < 0 {
		return label, 0
	}
	pid, _ := strconv.Atoi(label[i+1:])
	return label[:i], pid
}

func parseUint(s string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	return n
}

// satSub avoids underflow when a counter resets between samples.
func satSub(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
