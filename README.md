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

**Phase 0 (safety spine) and Phase 1 (trustworthy v1) are complete** — building
and unit-tested under `-race`. See `Docs/Phases.md` §0 for the authoritative
done/remaining breakdown.

- **Phase 0** — `operation` (with `FileDeleteOp`/`ProcessKillOp`), value-sealing
  `plan`, the `safety` spine (deny floor, firmlink, tiered identity, fd-anchored
  `fsdelete`, gate, revalidate), bounded-and-degrading `trash`, `0600` JSONL
  `history`, hardened `syscmd`, the `privilege` chokepoint, and `procctl`.
- **Phase 1** — `clean` (safe-cache allowlist → frozen plan → gate → trash),
  `restore`, the read-only `get-coffee` telemetry dashboard, gated process kill
  (`ps` + hidden privileged killer), and `doctor`. CLI commands: `doctor`,
  `clean`, `restore`, `get-coffee`, `ps`.
- **Phase 2 (complete)** —
  - `ashen` (the APFS-conservative duplicate finder; `internal/dedup`):
    three-pass hash detection → frozen plan → gate → trash, with
    hardlink/bundle/symlink/floor guards and the shared-block correction. The
    `dedup` spec name is kept as a hidden alias.
  - `nuke <app>` (the precision uninstaller; `internal/uninstaller`): bundle-ID
    tracing, `~/Library` + launch-agent tracing, `pkgutil` payload harvest with
    the `--file-info` shared-dependency guard, an ordered plan (ServiceUnload →
    FileDelete → ReceiptForget), and explicit UI scope bounds. New op kinds
    `ServiceUnloadOp` / `ReceiptForgetOp` execute via the privilege chokepoint;
    `pkg/plist` provides binary-safe Info.plist reading.
  - `orphans` (the orphaned launch-agent purge; `internal/orphans`): flags
    launchd jobs whose resolved executable is gone, conservatively (env-launchers
    unwrapped, shell/relative programs declined, Apple-managed skipped), and
    boots them out before trashing the plist.
  - `restore` hardened: confirmed FileDelete-only with a >50 MB large-object test.
- **Phase 3 (complete)** — read-only `net` + `audit` inspection suites, all with
  `--json` and graceful-degradation parsing contracts:
  - `audit` (alias `secure`; `internal/audit`): a read-only security-posture view
    (SIP/Gatekeeper/FileVault via `csrutil`/`spctl`/`fdesetup` + XProtect version),
    with per-finding severity and `--json`. Parsing contracts degrade to "unknown".
  - `net` (`internal/net`): per-app socket mapper + open-port auditor. Parses
    `lsof -F` field output, flags externally-reachable (wildcard) listening ports
    vs loopback, bounded cached reverse-DNS, `--json`/`--no-dns`.
  - `audit logs` (`internal/audit`): sudo/auth timeline from the unified log
    (root, via the chokepoint) with success/failure/denied classification and
    per-user rapid-failure burst detection; `--json`/`--since`.
  - `net lan` (`internal/net`): passive ARP-cache LAN discovery (IP/MAC/vendor via
    a curated OUI subset/hostname) with a warn-gated active port probe (`--probe`).
  - `net bw` (`internal/net`): per-interface throughput (the reliable core) plus
    EXPERIMENTAL per-process tx/rx by differencing a two-sample `nettop` capture,
    degrading to "unavailable" cleanly; `--json`/`--interval`/`--top`.
  - Multi-volume disk: `get-coffee` now enumerates every mounted physical volume
    and surfaces the fullest one in the DISK tile (was root-only).
  - Live NET widget (EXPERIMENTAL): a supervised, unprivileged `nettop` stream
    (`internal/telemetry`) diffs consecutive frames for per-process tx/rx, shown
    in the `get-coffee` dashboard with an honest health badge (warming/stalled).

The remaining engine stubs (`optimizer`, `scheduler`) are design-intent doc
comments pending Phases 3–4. Carry-forward on-device seams (native Cgo Trash,
IOKit thermals, live powermetrics) are flagged in `Docs/Phases.md` §1.
