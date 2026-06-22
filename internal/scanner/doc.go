// Package scanner performs concurrent, read-only WalkDir discovery into a
// double-buffered ScanSnapshot (§3.2, §3.5).
//
// A bounded worker pool (min(NumCPU, 8)) walks directory roots with
// filepath.WalkDir (used only for discovery, never deletion). Matches
// accumulate in an append-only store; the builder sorts the inactive buffer in
// place and publishes it via atomic.Pointer.Store (≈ zero per-tick allocation
// at steady state, snapshot size-capped). The frozen plan is built from the
// complete store, not the paged per-tick view. Every worker selects on
// ctx.Done() at each directory boundary and exits promptly on cancellation.
package scanner
