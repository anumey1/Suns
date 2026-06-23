package trash

import (
	"errors"
	"time"
)

// errNativeUnavailable means the native (Cgo NSFileManager) trash path is not
// enabled in this build; callers must degrade to the pure-Go fallback.
var errNativeUnavailable = errors.New("trash: native path unavailable")

// errNativeTimeout means a native call exceeded its per-call timeout and may
// have pinned an OS thread; the breaker counts these.
var errNativeTimeout = errors.New("trash: native call timed out")

// nativeTrash is the seam for the bounded Cgo NSFileManager trashItemAtURL:
// call (§4.4, §2.4). It MUST be implemented so that:
//
//   - it runs with a per-call timeout (the caller already holds the semaphore),
//   - a timeout returns errNativeTimeout so the breaker can count it,
//   - it never blocks the goroutine uncancellably beyond the bounded window.
//
// The real implementation requires Cgo and an Objective-C bridge to Foundation,
// plus the killable-helper volume-reachability probe (probe.go), and must be
// validated against a hung volume per §15. Until that lands and is proven,
// this returns errNativeUnavailable so every trash uses the proven pure-Go
// fallback — degrading, never bricking.
func nativeTrash(path string, timeout time.Duration) error {
	_ = path
	_ = timeout
	return errNativeUnavailable
}
