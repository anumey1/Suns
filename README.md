# Suns — Super User Nutcase Sessions

**Burn It All.**

A high-performance, safety-obsessed terminal-UI (TUI) system utility for macOS,
written in Go. Suns unifies a safe cleanup/operation engine, a precision
uninstaller, a deep system & security auditor, and the `get-coffee` live
telemetry dashboard behind one cohesive interface.

The authoritative design is **`Docs/SunsMasterTD.md`** (v6, single source of
truth). This README is a map of the scaffold; the master document is the spec.

## Core principle

Safe by default, dangerous on demand. A first run with zero flags and zero
configuration can never permanently destroy data (preview-and-confirm + move to
Trash). Destruction is always an explicit, deliberate escalation.

## Layout (see §8 of the master doc)

```
cmd/suns/            entry point: Cobra root, session bootstrap, CLI or TUI
internal/
  cli/               Cobra command definitions
  config/            Viper load → RWMutex-guarded SessionState (§4.9)
  tui/               Bubble Tea Model-Update-View; views, components, theme
  scanner/           concurrent WalkDir discovery → double-buffered snapshot
  uninstaller/       bundle-ID tracing; shared-dependency guard
  dedup/             APFS-conservative 3-pass duplicate finder
  optimizer/         DNS flush, mdutil, action wrappers
  telemetry/         supervised poller; powermetrics tokenizer; sysctl; IOKit
  audit/             SIP/Gatekeeper/FileVault posture; auth-log analysis
  net/               socket map, port audit, LAN scan, bandwidth
  scheduler/         launchd job authoring
pkg/
  operation/         Operation interface + concrete value-type op kinds (§4.2)
  plan/              Plan + Seal() value-copy (§4.5)
  safety/            gate, deny-floor, revalidate, fsdelete, firmlink (§4)
  trash/             bounded Cgo NSFileManager + pure-Go fallback (§4.4)
  history/           0600 JSONL operation history; export-redaction (§4.10)
  privilege/         centralized per-workflow sudo chokepoint (§6)
  syscmd/            hardened exec + parsing contracts (§6.3, §13.1)
  plist/             howett.net/plist + plutil + powermetrics tokenizer
assets/              //go:embed: logo, OUI db, safe-cache manifest, sensors.json
```

## Build sequence (§14)

- **Phase 0** — prove the dangerous core first (operation, plan, safety, trash,
  history, syscmd, privilege, doctor). Nothing destructive ships until the
  frozen value-plan, fd-anchored delete + firmlink handling, tiered identity
  validation, bounded-and-degrading trash, and the elevation chokepoint are all
  proven.
- **Phase 1 (v1)** — `clean` (safe-cache allowlist) + `restore` + read-only
  `get-coffee` dashboard.
- **Phase 2** — uninstaller + APFS-conservative dedup.
- **Phase 3** — net + audit suites.
- **Phase 4** — maintain, Docker prune, schedule, quarantined `lang-strip`,
  release pipeline.

## Status

Scaffold only — package skeletons with design-intent doc comments. No behavior
is implemented yet. Run `go mod tidy` once the Go toolchain is installed.
