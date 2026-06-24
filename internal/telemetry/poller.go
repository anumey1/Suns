package telemetry

import (
	"context"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

const historyLen = 60 // ~60 samples per sparkline (§7.2)

// Poller builds one SystemSnapshot per cadence and publishes it via
// atomic.Pointer; every widget reads the same snapshot, so there is no
// per-widget polling and no redundant syscalls (§7.2). The cheap in-process
// gopsutil sources run here; heavy/root sources are reported Unavailable until
// their live streaming is wired and elevated.
type Poller struct {
	interval time.Duration
	snap     atomic.Pointer[SystemSnapshot]

	cpuHist   *Ring
	memHist   *Ring
	diskRHist *Ring
	diskWHist *Ring

	prevIORead  uint64
	prevIOWrite uint64
	prevIOTime  time.Time
	maxProcs    int

	powerMu  sync.RWMutex
	powerSrc *PowerSource // optional; set once elevated (§7.2)
}

// AttachPowerSource wires a supervised powermetrics source whose latest sample
// is merged into subsequent snapshots. Called after elevation (§11 `e`).
func (p *Poller) AttachPowerSource(ps *PowerSource) {
	p.powerMu.Lock()
	defer p.powerMu.Unlock()
	p.powerSrc = ps
}

func (p *Poller) power() *PowerSource {
	p.powerMu.RLock()
	defer p.powerMu.RUnlock()
	return p.powerSrc
}

// New returns a Poller with the given cadence (default 1s).
func New(interval time.Duration) *Poller {
	if interval <= 0 {
		interval = time.Second
	}
	return &Poller{
		interval:  interval,
		cpuHist:   NewRing(historyLen),
		memHist:   NewRing(historyLen),
		diskRHist: NewRing(historyLen),
		diskWHist: NewRing(historyLen),
		maxProcs:  10,
	}
}

// Snapshot returns the most recently published snapshot (lock-free), or nil
// before the first sample.
func (p *Poller) Snapshot() *SystemSnapshot { return p.snap.Load() }

// Run samples on the cadence until ctx is cancelled, publishing each snapshot
// atomically. It primes the CPU counters first so the first sample is a real
// delta rather than since-boot.
func (p *Poller) Run(ctx context.Context) {
	_, _ = cpu.PercentWithContext(ctx, 0, true)
	p.prevIOTime = time.Now()
	p.readDiskBaseline(ctx)

	p.snap.Store(p.sample(ctx))
	t := time.NewTicker(p.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			p.snap.Store(p.sample(ctx))
		}
	}
}

func (p *Poller) readDiskBaseline(ctx context.Context) {
	if io, err := disk.IOCountersWithContext(ctx); err == nil {
		var r, w uint64
		for _, c := range io {
			r += c.ReadBytes
			w += c.WriteBytes
		}
		p.prevIORead, p.prevIOWrite = r, w
	}
}

