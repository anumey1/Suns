package uninstaller

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plist"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// Runner is the unprivileged discovery executor (pkgutil reads). Production uses
// syscmd.New(); tests inject a fake. Discovery never elevates (§2.1).
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Options configures a Plan run. Zero values select production defaults.
type Options struct {
	Runner     Runner   // discovery runner; default syscmd.New()
	Home       string   // home dir override (tests); default os.UserHomeDir()
	UID        int      // gui-domain uid (tests); default os.Getuid()
	SearchDirs []string // where to look for <name>.app; default /Applications, ~/Applications
	Threshold  int64    // tiered-identity threshold; 0 → identity.DefaultLargeThreshold
}

// Result is the read-only outcome of Plan: a sealed-ready, ORDERED op set plus
// the transparency record of what was deliberately retained and the explicit
// scope bounds the UI must show (§10.7).
type Result struct {
	App      string                // resolved .app path ("" if not found on disk)
	BundleID string                // CFBundleIdentifier
	Ops      []operation.Operation // ServiceUnload… → FileDelete… → ReceiptForget…
	Retained []string              // files excluded as shared dependencies (with claimants)
	Receipts []string              // package ids that will be forgotten
	Bounds   []string              // what nuke does NOT comprehensively chase
}

// launchdJob is the subset of a launchd plist used to identify a job.
type launchdJob struct {
	Label            string   `plist:"Label"`
	Program          string   `plist:"Program"`
	ProgramArguments []string `plist:"ProgramArguments"`
}

// libSubdirs under ~/Library that commonly hold per-app support files traced by
// bundle-ID match (best-effort — apps do not always name files by bundle ID).
var libSubdirs = []string{
	"Application Support", "Caches", "Preferences", "Logs",
	"Containers", "Group Containers", "Saved Application State",
	"HTTPStorages", "WebKit", "Cookies",
}

type engine struct {
	ctx       context.Context
	run       Runner
	home      string
	uid       int
	threshold int64

	service  []operation.Operation // bootouts (run first)
	files    []operation.Operation // deletes (app, library, payload, plists)
	forget   []operation.Operation // receipt forgets (run last)
	retained []string
	receipts []string
	seen     map[string]bool // path dedupe across discovery sources
}

// Plan traces an application and assembles the ordered uninstall plan. It is
// read-only: no deletion happens here. Discovery is unprivileged; only the
// returned ops elevate at execution (ReceiptForget and system-domain
// ServiceUnload via the chokepoint).
func Plan(ctx context.Context, target string, opts Options) (Result, error) {
	if opts.Runner == nil {
		opts.Runner = syscmd.New()
	}
	if opts.Home == "" {
		opts.Home, _ = os.UserHomeDir()
	}
	if opts.UID == 0 {
		opts.UID = os.Getuid()
	}
	if opts.Threshold == 0 {
		opts.Threshold = identity.DefaultLargeThreshold
	}
	if len(opts.SearchDirs) == 0 {
		opts.SearchDirs = []string{"/Applications", filepath.Join(opts.Home, "Applications")}
	}

	e := &engine{
		ctx: ctx, run: opts.Runner, home: opts.Home, uid: opts.UID,
		threshold: opts.Threshold, seen: map[string]bool{},
	}

	app, err := resolveApp(target, opts.SearchDirs)
	if err != nil {
		return Result{}, err
	}

	bundleID, err := plist.BundleIdentifier(filepath.Join(app, "Contents", "Info.plist"))
	if err != nil {
		return Result{}, fmt.Errorf("uninstaller: reading bundle id: %w", err)
	}

	// The app bundle itself.
	e.addFileDelete(app)
	// ~/Library support files traced by bundle-ID match (best-effort).
	e.traceLibrary(bundleID)
	// Related launch agents/daemons.
	e.traceLaunchJobs(bundleID, app)
	// .pkg receipts: harvest payload, guard shared deps, then forget.
	e.tracePackages(bundleID)

	ops := make([]operation.Operation, 0, len(e.service)+len(e.files)+len(e.forget))
	ops = append(ops, e.service...) // bootout before any file removal
	ops = append(ops, e.files...)
	ops = append(ops, e.forget...) // forget receipts last

	return Result{
		App:      app,
		BundleID: bundleID,
		Ops:      ops,
		Retained: e.retained,
		Receipts: e.receipts,
		Bounds: []string{
			"Mac App Store containers and app-group containers are not comprehensively chased.",
			"Shared frameworks used by other apps are retained, not removed.",
			"Login items, privileged helper tools, and LaunchServices registrations are not fully cleaned.",
			"Support files are traced by bundle-ID match, which is best-effort — review the plan.",
		},
	}, nil
}

// resolveApp finds the .app bundle from an explicit path or a name, searching one
// level of subfolders within each search dir.
func resolveApp(target string, searchDirs []string) (string, error) {
	if strings.HasSuffix(target, ".app") {
		if fi, err := os.Stat(target); err == nil && fi.IsDir() {
			return filepath.Abs(target)
		}
	}
	name := target
	if !strings.HasSuffix(name, ".app") {
		name += ".app"
	}
	for _, dir := range searchDirs {
		cand := filepath.Join(dir, name)
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand, nil
		}
		entries, _ := os.ReadDir(dir)
		for _, ent := range entries {
			if ent.IsDir() {
				sub := filepath.Join(dir, ent.Name(), name)
				if fi, err := os.Stat(sub); err == nil && fi.IsDir() {
					return sub, nil
				}
			}
		}
	}
	return "", fmt.Errorf("uninstaller: could not find application %q in %s", target, strings.Join(searchDirs, ", "))
}

