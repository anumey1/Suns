package uninstaller

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hplist "howett.net/plist"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// fakeRunner returns canned pkgutil output.
type fakeRunner struct {
	pkgs       []string
	volume     string
	files      map[string][]string // pkgID → relative payload paths
	fileOwners map[string][]string // absolute path → claiming pkgids
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (syscmd.Result, error) {
	out := func(s string) (syscmd.Result, error) { return syscmd.Result{Stdout: []byte(s)}, nil }
	if name != "pkgutil" || len(args) == 0 {
		return syscmd.Result{}, nil
	}
	switch args[0] {
	case "--pkgs":
		return out(strings.Join(f.pkgs, "\n"))
	case "--pkg-info":
		return out("package-id: " + args[1] + "\nvolume: " + f.volume + "\nlocation: ")
	case "--files":
		return out(strings.Join(f.files[args[1]], "\n"))
	case "--file-info":
		var b strings.Builder
		b.WriteString("path: " + args[1] + "\n")
		for _, o := range f.fileOwners[args[1]] {
			b.WriteString("pkgid: " + o + "\n")
		}
		return out(b.String())
	}
	return syscmd.Result{}, nil
}

func writeInfoPlist(t *testing.T, app, bundleID string) {
	t.Helper()
	dir := filepath.Join(app, "Contents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := hplist.Marshal(map[string]any{"CFBundleIdentifier": bundleID, "CFBundleName": "Foo"}, hplist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Info.plist"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPlan_SharedDependencyGuardAndOrdering(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	apps := filepath.Join(home, "Applications")
	volume := filepath.Join(root, "vol")
	const bundleID = "com.vendor.foo"
	const pkgID = "com.vendor.foo.pkg"

	// App bundle.
	app := filepath.Join(apps, "Foo.app")
	writeInfoPlist(t, app, bundleID)

	// ~/Library remnant traced by bundle-ID match.
	writeFile(t, filepath.Join(home, "Library", "Application Support", bundleID, "state.db"), "x")

	// A related user LaunchAgent (exercises ServiceUnload ordering).
	agent := filepath.Join(home, "Library", "LaunchAgents", "com.vendor.foo.helper.plist")
	agentData, _ := hplist.Marshal(map[string]any{"Label": "com.vendor.foo.helper"}, hplist.XMLFormat)
	writeFile(t, agent, string(agentData))

	// .pkg payload: one sole-owned file, one shared dependency.
	sole := filepath.Join(volume, "usr/local/bin/foo")
	shared := filepath.Join(volume, "usr/local/lib/libshared.dylib")
	writeFile(t, sole, "binary")
	writeFile(t, shared, "shared")

	runner := &fakeRunner{
		pkgs:   []string{pkgID, "com.unrelated.other.pkg"},
		volume: volume,
		files:  map[string][]string{pkgID: {"usr/local/bin/foo", "usr/local/lib/libshared.dylib"}},
		fileOwners: map[string][]string{
			sole:   {pkgID},
			shared: {pkgID, "com.other.app.pkg"}, // claimed by two → shared
		},
	}

	res, err := Plan(context.Background(), "Foo", Options{
		Runner: runner, Home: home, UID: 501, SearchDirs: []string{apps},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if res.BundleID != bundleID {
		t.Errorf("BundleID = %q, want %q", res.BundleID, bundleID)
	}

	// Shared dependency retained, not deleted.
	if len(res.Retained) != 1 || !strings.Contains(res.Retained[0], "libshared.dylib") {
		t.Errorf("Retained = %v, want the shared dylib", res.Retained)
	}
	paths := offeredPaths(res)
	if paths[shared] {
		t.Errorf("shared dependency must NOT be offered for deletion")
	}
	if !paths[sole] {
		t.Errorf("sole-owned payload %q must be offered", sole)
	}
	if !paths[app] {
		t.Errorf("app bundle must be offered")
	}

	// Exactly one receipt forget, for the related package.
	var forgets []string
	for _, op := range res.Ops {
		if r, ok := op.(operation.ReceiptForgetOp); ok {
			forgets = append(forgets, r.PackageID)
		}
	}
	if len(forgets) != 1 || forgets[0] != pkgID {
		t.Errorf("forgets = %v, want [%s]", forgets, pkgID)
	}

	assertOrdering(t, res.Ops)
}

// offeredPaths returns the set of FileDelete target paths.
func offeredPaths(res Result) map[string]bool {
	set := map[string]bool{}
	for _, op := range res.Ops {
		if fd, ok := op.(operation.FileDeleteOp); ok {
			set[fd.Path] = true
		}
	}
	return set
}

// assertOrdering checks ServiceUnload* precede FileDelete* precede ReceiptForget*.
func assertOrdering(t *testing.T, ops []operation.Operation) {
	t.Helper()
	rank := map[operation.OpKind]int{
		operation.KindServiceUnload: 0,
		operation.KindFileDelete:    1,
		operation.KindReceiptForget: 2,
	}
	last := -1
	sawService, sawFile, sawForget := false, false, false
	for _, op := range ops {
		r, ok := rank[op.Kind()]
		if !ok {
			t.Fatalf("unexpected op kind %s", op.Kind())
		}
		if r < last {
			t.Fatalf("op ordering violated: %s appeared after a later-ranked op", op.Kind())
		}
		last = r
		switch op.Kind() {
		case operation.KindServiceUnload:
			sawService = true
		case operation.KindFileDelete:
			sawFile = true
		case operation.KindReceiptForget:
			sawForget = true
		}
	}
	if !sawService || !sawFile || !sawForget {
		t.Errorf("expected all three op kinds present: service=%v file=%v forget=%v", sawService, sawFile, sawForget)
	}
}

func TestPlan_AppNotFound(t *testing.T) {
	if _, err := Plan(context.Background(), "DoesNotExist", Options{
		Home: t.TempDir(), SearchDirs: []string{t.TempDir()},
	}); err == nil {
		t.Fatal("want error when the app cannot be found")
	}
}
