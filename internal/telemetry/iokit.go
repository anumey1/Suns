package telemetry

// IOKit thermal sensors (§4.4, §5.2, §12.9).
//
// On Apple Silicon, on-die temperatures and fan RPM are read via IOKit
// (IOHIDEventSystemClient); on Intel, via the legacy SMC. Both require Cgo, and
// the sensor keys differ across M1/M2/M3/M4 generations — resolved via the
// embedded board-ID → sensor-key manifest (assets/sensors.json), never fetched
// remotely.
//
// Like the native Trash call (pkg/trash), each IOKit read MUST be bounded: run
// under a per-call timeout in a bounded goroutine, supervised, so a wedged
// sensor marks the source stale/unavailable rather than pinning OS threads
// without limit (§4.4 IOKit containment). A fanless Mac correctly reports "N/A"
// fans, not 0 RPM.
//
// Phase 1 status: thermals are sourced from the supervised powermetrics stream
// (powermetrics.go) once elevated, which covers GPU/thermal/power without Cgo.
// The direct IOKit Cgo path is the alternative/Intel route and is staged here
// behind this seam — it must be implemented with the bounded-call discipline
// above and validated on-device against assets/sensors.json before being
// enabled. Until then, thermals come from powermetrics (elevated) or read N/A.
