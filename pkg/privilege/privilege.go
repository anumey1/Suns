// Package privilege is the centralized, per-workflow sudo chokepoint (§6).
//
// All privileged execution funnels through ONE elevation chokepoint, never
// scattered across engines. It acquires a ticket per workflow with `sudo -v`
// via Bubble Tea's tea.Exec (which releases the terminal so the password prompt
// renders directly, then restores the TUI), handles macOS tty_tickets
// correctly for privileged background subprocesses, and keeps NO persistent
// root daemon — it prefers a clear re-prompt over an ambient-root surface.
//
// Defined failure states: canceled prompt (dependent ops skipped as
// skipped:no-privilege, workflow continues for non-privileged ops), ticket
// expiry mid-workflow (re-prompt once, then abort the privileged remainder
// cleanly), and partial privileged batch (each op independently validated and
// recorded).
//
// The boundary (§6.4): all scanning/discovery is unprivileged; the chokepoint
// executes only narrow allowlisted root ACTIONS (DNS flush, the auth-log
// log-show query, pkgutil --forget / --file-info, daemon installation, and the
// privileged validate-and-kill for ProcessKill). It never performs broad
// protected-path reads on behalf of the scanner.
package privilege

// TODO(phase0): the chokepoint with TTY-ticket inheritance and failure-state
// handling; no persistent runner.
