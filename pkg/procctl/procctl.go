// Package procctl is process introspection and the atomic validate-and-signal
// primitive behind ProcessKill (§4.7, §12.8).
//
// Killing a process is identity-sensitive: between discovery and the kill, the
// OS can recycle a PID to a different process. ValidateAndSignal re-reads the
// target's identity (birth time + executable path) and only sends the signal if
// it still matches — defeating PID reuse. Because the read and the signal happen
// in the same process invocation, this is the atomic unit the privilege
// chokepoint runs under elevation for root/other-user targets (§4.7).
package procctl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/shirou/gopsutil/v4/process"
)

// Current reads the live identity of pid plus its owner UID and status.
func Current(pid int) (id identity.ProcessIdent, ownerUID int, status string, err error) {
	p, err := process.NewProcess(int32(pid))
	if err != nil {
		return identity.ProcessIdent{}, -1, "", err
	}
	ct, err := p.CreateTime() // ms since the Unix epoch
	if err != nil {
		return identity.ProcessIdent{}, -1, "", err
	}
	exe, _ := p.Exe()
	id = identity.ProcessIdent{PID: pid, Birth: time.UnixMilli(ct), Exec: exe}

	ownerUID = -1
	if uids, e := p.Uids(); e == nil && len(uids) > 0 {
		ownerUID = int(uids[0])
	}
	if st, e := p.Status(); e == nil {
		status = strings.Join(st, ",")
	}
	return id, ownerUID, status, nil
}

// IsOwnUser reports whether ownerUID is the current user (the unprivileged
// fast path for validation and kill, §4.7).
func IsOwnUser(ownerUID int) bool { return ownerUID >= 0 && ownerUID == os.Getuid() }

// ValidateAndSignal re-reads the target's identity, compares it to expect, and
// sends sig only if birth time and executable path still match. A mismatch
// returns identity.ErrIdentityMismatch and sends NOTHING (PID-reuse defense).
func ValidateAndSignal(expect identity.ProcessIdent, sig syscall.Signal) error {
	cur, _, _, err := Current(expect.PID)
	if err != nil {
		return fmt.Errorf("procctl: target %d gone: %w", expect.PID, err)
	}
	if !cur.Birth.Equal(expect.Birth) || cur.Exec != expect.Exec {
		return fmt.Errorf("%w: PID %d now refers to a different process", identity.ErrIdentityMismatch, expect.PID)
	}
	return syscall.Kill(expect.PID, sig)
}

// Row is one process in the inspector listing (§12.8).
type Row struct {
	PID     int
	PPID    int
	Name    string
	CPU     float64
	Mem     uint64
	OwnerID int
	Status  string
	Zombie  bool // already dead, awaiting reaping — cannot be kill -9'd
	HighCPU bool // sustained-high-CPU heuristic (runaway candidate)
}

// runawayCPUThreshold flags a process as a runaway candidate. True runaway
// detection (sustained high CPU with low I/O) needs history; this is the
// single-sample approximation surfaced as a hint (§12.8).
const runawayCPUThreshold = 90.0

// List enumerates processes for the inspector, flagging zombies and high-CPU
// candidates. Per-process errors are tolerated (the process may vanish mid-walk).
func List(ctx context.Context) ([]Row, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]Row, 0, len(procs))
	for _, p := range procs {
		name, _ := p.NameWithContext(ctx)
		cpu, _ := p.CPUPercentWithContext(ctx)
		ppid, _ := p.PpidWithContext(ctx)
		var rss uint64
		if mi, e := p.MemoryInfoWithContext(ctx); e == nil && mi != nil {
			rss = mi.RSS
		}
		owner := -1
		if uids, e := p.UidsWithContext(ctx); e == nil && len(uids) > 0 {
			owner = int(uids[0])
		}
		var status string
		if st, e := p.StatusWithContext(ctx); e == nil {
			status = strings.Join(st, ",")
		}
		zombie := strings.Contains(strings.ToLower(status), "zombie") || status == "Z"
		rows = append(rows, Row{
			PID: int(p.Pid), PPID: int(ppid), Name: name, CPU: cpu, Mem: rss,
			OwnerID: owner, Status: status, Zombie: zombie,
			HighCPU: cpu >= runawayCPUThreshold && !zombie,
		})
	}
	return rows, nil
}
