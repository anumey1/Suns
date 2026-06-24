// Package audit backs `suns audit` / `suns secure` (§12.13, §12.14). Read-only.
//
// Posture parses csrutil status (SIP), spctl --status (Gatekeeper), and fdesetup
// status (FileVault) into a single posture view, and reads the embedded XProtect
// version, surfacing a per-finding severity (ok / warn / risk / unknown) for
// color and an overall worst-of posture. Every tool is an unprivileged status
// read through the hardened executor; unexpected output degrades to an "unknown"
// finding rather than an error (§13.1 parsing contracts in parse.go).
//
// The sudo / auth-log analyzer (§12.14) — which queries the unified log
// (`log show --predicate 'process == "sudo"'`) and needs root via the §6
// chokepoint — is a later Phase 3 slice.
package audit
