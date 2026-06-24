// Package orphans backs `suns orphans` — the orphaned launch-agent purge
// (§12.3, Phases §2.2). Destructive · gated · ServiceUnload + FileDelete.
//
// It scans the user and system launchd directories, reads each job plist
// binary-safe, resolves the job's executable (handling env-launchers, declining
// shell wrappers and relative paths it cannot resolve), and flags a job as
// orphaned ONLY when its resolved, absolute executable is genuinely absent on
// disk. Apple-managed jobs are skipped. For each orphan it emits a
// `launchctl bootout` op BEFORE the plist's FileDelete, so unloading always
// precedes removal.
//
// It reports, it does not guarantee: launchd domains, disabled states, and
// update-time regeneration can race a bootout-then-remove, so the UI states
// these bounds (§2.2).
package orphans

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plist"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// Options configures a scan. Zero values select production defaults.
type Options struct {
	Home      string   // home dir override (tests); default os.UserHomeDir()
	UID       int      // gui-domain uid (tests); default os.Getuid()
	Dirs      []string // scan dirs override (tests)
	Threshold int64    // tiered-identity threshold; 0 → identity.DefaultLargeThreshold
}

// Orphan describes one launchd job whose executable is missing.
type Orphan struct {
	Plist       string
	Label       string
	Domain      string
	Privileged  bool
	MissingExec string
}

// Report is the read-only outcome of Find.
type Report struct {
	Orphans []Orphan
	// Ops are ServiceUnload (bootout) followed by FileDelete (plist), in that
	// order, ready for plan.New.
	Ops    []operation.Operation
	Bounds []string
}

type launchdJob struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
	Disabled         bool     `plist:"Disabled"`
}

// Find scans the launchd directories and returns the orphaned jobs plus an
// ordered op set. It is read-only and performs no deletion.
func Find(ctx context.Context, opts Options) (Report, error) {
	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}
	if opts.UID == 0 {
		opts.UID = os.Getuid()
	}
	if opts.Threshold == 0 {
		opts.Threshold = identity.DefaultLargeThreshold
	}
	if len(opts.Dirs) == 0 {
		opts.Dirs = []string{
			filepath.Join(opts.Home, "Library", "LaunchAgents"),
			"/Library/LaunchAgents",
			"/Library/LaunchDaemons",
		}
	}

	var orphans []Orphan
	for _, dir := range opts.Dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			select {
			case <-ctx.Done():
				return Report{}, ctx.Err()
			default:
			}
			if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".plist") {
				continue
			}
			path := filepath.Join(dir, ent.Name())
			if o, ok := classify(path, opts.UID); ok {
				orphans = append(orphans, o)
			}
		}
	}

	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Plist < orphans[j].Plist })

	var service, files []operation.Operation
	for _, o := range orphans {
		service = append(service, operation.ServiceUnloadOp{
			Domain: o.Domain, Label: o.Label, Plist: o.Plist, Privileged: o.Privileged,
		})
		if op, ok := fileDelete(o.Plist, opts.Threshold); ok {
			files = append(files, op)
		}
	}
	ops := append(service, files...) // bootout before plist removal

	return Report{
		Orphans: orphans,
		Ops:     ops,
		Bounds: []string{
			"launchd state can race: a job may be re-registered by its app or an update after removal.",
			"Disabled jobs and non-standard domains are reported, not guaranteed unloaded.",
			"Jobs launched via shell scripts or relative/PATH-resolved programs are left alone (unresolvable).",
		},
	}, nil
}

// classify decides whether the plist at path defines an orphaned job.
func classify(path string, uid int) (Orphan, bool) {
	if isAppleManaged(path) {
		return Orphan{}, false
	}
	var job launchdJob
	if plist.Decode(path, &job) != nil || job.Label == "" {
		return Orphan{}, false
	}
	if strings.HasPrefix(job.Label, "com.apple.") {
		return Orphan{}, false
	}
	exec, ok := resolveExec(job)
	if !ok {
		return Orphan{}, false // unresolvable → never flagged (conservative)
	}
	if _, err := os.Stat(exec); err == nil {
		return Orphan{}, false // executable present → not orphaned
	} else if !os.IsNotExist(err) {
		return Orphan{}, false // can't tell (permission) → leave alone
	}
	domain, privileged := domainFor(path, uid)
	return Orphan{
		Plist: path, Label: job.Label, Domain: domain, Privileged: privileged, MissingExec: exec,
	}, true
}

// resolveExec returns the job's executable path and whether it was confidently
// resolved to an absolute path we can check for existence. env-launchers are
// unwrapped; shell wrappers and relative/bare programs are declined.
func resolveExec(job launchdJob) (string, bool) {
	prog := job.Program
	rest := job.ProgramArguments
	if prog == "" {
		if len(job.ProgramArguments) == 0 {
			return "", false
		}
		prog = job.ProgramArguments[0]
		rest = job.ProgramArguments[1:]
	}

	switch base := filepath.Base(prog); {
	case base == "env":
		// /usr/bin/env [VAR=val ...] <real-exec> ...
		for _, tok := range rest {
			if isAssignment(tok) {
				continue
			}
			return tok, filepath.IsAbs(tok)
		}
		return "", false
	case isShell(base):
		// Runs an inline or scripted command; the real binary is undecidable.
		return "", false
	default:
		return prog, filepath.IsAbs(prog)
	}
}

func isAssignment(tok string) bool {
	i := strings.IndexByte(tok, '=')
	return i > 0 && !strings.Contains(tok[:i], "/")
}

func isShell(base string) bool {
	switch base {
	case "sh", "bash", "zsh", "dash", "ksh", "tcsh", "csh", "fish":
		return true
	}
	return false
}

// isAppleManaged skips Apple-owned locations outright.
func isAppleManaged(path string) bool {
	return strings.HasPrefix(path, "/System/") || strings.Contains(path, "/Library/Apple/")
}

// domainFor selects the launchd domain and elevation for a plist by location.
func domainFor(path string, uid int) (domain string, privileged bool) {
	switch {
	case strings.HasPrefix(path, "/Library/LaunchDaemons"):
		return "system", true
	case strings.HasPrefix(path, "/Library/"):
		return fmt.Sprintf("gui/%d", uid), true
	default:
		return fmt.Sprintf("gui/%d", uid), false
	}
}

// fileDelete builds a FileDelete op for the plist if it is off the deny floor.
func fileDelete(path string, threshold int64) (operation.FileDeleteOp, bool) {
	if floor.Check(path) != nil {
		return operation.FileDeleteOp{}, false
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return operation.FileDeleteOp{}, false
	}
	id, err := identity.ComputeFile(path, threshold)
	if err != nil {
		return operation.FileDeleteOp{}, false
	}
	return operation.FileDeleteOp{Path: path, Size: fi.Size(), Category: "orphan-launch-agent", Expected: id}, true
}
