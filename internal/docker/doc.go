// Package docker is the container-engine detection + prune engine (§12.15).
//
// Docker does not run natively on macOS — it runs inside a Linux VM provided by
// Docker Desktop, Colima, or OrbStack — so the SDK's default Unix socket often
// does not exist. This package therefore probes the known socket locations and
// locates the docker CLI binary across the places those runtimes install it,
// confirms the daemon is reachable, and previews reclaimable space via
// `docker system df` before any prune. The actual prune (`docker system
// prune -a [--volumes]`) is the on-device path: it runs the discovered binary
// through the hardened executor and is exercised in tests via an injected runner.
package docker
