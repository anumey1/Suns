package trash

// Volume-reachability probe (§4.4).
//
// Before a native trash batch, targets are grouped by volume and reachability
// is checked with statfs/access — but run inside a SHORT-LIVED HELPER
// SUBPROCESS that the parent can kill if it hangs, because a Go goroutine
// blocked in an in-kernel statfs cannot be cancelled while a child process can
// be killed. An unreachable volume causes its entire batch to be skipped and
// reported, never attempted natively.
//
// This is staged for the native path (native.go). It is intentionally a no-op
// while nativeTrash returns errNativeUnavailable: the pure-Go fallback does not
// pin OS threads and tolerates an unresponsive os.Rename via its error path, so
// it does not need the probe. When the Cgo native call is enabled, this must be
// implemented as a killable helper subprocess (e.g. a `suns __volprobe <path>`
// hidden subcommand the parent spawns and kills on timeout) and consulted
// before tryNative.
//
// TODO(phase0-native): implement the killable-helper probe and gate tryNative
// on it; cover with the §15 hung-volume integration test.
