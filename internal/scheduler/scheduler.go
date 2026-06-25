package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	hplist "howett.net/plist"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// Label is the LaunchAgent label for the scheduled burn daemon.
const Label = "com.suns.scheduled-clean"

// Runner executes launchctl. Production uses syscmd.New(); tests inject a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Config parameterizes the authored LaunchAgent.
type Config struct {
	BinaryPath string // absolute path to the suns binary (os.Executable)
	Hour       int    // local hour 0–23 for the daily run
	Minute     int    // local minute 0–59
}

// launchdPlist is the subset of a LaunchAgent plist Suns authors. RunAtLoad is
// deliberately false (no burn at install/login); the agent fires only on its
// calendar schedule, running the constrained `clean --scheduled`.
type launchdPlist struct {
	Label                 string         `plist:"Label"`
	ProgramArguments      []string       `plist:"ProgramArguments"`
	StartCalendarInterval map[string]int `plist:"StartCalendarInterval"`
	RunAtLoad             bool           `plist:"RunAtLoad"`
	ProcessType           string         `plist:"ProcessType"`
	StandardOutPath       string         `plist:"StandardOutPath"`
	StandardErrorPath     string         `plist:"StandardErrorPath"`
}

// GeneratePlist renders the LaunchAgent XML plist for cfg. It is pure and
// testable; Install writes its output and loads it.
func GeneratePlist(cfg Config) ([]byte, error) {
	if cfg.BinaryPath == "" {
		return nil, fmt.Errorf("scheduler: empty binary path")
	}
	logOut, logErr, err := logPaths()
	if err != nil {
		return nil, err
	}
	doc := launchdPlist{
		Label:            Label,
		ProgramArguments: []string{cfg.BinaryPath, "clean", "--scheduled"},
		StartCalendarInterval: map[string]int{
			"Hour":   clampInt(cfg.Hour, 0, 23),
			"Minute": clampInt(cfg.Minute, 0, 59),
		},
		RunAtLoad:         false,
		ProcessType:       "Background",
		StandardOutPath:   logOut,
		StandardErrorPath: logErr,
	}
	return hplist.MarshalIndent(doc, hplist.XMLFormat, "\t")
}

// Status reports the installed/loaded state of the agent.
type Status struct {
	Installed bool // the plist exists on disk
	Loaded    bool // launchctl reports the label in the user domain
	PlistPath string
}

// Install writes the plist into ~/Library/LaunchAgents and (re)loads it into the
// user GUI domain via launchctl. Re-installing is safe: any existing instance is
// booted out first. The launchctl calls are the on-device path.
func Install(ctx context.Context, r Runner, cfg Config) error {
	path, err := PlistPath()
	if err != nil {
		return err
	}
	data, err := GeneratePlist(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	domain := guiDomain()
	// Boot out any stale instance, ignoring "not loaded" errors, then bootstrap.
	_, _ = r.Run(ctx, "launchctl", "bootout", domain+"/"+Label)
	if res, err := r.Run(ctx, "launchctl", "bootstrap", domain, path); err != nil || res.ExitCode != 0 {
		return fmt.Errorf("scheduler: launchctl bootstrap failed (exit %d): %s", res.ExitCode, res.Stderr)
	}
	return nil
}

// Uninstall boots the agent out of the user domain and removes its plist. A
// not-loaded agent or a missing plist is treated as success (the desired end
// state already holds).
func Uninstall(ctx context.Context, r Runner) error {
	path, err := PlistPath()
	if err != nil {
		return err
	}
	_, _ = r.Run(ctx, "launchctl", "bootout", guiDomain()+"/"+Label)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CheckStatus reports whether the agent is installed and loaded.
func CheckStatus(ctx context.Context, r Runner) (Status, error) {
	path, err := PlistPath()
	if err != nil {
		return Status{}, err
	}
	st := Status{PlistPath: path}
	if _, err := os.Stat(path); err == nil {
		st.Installed = true
	}
	if res, err := r.Run(ctx, "launchctl", "print", guiDomain()+"/"+Label); err == nil && res.ExitCode == 0 {
		st.Loaded = true
	}
	return st, nil
}

// PlistPath returns ~/Library/LaunchAgents/<Label>.plist.
func PlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", Label+".plist"), nil
}

// logPaths returns the agent's stdout/stderr log paths under the Suns logs dir.
func logPaths() (string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(home, "Library", "Logs", "Suns")
	return filepath.Join(dir, "scheduled.out.log"), filepath.Join(dir, "scheduled.err.log"), nil
}

// guiDomain returns the launchctl GUI domain target for the current user.
func guiDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
