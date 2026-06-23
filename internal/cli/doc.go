// Package cli holds the Cobra command definitions (§8, §9).
//
// The organizing principle is the separation of destructive from read-only
// commands: only destructive commands route through the safety gate and emit
// typed operations into a frozen plan; read-only commands never prompt and
// never delete.
//
//	Destructive (gated): clean, nuke, dedup, maintain, lang-strip, schedule,
//	                     restore, ps (kill action)
//	Read-only:           scan/analyze, monitor, get-coffee, net, audit/secure,
//	                     ps (listing)
//	Meta:                config (get|set|show), doctor, version
//
// Global flags: --dry-run, --yes, --deathstar, --jarjar=<trash|obliterate>,
// --json, --verbose/--quiet, --reduce-motion (§9.4).
package cli
