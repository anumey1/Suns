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

// The floor is path-based and therefore mode-independent: there is no mode,
// flag, or config argument it could consult. This test documents that property
// by asserting a denied path stays denied regardless of how it is spelled.
func TestCheck_ModeIndependentByConstruction(t *testing.T) {
	variants := []string{"/System", "/System/", "/System/../System", "/./System"}
	for _, p := range variants {
		if Permits(p) {
			t.Errorf("Permits(%q) = true, want false under every spelling", p)
		}
	}
}
