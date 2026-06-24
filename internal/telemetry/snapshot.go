package telemetry

import "time"

// Health is a per-source supervised state (§7.4). The dashboard renders a
// staleness/health badge from this so dead data is never shown as live.
type Health string

const (
	HealthLive        Health = "live"
	HealthStale       Health = "stale"
	HealthStalled     Health = "stalled"
	HealthRestarting  Health = "restarting"
	HealthUnavailable Health = "unavailable"
)

// SourceState carries a source's health, the timestamp of its last good
// sample, and a human reason when degraded.
type SourceState struct {
	Health     Health
	LastSample time.Time
	Reason     string
}

// Ring is a fixed-size circular buffer feeding the sparklines — bounded memory,
// cheap (§7.2). Values are returned oldest→newest.
type Ring struct {
	buf    []float64
	filled int
	head   int
}

// NewRing returns a Ring holding at most size samples.
func NewRing(size int) *Ring {
	if size < 1 {
		size = 1
	}
	return &Ring{buf: make([]float64, size)}
}

// Push appends v, overwriting the oldest sample when full.
func (r *Ring) Push(v float64) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	if r.filled < len(r.buf) {
		r.filled++
	}
}

// Values returns the samples oldest→newest (a fresh slice; safe to keep).
func (r *Ring) Values() []float64 {
	out := make([]float64, r.filled)
	start := (r.head - r.filled + len(r.buf)) % len(r.buf)
	for i := 0; i < r.filled; i++ {
		out[i] = r.buf[(start+i)%len(r.buf)]
	}
	return out
}

// CPU is aggregate + per-core utilization plus a rolling history.
type CPU struct {
	Percent float64
	PerCore []float64
	History []float64
}

// Memory is the macOS memory picture (§12.10).
type Memory struct {
	UsedPercent float64
	Total       uint64
	Used        uint64
	Available   uint64
	SwapUsed    uint64
	SwapTotal   uint64
	History     []float64
}

// DiskUsage is per-volume capacity.
type DiskUsage struct {
	Path        string
	Total       uint64
	Used        uint64
	Free        uint64
	UsedPercent float64
}

// DiskIO is read/write throughput with rolling history.
type DiskIO struct {
	ReadBytesPerSec  float64
	WriteBytesPerSec float64
	ReadHistory      []float64
	WriteHistory     []float64
}

// ProcInfo is one row of the top-processes table.
type ProcInfo struct {
	PID  int32
	Name string
	CPU  float64
	Mem  uint64
}

// NetProc is one row of the experimental per-process network attribution, fed by
// the supervised nettop stream (§3.3, §7.5). It is EXPERIMENTAL: present only
// while the stream is live and labelled as such in the UI.
type NetProc struct {
	Name          string
	PID           int
	RxBytesPerSec float64
	TxBytesPerSec float64
}

// Host is uptime + load averages.
type Host struct {
	Uptime uint64
	Load1  float64
	Load5  float64
	Load15 float64
}

// Battery is charge state from pmset (§12, widget 6). Present is false on
// desktops; wattage/cycle/health (ioreg) are a later enhancement.
type Battery struct {
	Present       bool
	Percent       float64
	State         string // charging | discharging | charged
	TimeRemaining string // "H:MM" or "" when unknown
	OnAC          bool
}

// Power is the powermetrics-derived picture (§7.2): GPU utilization, package/
// system power draw, on-die temperatures, and the throttle flag. Populated only
// while the elevated powermetrics stream is live.
type Power struct {
	GPUPercent   float64
	PackageWatts float64
	SystemWatts  float64
	CPUTempC     float64
	GPUTempC     float64
	Throttled    bool
}

// SystemSnapshot is the single immutable picture every widget reads (§7.2). It
// is published via atomic.Pointer and never mutated after publish.
type SystemSnapshot struct {
	Time     time.Time
	CPU      CPU
	Memory   Memory
	Disks    []DiskUsage
	DiskIO   DiskIO
	TopProcs []ProcInfo
	Host     Host
	Battery  Battery
	Power    Power
	NetProcs []NetProc // experimental per-process bandwidth (nettop), if streaming

	// Sources carries health for each named source. The heavy/root sources
	// (powermetrics GPU/thermal/power, IOKit, nettop) are present as
	// Unavailable until their live streaming is wired and elevated (§7.2).
	Sources map[string]SourceState
}

// Source names.
const (
	SrcCPU     = "cpu"
	SrcMemory  = "memory"
	SrcDisk    = "disk"
	SrcProcess = "process"
	SrcThermal = "thermal"
	SrcGPU     = "gpu"
	SrcPower   = "power"
	SrcBattery = "battery"
	SrcNetwork = "network"
)
