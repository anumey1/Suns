package audit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	hplist "howett.net/plist"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// fakeRunner returns canned status output keyed by command name.
type fakeRunner struct{ out map[string]string }

func (f fakeRunner) Run(_ context.Context, name string, _ ...string) (syscmd.Result, error) {
	if s, ok := f.out[name]; ok {
		return syscmd.Result{Stdout: []byte(s)}, nil
	}
	return syscmd.Result{}, nil // unknown tool → empty → "unknown" finding
}

func TestPosture_WorstSeverityAndFields(t *testing.T) {
	r := fakeRunner{out: map[string]string{
		"csrutil":  "System Integrity Protection status: enabled.",
		"spctl":    "assessments disabled", // risk
		"fdesetup": "FileVault is On.",
	}}
	rep, err := Posture(context.Background(), r)
	if err != nil {
		t.Fatalf("Posture: %v", err)
	}
	if rep.SIP.Severity != SevOK || rep.FileVault.Severity != SevOK {
		t.Errorf("SIP/FileVault = %v/%v", rep.SIP.Severity, rep.FileVault.Severity)
	}
	if rep.Gatekeeper.State != "disabled" {
		t.Errorf("Gatekeeper state = %q", rep.Gatekeeper.State)
	}
	if rep.Severity != SevRisk {
		t.Errorf("overall severity = %q, want risk (Gatekeeper disabled)", rep.Severity)
	}
}

func TestPosture_MissingToolsDegrade(t *testing.T) {
	rep, err := Posture(context.Background(), fakeRunner{out: map[string]string{}})
	if err != nil {
		t.Fatalf("Posture: %v", err)
	}
	for _, f := range rep.Findings() {
		if f.Severity != SevUnknown {
			t.Errorf("%s should be unknown when its tool is absent, got %s", f.Name, f.Severity)
		}
	}
}

func TestReadXProtectVersion(t *testing.T) {
	dir := t.TempDir()
	info := filepath.Join(dir, "Info.plist")
	data, _ := hplist.Marshal(map[string]any{"CFBundleShortVersionString": "5180"}, hplist.XMLFormat)
	if err := os.WriteFile(info, data, 0o644); err != nil {
		t.Fatal(err)
	}
	saved := xprotectPaths
	xprotectPaths = []string{info}
	defer func() { xprotectPaths = saved }()

	if got := readXProtectVersion(); got != "5180" {
		t.Errorf("readXProtectVersion = %q, want 5180", got)
	}
}
