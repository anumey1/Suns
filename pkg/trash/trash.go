// Package trash implements bounded-and-degrading native Trash (§4.4) — one of
// the most important mechanisms in the design.
//
// The correct way to move a file to the macOS Trash is the Cocoa call
// NSFileManager trashItemAtURL:, reached via Cgo. But every Cgo call pins an OS
// thread; a hung call (dead SMB mount, stalled FSEvents) loses that thread to
// the Go runtime, and enough of them crash the program. This package therefore
// layers four defenses:
//
//  1. A volume-reachability probe run inside a killable helper subprocess (a
//     goroutine blocked in an in-kernel statfs cannot be cancelled, but a child
//     process can be killed). Unreachable volumes are skipped and reported.
//  2. A concurrency semaphore (small N, e.g. 4) capping in-flight Cgo calls,
//     each with a per-call timeout.
//  3. A circuit breaker that DEGRADES, never refuses: after K consecutive
//     timeouts it falls back to a pure-Go ~/.Trash move (os.Rename same-volume,
//     the volume's /.Trashes/<uid> or copy-then-unlink across volumes — never a
//     silent permanent delete). A healthy local volume always succeeds even
//     while a dead mount has tripped the breaker.
//  4. The same bounding discipline for the other Cgo path (IOKit thermal reads).
//
// The fallback has no OS "Put Back" metadata, which is acceptable because
// restore uses Suns's own operation-history log, not OS Put-Back (§4.8).
package trash

// TODO(phase0): probe → semaphore → breaker → pure-Go fallback, with real
// hung-volume tests (§15).
