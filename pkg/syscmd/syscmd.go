// Package syscmd is the hardened external-command execution layer (§6.3) plus
// per-command parsing contracts (§13.1). Every external command — privileged or
// not — goes through it.
//
// It enforces: absolute, pinned executable paths (no PATH lookup → no $PATH
// hijack); a scrubbed environment with LC_ALL=C for locale-deterministic,
// parseable output; no shell (exec.Command with an argument slice → no shell
// injection); a compiled-in command allowlist (a non-allowlisted command is a
// programming error caught in tests); bounded stdout/stderr; and a context
// timeout on every call.
//
// For each scraped tool (nettop, lsof, log show, powermetrics, vm_stat, sysctl,
// pmset, iostat, csrutil, spctl, fdesetup, mdutil, Docker, Git) a parsing
// contract defines the expected format, a parser with version detection, and
// graceful degradation ("telemetry unavailable on this OS version" rather than
// garbage). Structured output (plist/JSON) is preferred over text scraping.
package syscmd

// TODO(phase0): the hardened Run with allowlist, pinned paths, LC_ALL=C,
// bounded output, timeout; parsing-contract registry.
