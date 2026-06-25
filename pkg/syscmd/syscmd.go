// Package syscmd is the hardened external-command execution layer (§6.3).
//
// Every external command — privileged or not — goes through a Runner, which
// enforces: absolute pinned executable paths (no PATH lookup, so no $PATH
// hijack); a scrubbed environment with LC_ALL=C for locale-deterministic,
// parseable output; no shell (exec.Command with an argument slice, never
// sh -c, so no shell injection); a compiled-in command allowlist (a
// non-allowlisted command is a programming error); bounded stdout/stderr; and a
// context timeout on every call.
//
// Per-command parsing contracts (§13.1) layer on top of this in later phases.
package syscmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// ErrNotAllowed is returned when a command name is not in the Runner allowlist.
var ErrNotAllowed = errors.New("syscmd: command not in allowlist")

// DefaultMaxOutput bounds captured stdout/stderr so a runaway tool cannot
// exhaust memory.
const DefaultMaxOutput = 8 << 20 // 8 MiB

// productionAllowlist maps a command name to its absolute, pinned path. Only
// commands Suns actually invokes appear here (§6.3, §6.4). This list grows as
// engines land in later phases.
var productionAllowlist = map[string]string{
	"lsof":         "/usr/sbin/lsof",
	"arp":          "/usr/sbin/arp",
	"nettop":       "/usr/bin/nettop",
	"powermetrics": "/usr/bin/powermetrics",
	"pkgutil":      "/usr/sbin/pkgutil",
	"dscacheutil":  "/usr/bin/dscacheutil",
	"killall":      "/usr/bin/killall",
	"log":          "/usr/bin/log",
	"csrutil":      "/usr/bin/csrutil",
	"spctl":        "/usr/sbin/spctl",
	"fdesetup":     "/usr/bin/fdesetup",
	"mdutil":       "/usr/bin/mdutil",
	"pmset":        "/usr/bin/pmset",
	"vm_stat":      "/usr/bin/vm_stat",
	"sysctl":       "/usr/sbin/sysctl",
	"launchctl":    "/bin/launchctl",
	"sudo":         "/usr/bin/sudo",
	"git":          "/usr/bin/git",
}

// Runner executes allowlisted commands under the hardening rules.
type Runner struct {
	allow     map[string]string
	maxOutput int
}

// New returns a Runner with the production allowlist.
func New() *Runner {
	return &Runner{allow: productionAllowlist, maxOutput: DefaultMaxOutput}
}

// NewWithAllowlist returns a Runner with a custom name→path allowlist. It is
// used by tests; production code uses New.
func NewWithAllowlist(allow map[string]string) *Runner {
	return &Runner{allow: allow, maxOutput: DefaultMaxOutput}
}

// Path returns the absolute pinned path for an allowlisted command name. It is
// used by the privilege chokepoint to build `sudo <pinned-path> …` invocations
// without a PATH lookup.
func (r *Runner) Path(name string) (string, bool) {
	p, ok := r.allow[name]
	return p, ok
}

// Result is the outcome of a Run.
type Result struct {
	Stdout    []byte
	Stderr    []byte
	ExitCode  int
	Truncated bool // true if either stream hit maxOutput
}

// Run executes the allowlisted command identified by name with the given
// arguments. Arguments are passed verbatim as a slice and never interpolated
// into a shell, so shell metacharacters in args are inert. The provided context
// bounds the call; callers should always pass one with a deadline.
func (r *Runner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	path, ok := r.allow[name]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrNotAllowed, name)
	}

	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = []string{
		"LC_ALL=C",
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
	}

	var out, errb boundedBuffer
	out.limit = r.maxOutput
	errb.limit = r.maxOutput
	cmd.Stdout = &out
	cmd.Stderr = &errb

	runErr := cmd.Run()
	res := Result{
		Stdout:    out.Bytes(),
		Stderr:    errb.Bytes(),
		Truncated: out.truncated || errb.truncated,
	}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	return res, runErr
}

// boundedBuffer captures up to limit bytes and then discards the rest, flagging
// truncation.
type boundedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return len(p), nil // pretend success; discard
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	return b.buf.Write(p)
}

func (b *boundedBuffer) Bytes() []byte { return b.buf.Bytes() }