// traceLibrary adds FileDelete ops for ~/Library entries whose name matches the
// bundle ID (best-effort).
func (e *engine) traceLibrary(bundleID string) {
	for _, sub := range libSubdirs {
		dir := filepath.Join(e.home, "Library", sub)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if strings.Contains(ent.Name(), bundleID) {
				e.addFileDelete(filepath.Join(dir, ent.Name()))
			}
		}
	}
}

// traceLaunchJobs adds ServiceUnload + plist FileDelete for launchd jobs related
// to the app (by label/program match against the bundle ID or app path).
func (e *engine) traceLaunchJobs(bundleID, app string) {
	dirs := []string{
		filepath.Join(e.home, "Library", "LaunchAgents"),
		"/Library/LaunchAgents",
		"/Library/LaunchDaemons",
	}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if !strings.HasSuffix(ent.Name(), ".plist") {
				continue
			}
			path := filepath.Join(dir, ent.Name())
			var job launchdJob
			if plist.Decode(path, &job) != nil || job.Label == "" {
				continue
			}
			if !jobRelated(job, bundleID, app, ent.Name()) {
				continue
			}
			e.addServiceUnload(path, job)
			e.addFileDelete(path) // bootout op is emitted first; this removes the file after
		}
	}
}

// jobRelated decides whether a launchd job belongs to the app being removed.
func jobRelated(job launchdJob, bundleID, app, fileName string) bool {
	hay := strings.Join(append([]string{job.Label, job.Program, fileName}, job.ProgramArguments...), "\n")
	return strings.Contains(hay, bundleID) || (app != "" && strings.Contains(hay, app))
}

// tracePackages harvests .pkg payloads, applies the shared-dependency guard, and
// queues a ReceiptForget per related receipt.
func (e *engine) tracePackages(bundleID string) {
	res, err := e.run.Run(e.ctx, "pkgutil", "--pkgs")
	if err != nil {
		return
	}
	for _, pkgID := range parseLines(res.Stdout) {
		if !relatedPkg(pkgID, bundleID) {
			continue
		}
		infoRes, _ := e.run.Run(e.ctx, "pkgutil", "--pkg-info", pkgID)
		info := parsePkgInfo(infoRes.Stdout)
		filesRes, _ := e.run.Run(e.ctx, "pkgutil", "--files", pkgID)
		for _, rel := range parseLines(filesRes.Stdout) {
			abs := filepath.Join(info.Volume, rel)
			fiRes, _ := e.run.Run(e.ctx, "pkgutil", "--file-info", abs)
			claimants := parseFileInfoPkgIDs(fiRes.Stdout)
			if len(claimants) > 1 {
				// Shared dependency: never delete; record for transparency.
				e.retained = append(e.retained, fmt.Sprintf("%s (claimed by %s)", abs, strings.Join(claimants, ", ")))
				continue
			}
			if isLaunchdPlist(abs) {
				var job launchdJob
				if plist.Decode(abs, &job) == nil && job.Label != "" {
					e.addServiceUnload(abs, job)
				}
			}
			e.addFileDelete(abs)
		}
		e.receipts = append(e.receipts, pkgID)
		e.forget = append(e.forget, operation.ReceiptForgetOp{PackageID: pkgID})
	}
}

// addFileDelete appends a FileDelete op for path if it exists, is off the deny
// floor, and has not already been queued.
func (e *engine) addFileDelete(path string) {
	if e.seen[path] {
		return
	}
	if floor.Check(path) != nil {
		return
	}
	fi, err := os.Lstat(path)
	if err != nil {
		return
	}
	var size int64
	if fi.IsDir() {
		size = dirSize(e.ctx, path)
	} else {
		size = fi.Size()
	}
	id, err := identity.ComputeFile(path, e.threshold)
	if err != nil {
		return
	}
	e.seen[path] = true
	e.files = append(e.files, operation.FileDeleteOp{Path: path, Size: size, Category: "uninstall", Expected: id})
}

// addServiceUnload appends a bootout op for a launchd job, choosing the domain
// and elevation by the plist location.
func (e *engine) addServiceUnload(path string, job launchdJob) {
	key := "svc:" + path
	if e.seen[key] {
		return
	}
	domain := fmt.Sprintf("gui/%d", e.uid)
	privileged := false
	if strings.HasPrefix(path, "/Library/") {
		privileged = true
		if strings.HasPrefix(path, "/Library/LaunchDaemons") {
			domain = "system"
		}
	}
	e.seen[key] = true
	e.service = append(e.service, operation.ServiceUnloadOp{
		Domain: domain, Label: job.Label, Plist: path, Privileged: privileged,
	})
}

func isLaunchdPlist(path string) bool {
	return strings.HasSuffix(path, ".plist") &&
		(strings.Contains(path, "/LaunchAgents/") || strings.Contains(path, "/LaunchDaemons/"))
}

// dirSize sums regular-file sizes under dir (read-only, error-tolerant).
func dirSize(ctx context.Context, dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil
		}
		if d.Type().IsRegular() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}