// sample builds one snapshot. Each source is independent: an error marks that
// source Unavailable rather than failing the whole snapshot (§13.2).
func (p *Poller) sample(ctx context.Context) *SystemSnapshot {
	now := time.Now()
	s := &SystemSnapshot{Time: now, Sources: map[string]SourceState{}}

	// CPU (per-core in one measurement; aggregate is the mean).
	if per, err := cpu.PercentWithContext(ctx, 0, true); err == nil && len(per) > 0 {
		var sum float64
		for _, v := range per {
			sum += v
		}
		agg := sum / float64(len(per))
		p.cpuHist.Push(agg)
		s.CPU = CPU{Percent: agg, PerCore: per, History: p.cpuHist.Values()}
		s.Sources[SrcCPU] = SourceState{Health: HealthLive, LastSample: now}
	} else {
		s.Sources[SrcCPU] = SourceState{Health: HealthUnavailable, Reason: "cpu read failed"}
	}

	// Memory.
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		p.memHist.Push(vm.UsedPercent)
		s.Memory = Memory{
			UsedPercent: vm.UsedPercent, Total: vm.Total, Used: vm.Used, Available: vm.Available,
			History: p.memHist.Values(),
		}
		if sw, err := mem.SwapMemoryWithContext(ctx); err == nil {
			s.Memory.SwapUsed, s.Memory.SwapTotal = sw.Used, sw.Total
		}
		s.Sources[SrcMemory] = SourceState{Health: HealthLive, LastSample: now}
	} else {
		s.Sources[SrcMemory] = SourceState{Health: HealthUnavailable, Reason: "memory read failed"}
	}

	// Disk usage (every mounted physical volume) + I/O rates.
	s.Disks = readVolumes(ctx)
	if len(s.Disks) > 0 {
		s.Sources[SrcDisk] = SourceState{Health: HealthLive, LastSample: now}
	} else {
		s.Sources[SrcDisk] = SourceState{Health: HealthUnavailable, Reason: "disk usage read failed"}
	}
	if io, err := disk.IOCountersWithContext(ctx); err == nil {
		var r, w uint64
		for _, c := range io {
			r += c.ReadBytes
			w += c.WriteBytes
		}
		elapsed := now.Sub(p.prevIOTime).Seconds()
		if elapsed > 0 && p.prevIOTime.Before(now) && (p.prevIORead > 0 || p.prevIOWrite > 0) {
			rps := float64(saturatingSub(r, p.prevIORead)) / elapsed
			wps := float64(saturatingSub(w, p.prevIOWrite)) / elapsed
			p.diskRHist.Push(rps)
			p.diskWHist.Push(wps)
			s.DiskIO = DiskIO{
				ReadBytesPerSec: rps, WriteBytesPerSec: wps,
				ReadHistory: p.diskRHist.Values(), WriteHistory: p.diskWHist.Values(),
			}
		}
		p.prevIORead, p.prevIOWrite, p.prevIOTime = r, w, now
	}

	// Host: uptime + load.
	if hi, err := host.InfoWithContext(ctx); err == nil {
		s.Host.Uptime = hi.Uptime
	}
	if la, err := load.AvgWithContext(ctx); err == nil {
		s.Host.Load1, s.Host.Load5, s.Host.Load15 = la.Load1, la.Load5, la.Load15
	}

	// Top processes by CPU.
	s.TopProcs = topProcesses(ctx, p.maxProcs)
	if s.TopProcs != nil {
		s.Sources[SrcProcess] = SourceState{Health: HealthLive, LastSample: now}
	}

	// Battery via pmset (read-only, no elevation).
	if b, ok := fetchBattery(ctx); ok {
		s.Battery = b
		s.Sources[SrcBattery] = SourceState{Health: HealthLive, LastSample: now}
	} else {
		s.Sources[SrcBattery] = SourceState{Health: HealthUnavailable, Reason: "no battery (desktop)"}
	}

	// GPU / thermal / power from the supervised powermetrics stream, if it has
	// been attached (after elevation). Otherwise honestly Unavailable.
	if ps := p.power(); ps != nil {
		pw, st := ps.Latest()
		s.Power = pw
		for _, src := range []string{SrcGPU, SrcThermal, SrcPower} {
			s.Sources[src] = st
		}
	} else {
		for _, src := range []string{SrcGPU, SrcThermal, SrcPower} {
			s.Sources[src] = SourceState{Health: HealthUnavailable, Reason: "press e to elevate"}
		}
	}

	// Per-process network attribution remains experimental / unwired (§7.5).
	s.Sources[SrcNetwork] = SourceState{Health: HealthUnavailable, Reason: "experimental; not yet wired"}
	return s
}

func topProcesses(ctx context.Context, n int) []ProcInfo {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil
	}
	infos := make([]ProcInfo, 0, len(procs))
	for _, pr := range procs {
		cpuPct, _ := pr.CPUPercentWithContext(ctx)
		name, _ := pr.NameWithContext(ctx)
		var rss uint64
		if mi, err := pr.MemoryInfoWithContext(ctx); err == nil && mi != nil {
			rss = mi.RSS
		}
		infos = append(infos, ProcInfo{PID: pr.Pid, Name: name, CPU: cpuPct, Mem: rss})
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].CPU > infos[j].CPU })
	if len(infos) > n {
		infos = infos[:n]
	}
	return infos
}

// readVolumes reports usage for every mounted physical volume (§3.3), with the
// root volume first. It falls back to root-only if partition enumeration fails.
// Synthetic/virtual mounts (no /dev device) are skipped, and each mountpoint is
// reported once.
func readVolumes(ctx context.Context) []DiskUsage {
	var vols []DiskUsage
	seen := map[string]bool{}
	if parts, err := disk.PartitionsWithContext(ctx, false); err == nil {
		for _, pt := range parts {
			if seen[pt.Mountpoint] || !strings.HasPrefix(pt.Device, "/dev/") {
				continue
			}
			u, err := disk.UsageWithContext(ctx, pt.Mountpoint)
			if err != nil || u.Total == 0 {
				continue
			}
			seen[pt.Mountpoint] = true
			vols = append(vols, DiskUsage{Path: pt.Mountpoint, Total: u.Total, Used: u.Used, Free: u.Free, UsedPercent: u.UsedPercent})
		}
	}
	if len(vols) == 0 {
		if u, err := disk.UsageWithContext(ctx, "/"); err == nil {
			vols = append(vols, DiskUsage{Path: "/", Total: u.Total, Used: u.Used, Free: u.Free, UsedPercent: u.UsedPercent})
		}
	}
	// Root volume first; the rest by mountpoint.
	sort.SliceStable(vols, func(i, j int) bool {
		if (vols[i].Path == "/") != (vols[j].Path == "/") {
			return vols[i].Path == "/"
		}
		return vols[i].Path < vols[j].Path
	})
	return vols
}

// saturatingSub avoids underflow when counters reset.
func saturatingSub(a, b uint64) uint64 {
	if a < b {
		return 0
	}
	return a - b
}
