package floor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck_DeniesProtectedPaths(t *testing.T) {
	denied := []string{
		"/",
		"/System",
		"/System/Library/CoreServices",
		"/usr",
		"/usr/bin/env",
		"/bin",
		"/bin/sh",
		"/sbin/launchd",
		"/Library/Apple/usr/libexec",
		"/private/var/db/anything",
	}
	for _, p := range denied {
		if err := Check(p); err == nil {
			t.Errorf("Check(%q) = nil, want ErrDenied", p)
		}
	}
}

func TestCheck_AllowsUsrLocalCarveOut(t *testing.T) {
	allowed := []string{
		"/usr/local/bin/suns",
		"/usr/local/lib/libfoo.dylib",
	}
	for _, p := range allowed {
		if err := Check(p); err != nil {
			t.Errorf("Check(%q) = %v, want nil (carve-out)", p, err)
		}
	}
}

func TestCheck_HomeRootProtectedButSubpathsAllowed(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home directory")
	}
	if err := Check(home); err == nil {
		t.Errorf("Check(home root %q) = nil, want ErrDenied", home)
	}
	sub := filepath.Join(home, "Library", "Caches", "com.example")
	if err := Check(sub); err != nil {
		t.Errorf("Check(home subpath %q) = %v, want nil", sub, err)
	}
	icloud := filepath.Join(home, "Library", "Mobile Documents", "x")
	if err := Check(icloud); err == nil {
		t.Errorf("Check(iCloud %q) = nil, want ErrDenied", icloud)
	}
}

// The floor consults only the path and its volume, never a mode, flag, or config
// argument, so it is mode-independent by construction. This test documents that
// property by asserting a denied path stays denied regardless of how it is
// spelled.
func TestCheck_ModeIndependentByConstruction(t *testing.T) {
	variants := []string{"/System", "/System/", "/System/../System", "/./System"}
	for _, p := range variants {
		if Permits(p) {
			t.Errorf("Permits(%q) = true, want false under every spelling", p)
		}
	}
}

// §5.3 "anything on a System Volume": a path on the read-only System Volume that
// the prefix list does not enumerate is still denied by volume residence. On
// modern macOS the System Volume is mounted read-only at "/", so a real
// system-content path under it (resolved through no firmlink) lands on a
// read-only mount.
func TestCheck_DeniesReadOnlySystemVolume(t *testing.T) {
	// "/" itself is denied by an explicit rule; verify the residence guard via a
	// System-Volume path. If the host has an unusual mount layout where this path
	// is not read-only, skip rather than fail.
	const p = "/System/Library/CoreServices/SystemVersion.plist"
	if _, err := os.Stat(p); err != nil {
		t.Skipf("system path %q not present: %v", p, err)
	}
	if !onReadOnlyVolume(p) {
		t.Skipf("%q is not on a read-only volume on this host", p)
	}
	if Check(p) == nil {
		t.Errorf("Check(%q) = nil, want ErrDenied (read-only system volume)", p)
	}
}

// The residence guard must not deny writable user paths: a temp file the test
// creates lives on a read-write volume and stays permitted.
func TestCheck_AllowsWritableVolume(t *testing.T) {
	f := filepath.Join(t.TempDir(), "scratch")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	if onReadOnlyVolume(f) {
		t.Skipf("temp dir %q is unexpectedly read-only on this host", f)
	}
	if err := Check(f); err != nil {
		t.Errorf("Check(%q) = %v, want nil (writable volume)", f, err)
	}
}
