// Package doctor implements `suns doctor`: a read-only environment, permission,
// tool-version, and capability self-check (§9.3). It never modifies anything.
package doctor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Status is the outcome of a single check.
type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
)

// Check is one self-check result.
type Check struct {
	Name   string
	Status Status
	Detail string
}

// Report is the full set of checks.
type Report struct {
	Checks []Check
}

// OK reports whether no check failed.
func (r Report) OK() bool {
	for _, c := range r.Checks {
		if c.Status == Fail {
			return false
		}
	}
	return true
}

// pinnedTools are the external binaries Suns relies on; doctor reports whether
// each is present at its pinned path (§6.3).
var pinnedTools = []string{
	"/usr/sbin/lsof",
	"/usr/sbin/arp",
	"/usr/bin/nettop",
	"/usr/bin/powermetrics",
	"/usr/sbin/pkgutil",
	"/usr/bin/dscacheutil",
	"/usr/bin/log",
	"/usr/bin/csrutil",
	"/usr/sbin/spctl",
	"/usr/bin/fdesetup",
	"/usr/bin/pmset",
	"/usr/bin/vm_stat",
	"/usr/sbin/sysctl",
	"/bin/launchctl",
	"/usr/bin/sudo",
	"/usr/bin/git",
}

// Run executes all checks and returns a Report.
func Run(ctx context.Context) Report {
	var r Report
	add := func(name string, status Status, detail string) {
		r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
	}

	// Platform.
	if runtime.GOOS == "darwin" {
		add("platform", OK, fmt.Sprintf("macOS / %s", runtime.GOARCH))
	} else {
		add("platform", Warn, fmt.Sprintf("%s/%s — Suns targets macOS", runtime.GOOS, runtime.GOARCH))
	}
	add("go runtime", OK, runtime.Version())

	// Home, Trash, and history locations.
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		add("home directory", Fail, "could not resolve $HOME")
		return r
	}
	add("home directory", OK, home)

	trashDir := filepath.Join(home, ".Trash")
	if writable(trashDir) {
		add("trash directory", OK, trashDir)
	} else {
		add("trash directory", Warn, trashDir+" — not present/writable; pure-Go fallback will create it")
	}

	histDir := filepath.Join(home, "Library", "Application Support", "Suns")
	add("history directory", OK, filepath.Join(histDir, "history.jsonl"))

	// External tools.
	var missing []string
	for _, p := range pinnedTools {
		if _, err := os.Stat(p); err != nil {
			missing = append(missing, filepath.Base(p))
		}
	}
	if len(missing) == 0 {
		add("external tools", OK, fmt.Sprintf("%d/%d present", len(pinnedTools), len(pinnedTools)))
	} else {
		add("external tools", Warn, fmt.Sprintf("missing: %v", missing))
	}

	// Full Disk Access hint — a single bounded probe of a representative
	// protected path (§5.1). This is heuristic: a hint only.
	fdaStatus, fdaDetail := fdaProbe(ctx, home)
	add("full disk access", fdaStatus, fdaDetail)

	return r
}

// writable reports whether dir exists and a temp file can be created in it.
func writable(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	f, err := os.CreateTemp(dir, ".suns-doctor-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// fdaProbe reads a representative TCC-protected directory under a short
// deadline. EPERM ("operation not permitted") indicates limited mode; success
// indicates Full Disk Access is likely granted to the host terminal. The probe
// is bounded by a goroutine + timeout so a stalled path cannot hang doctor.
func fdaProbe(ctx context.Context, home string) (Status, string) {
	target := filepath.Join(home, "Library", "Mail")
	if _, err := os.Stat(target); os.IsNotExist(err) {
		// No Mail data on this machine; fall back to another protected area.
		target = filepath.Join(home, "Library", "Application Support", "com.apple.TCC")
	}

	type result struct{ err error }
	ch := make(chan result, 1)
	go func() {
		_, err := os.ReadDir(target)
		ch <- result{err: err}
	}()

	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	select {
	case <-probeCtx.Done():
		return Warn, "probe timed out — treat as unknown"
	case res := <-ch:
		switch {
		case res.err == nil:
			return OK, "representative protected path readable (FDA likely granted)"
		case os.IsPermission(res.err):
			return Warn, "limited mode — grant Full Disk Access to your terminal in System Settings"
		case os.IsNotExist(res.err):
			return OK, "no representative protected path present to probe"
		default:
			return Warn, fmt.Sprintf("inconclusive: %v", res.err)
		}
	}
}
