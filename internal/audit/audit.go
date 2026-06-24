package audit

import (
	"context"
	"os"

	"github.com/anumey1/Suns/pkg/plist"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// Runner is the unprivileged executor for the status tools. Production uses
// syscmd.New(); tests inject a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Severity classifies a finding for color and the overall posture (§12.13).
type Severity string

const (
	SevOK      Severity = "ok"
	SevWarn    Severity = "warn"
	SevRisk    Severity = "risk"
	SevUnknown Severity = "unknown"
)

// rank orders severities so the overall posture is the worst finding.
func (s Severity) rank() int {
	switch s {
	case SevRisk:
		return 3
	case SevWarn:
		return 2
	case SevUnknown:
		return 1
	default:
		return 0
	}
}

// Finding is one posture check.
type Finding struct {
	Name     string   `json:"name"`
	State    string   `json:"state"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail,omitempty"`
}

// Report is the full security posture.
type Report struct {
	SIP             Finding  `json:"sip"`
	Gatekeeper      Finding  `json:"gatekeeper"`
	FileVault       Finding  `json:"filevault"`
	XProtectVersion string   `json:"xprotect_version,omitempty"`
	Severity        Severity `json:"severity"` // worst of the findings
}

// Findings returns the three core findings in display order.
func (r Report) Findings() []Finding { return []Finding{r.SIP, r.Gatekeeper, r.FileVault} }

// xprotectPaths are the candidate XProtect bundle Info.plists, newest layout
// first. A package var so tests can point it at a fixture.
var xprotectPaths = []string{
	"/Library/Apple/System/Library/CoreServices/XProtect.bundle/Contents/Info.plist",
	"/System/Library/CoreServices/XProtect.bundle/Contents/Info.plist",
}

// Posture runs the status tools and assembles the read-only report. A tool that
// is missing or emits unexpected output yields an "unknown" finding rather than
// an error, so a partial environment still produces a useful posture.
func Posture(ctx context.Context, r Runner) (Report, error) {
	run := func(name string, args ...string) []byte {
		res, err := r.Run(ctx, name, args...)
		if err != nil {
			return nil
		}
		return res.Stdout
	}

	rep := Report{
		SIP:             parseSIP(run("csrutil", "status")),
		Gatekeeper:      parseGatekeeper(run("spctl", "--status")),
		FileVault:       parseFileVault(run("fdesetup", "status")),
		XProtectVersion: readXProtectVersion(),
	}

	worst := SevOK
	for _, f := range rep.Findings() {
		if f.Severity.rank() > worst.rank() {
			worst = f.Severity
		}
	}
	rep.Severity = worst
	return rep, nil
}

// readXProtectVersion reads CFBundleShortVersionString from the first XProtect
// bundle Info.plist that decodes (binary-safe, unprivileged).
func readXProtectVersion() string {
	var v struct {
		Version string `plist:"CFBundleShortVersionString"`
	}
	for _, p := range xprotectPaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if plist.Decode(p, &v) == nil && v.Version != "" {
			return v.Version
		}
	}
	return ""
}
