// Package privilege is the centralized, per-workflow sudo chokepoint (§6).
//
// All privileged execution funnels through ONE Chokepoint, never scattered
// across engines. It acquires a ticket per workflow, keeps NO persistent root
// process, and runs only narrow, allowlisted root ACTIONS (DNS flush, the
// auth-log query, pkgutil, daemon install, the validated process kill) — never
// broad protected-path reads (§6.4).
//
// The interactive prompt is abstracted behind Prompter so the mechanism can
// differ by front-end: the TUI uses tea.Exec to release the terminal for the
// password prompt (Phase 1), the CLI prompts inline (TerminalPrompter), and
// tests inject a fake. Ticket checking is likewise a seam, so the failure-state
// logic is testable without real sudo.
package privilege

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/anumey1/Suns/pkg/syscmd"
)

const sudoPath = "/usr/bin/sudo"

// Errors describing the defined failure states (§6.1).
var (
	// ErrCanceled means the user aborted the password prompt; dependent
	// operations are skipped (skipped:no-privilege) and the workflow continues
	// for non-privileged work.
	ErrCanceled = errors.New("privilege: authentication canceled")
	// ErrExpired means the sudo ticket expired mid-workflow and re-prompting
	// failed; the privileged remainder is aborted cleanly.
	ErrExpired = errors.New("privilege: ticket expired")
	// ErrActionNotAllowed means a caller requested a privileged action that is
	// not on the narrow allowlist — a programming error.
	ErrActionNotAllowed = errors.New("privilege: action not allowlisted")
)

// Prompter performs the interactive `sudo -v` authentication. Implementations
// must render the password prompt on the real terminal.
type Prompter interface {
	Authenticate(ctx context.Context) error
}

// privilegedActions are the narrow root actions the chokepoint may run (§6.4).
// Each name must also exist in the syscmd allowlist so its path can be pinned.
var privilegedActions = map[string]bool{
	"dscacheutil": true, // DNS flush
	"killall":     true, // HUP mDNSResponder (DNS flush)
	"log":         true, // auth-log query
	"pkgutil":     true, // --forget / --file-info
	"launchctl":   true, // daemon install / bootout
}

// Chokepoint is the single elevation point.
type Chokepoint struct {
	runner    *syscmd.Runner
	prompter  Prompter
	allow     map[string]bool
	hasTicket func(ctx context.Context) bool
}

// New returns a production Chokepoint using the given Prompter.
func New(prompter Prompter) *Chokepoint {
	r := syscmd.New()
	c := &Chokepoint{
		runner:   r,
		prompter: prompter,
		allow:    privilegedActions,
	}
	c.hasTicket = c.checkTicket
	return c
}

// Options configures a Chokepoint for tests.
type Options struct {
	Runner    *syscmd.Runner
	Prompter  Prompter
	Allow     map[string]bool
	HasTicket func(ctx context.Context) bool
}

// NewWithOptions builds a Chokepoint with injectable seams (tests).
func NewWithOptions(o Options) *Chokepoint {
	c := &Chokepoint{
		runner:    o.Runner,
		prompter:  o.Prompter,
		allow:     o.Allow,
		hasTicket: o.HasTicket,
	}
	if c.allow == nil {
		c.allow = privilegedActions
	}
	if c.hasTicket == nil {
		c.hasTicket = c.checkTicket
	}
	return c
}

// Acquire ensures a valid sudo ticket, prompting once if none is present. A
// canceled prompt returns ErrCanceled; callers then skip privileged work.
func (c *Chokepoint) Acquire(ctx context.Context) error {
	if c.hasTicket(ctx) {
		return nil
	}
	if c.prompter == nil {
		return ErrCanceled
	}
	return c.prompter.Authenticate(ctx)
}

// Run executes an allowlisted privileged action as `sudo -n <pinned-path>
// <args>`. It ensures a ticket first; if the ticket has expired by the time the
// action runs, it re-prompts once and retries, then aborts with ErrExpired
// (§6.1). A non-allowlisted action is ErrActionNotAllowed.
func (c *Chokepoint) Run(ctx context.Context, name string, args ...string) (syscmd.Result, error) {
	if !c.allow[name] {
		return syscmd.Result{}, fmt.Errorf("%w: %q", ErrActionNotAllowed, name)
	}
	innerPath, ok := c.runner.Path(name)
	if !ok {
		return syscmd.Result{}, fmt.Errorf("privilege: %q has no pinned path in syscmd", name)
	}
	if err := c.Acquire(ctx); err != nil {
		return syscmd.Result{}, err
	}

	sudoArgs := append([]string{"-n", innerPath}, args...)
	res, err := c.runner.Run(ctx, "sudo", sudoArgs...)
	if !ticketRejected(res, err) {
		return res, err
	}

	// Ticket expired between Acquire and Run: re-prompt once, then retry.
	if c.prompter == nil {
		return res, ErrExpired
	}
	if perr := c.prompter.Authenticate(ctx); perr != nil {
		return res, ErrExpired
	}
	res, err = c.runner.Run(ctx, "sudo", sudoArgs...)
	if ticketRejected(res, err) {
		return res, ErrExpired
	}
	return res, err
}

// checkTicket reports whether a sudo ticket is currently valid via the
// non-interactive `sudo -n -v` (exit 0 ⇒ valid). It uses the pinned sudo path
// and a scrubbed environment.
func (c *Chokepoint) checkTicket(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, sudoPath, "-n", "-v")
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	return cmd.Run() == nil
}

// ticketRejected detects sudo's "no valid ticket under -n" failure: exit code 1
// with the password-required message on stderr.
func ticketRejected(res syscmd.Result, err error) bool {
	if err == nil {
		return false
	}
	return res.ExitCode == 1
}

// TerminalPrompter authenticates by running interactive `sudo -v` with the
// process's real stdio so the password prompt renders on the terminal. It is
// the CLI front-end's prompter; the TUI supplies a tea.Exec-based one (Phase 1)
// behind this same interface. It bypasses syscmd's output capture precisely
// because the prompt needs the real TTY, but still uses the pinned sudo path.
type TerminalPrompter struct{}

// Authenticate runs `sudo -v`, returning ErrCanceled if the user aborts.
func (TerminalPrompter) Authenticate(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, sudoPath, "-v")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "TERM=" + os.Getenv("TERM")}
	if err := cmd.Run(); err != nil {
		return ErrCanceled
	}
	return nil
}
