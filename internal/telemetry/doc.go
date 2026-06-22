// Package telemetry powers `suns monitor` and the get-coffee dashboard (§7, §11).
//
// A single telemetry.Poller builds one *SystemSnapshot per cadence and
// publishes it via atomic.Pointer; every widget reads the same snapshot (no
// per-widget polling). Heavy sources (powermetrics, nettop) are launched ONCE
// as long-lived streams — never per-tick spawned, which would inflate the very
// readings being measured — and decoded continuously; cheap gopsutil sources
// poll in-process at ~1 Hz. Each source is a SUPERVISED service with health
// states (live/stale/stalled/restarting/unavailable), a watchdog with
// exponential backoff, parser resync, and staleness badges so dead data is
// never shown as live. The powermetrics tokenizer (pkg/plist) is read-deadline
// guarded; IOKit thermal reads are bounded Cgo (§4.4). Per-process network
// attribution via nettop is experimental; sysctl interface totals are the
// reliable core (§7.5).
package telemetry
