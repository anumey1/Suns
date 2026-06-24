package orphans

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	hplist "howett.net/plist"

	"github.com/anumey1/Suns/pkg/operation"
)

// writeJob writes a launchd plist (XML) into dir and returns its path.
func writeJob(t *testing.T, dir, name string, m map[string]any) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := hplist.Marshal(m, hplist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func find(t *testing.T, dir string) Report {
	t.Helper()
	rep, err := Find(context.Background(), Options{Home: t.TempDir(), UID: 501, Dirs: []string{dir}})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	return rep
}

func labels(rep Report) map[string]bool {
	set := map[string]bool{}
	for _, o := range rep.Orphans {
		set[o.Label] = true
	}
	return set
}

func TestFind_FlagsMissingExecutable(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone", "bin", "daemon")
	writeJob(t, dir, "com.acme.daemon.plist", map[string]any{
		"Label": "com.acme.daemon", "Program": missing,
	})
	rep := find(t, dir)
	if !labels(rep)["com.acme.daemon"] {
		t.Fatalf("missing-exec job should be flagged: %+v", rep.Orphans)
	}
	assertOrdering(t, rep.Ops)
}

func TestFind_PresentExecutableNotFlagged(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "com.acme.live.plist", map[string]any{
		"Label": "com.acme.live", "Program": "/usr/bin/true", // exists
	})
	if len(find(t, dir).Orphans) != 0 {
		t.Fatal("job with a present executable must not be flagged")
	}
}

func TestFind_AppleManagedSkipped(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "com.apple.something.plist", map[string]any{
		"Label": "com.apple.something", "Program": "/definitely/not/here",
	})
	if len(find(t, dir).Orphans) != 0 {
		t.Fatal("com.apple.* jobs must be skipped")
	}
}

func TestFind_EnvLauncherResolved(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(t.TempDir(), "missing", "real-bin")
	writeJob(t, dir, "com.acme.env.plist", map[string]any{
		"Label":            "com.acme.env",
		"ProgramArguments": []any{"/usr/bin/env", "FOO=1", "BAR=2", missing, "--flag"},
	})
	o := find(t, dir).Orphans
	if len(o) != 1 || o[0].MissingExec != missing {
		t.Fatalf("env-launcher should resolve to %q, got %+v", missing, o)
	}
}

func TestFind_ShellWrapperUnresolvable(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "com.acme.shell.plist", map[string]any{
		"Label":            "com.acme.shell",
		"ProgramArguments": []any{"/bin/sh", "-c", "/who/knows/what arg"},
	})
	if len(find(t, dir).Orphans) != 0 {
		t.Fatal("shell-wrapped jobs are unresolvable and must be left alone")
	}
}

func TestFind_RelativeProgramSkipped(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "com.acme.rel.plist", map[string]any{
		"Label": "com.acme.rel", "Program": "relative/path/bin",
	})
	if len(find(t, dir).Orphans) != 0 {
		t.Fatal("relative program paths are unresolvable and must be left alone")
	}
}

func TestFind_NoLabelSkipped(t *testing.T) {
	dir := t.TempDir()
	writeJob(t, dir, "nolabel.plist", map[string]any{
		"Program": "/definitely/not/here",
	})
	if len(find(t, dir).Orphans) != 0 {
		t.Fatal("a plist with no Label must be skipped")
	}
}

// assertOrdering checks ServiceUnload ops precede FileDelete ops.
func assertOrdering(t *testing.T, ops []operation.Operation) {
	t.Helper()
	sawFile := false
	for _, op := range ops {
		switch op.Kind() {
		case operation.KindFileDelete:
			sawFile = true
		case operation.KindServiceUnload:
			if sawFile {
				t.Fatal("ServiceUnload appeared after a FileDelete (bootout must precede plist removal)")
			}
		}
	}
}
