// Package audit backs `suns audit` / `suns secure` (§12.13, §12.14). Read-only
// (the auth-log query needs root via the §6 chokepoint).
//
// It parses csrutil status (SIP), spctl --status (Gatekeeper), and fdesetup
// status (FileVault) into a single posture view, and surfaces failed
// authentication / privilege-escalation attempts via the unified log
// (`log show --predicate 'process == "sudo"' --last 1d`, plus Authorization /
// opendirectoryd predicates) — NOT the deprecated /var/log/system.log.
package audit
