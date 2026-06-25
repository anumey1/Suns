// Package purge holds the path-scoped filesystem-hygiene engines: the Empty
// Directory Purger (§12.19) and the Broken Symlink Auditor (§12.18).
//
// Both are read-only discoverers that emit typed FileDeleteOps (🟢) for a
// separate gated execution — they never delete during discovery. Unlike the
// safe-cache cleaner (internal/scanner), they operate on an operator-named root
// scope rather than a curated allowlist, so the CLI confirms the scope before
// acting. Every emitted target is deny-floor-checked and carries the tiered
// identity recorded at plan time; removal routes through the fd-anchored,
// no-follow deleter (§4.6) like any other FileDeleteOp.
package purge
