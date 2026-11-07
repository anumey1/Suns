# Suns: status

> Safety-obsessed macOS maintenance CLI and TUI in Go: cleans, uninstalls, de-duplicates, audits. Read §1 before
> changing anything. History: `Docs/Changelog.md`.

| | |
|---|---|
| **Updated** | 2026-11-07: stats.md overhauled to the MakeStats template |
| **Verified** | 2026-11-07, full, against code and a build-and-test run (method: §13) |
| **Identity** | Go module `github.com/anumey1/Suns`, binary `suns`, version `0.0.0-dev` (`internal/cli/root.go` `Version`) |
| **Stack** | Go 1.25 (toolchain 1.26.4), macOS only; Cobra, Viper, Bubble Tea, gopsutil v4, `howett.net/plist`, `x/sys` |
| **Branch · verify** | `main` · `go build ./... && go vet ./... && go test -race ./...` |
| **Live** | Not released: no release, binary or tap exists; the release pipeline is parked (R-1, 2026-11-07) [owner] |
| **Active plan** | None. Phases 0–4 done in scope; `Docs/Phases.md` Phases §4.7 and Phases §4.8 are parked (§2, R-1) |
| **Formats** | `history.jsonl` field names (no version field; restore reads them), `safe_cache.json` `version` 1 (§6) |
| **Read first** | `CLAUDE.md`, `.claude/rules/safety.md` (loaded for `pkg/` safety packages) |

## 0. Current state

| What | State |
|---|---|
| Build and tests | Green: build, vet and `go test -race -count=1 ./...` pass, 172 test functions in 24 packages (as of 2026-11-07; recount: §13) [build] |
| Live | Not released. Release pipeline and `lang-strip` parked by owner ruling R-1 (§10.1) |
| Built | Phases 0–4 in scope: every command in §3 exists and is unit-tested through its engine |
| Bugs that matter now | L-2: a scheduled clean without Full Disk Access records `ok` while cleaning nothing. L-5: `doctor` does not check two pinned tools (§11.1) |
| Waiting on the owner | On-device checks O-1 to O-7 (§2); decide O-12 (`-y` flag) |
| Latest Changelog entry | 2026-11-07: stats.md overhauled to the MakeStats template |

## 1. Rules and hazards

### 1.1 Working rules

These are this project's rules beyond the global ones; `CLAUDE.md` states them in one line each.

1. **Prove behaviour with unit tests, never by running `suns`.** Allowed runs: `--dry-run`, `--help`, `help`,
   `--version`. Why: every other command touches the owner's real Mac (see §1.2). Ask the owner to run anything else
   with `!`.
2. **A new destructive action is an `Operation` value type** in `pkg/operation`, routed through `plan.New().Seal()`
   → `safety.BuildGroups`/`Render` → `safety.Execute`, re-verifying its target in `ValidateAtExec` and recording a
   typed `HistoryEntry`. Why: the gate, the time-of-use re-check and the undo record exist only on that path.
3. **A new external tool is pinned twice**: in `pkg/syscmd` `productionAllowlist` and in `internal/doctor`
   `pinnedTools`, with a parsing contract tested against a captured fixture through an injected runner. Why: absolute
   paths stop `$PATH` hijacking, and tool output changes between macOS versions (see the nettop trap, §11.2).
4. **Tests that delete use only `t.TempDir()`**, and no test runs a real destructive or privileged command. Why:
   `go test` runs as the owner on the owner's Mac.
5. **Package docs live in `doc.go`.** Why: a second package comment elsewhere fails `go vet`.
6. **Changes to `pkg/safety`, `pkg/trash`, `pkg/privilege`, `pkg/operation`, `pkg/plan`, `pkg/syscmd`,
   `pkg/history`** follow `.claude/rules/safety.md` and need the strongest tests. Why: these packages are what makes
   any deletion safe.

### 1.2 Hazards

| Hazard | What breaks | Guard |
|---|---|---|
| Running `suns` from a session | Trashes or permanently deletes files, kills processes, installs a launchd agent, prompts for sudo, probes the LAN, on the owner's Mac | `~/.claude/hooks/suns-guard.sh` (PreToolUse, `.claude/settings.json`) blocks all but `--dry-run`/`--help`/`help`/`version` |
| A config file with `deletion_mode: obliterate` and `confirm_mode: true` | Plain `suns clean` then deletes permanently with no gate. The "first run can't destroy data" promise holds only with no config file [inferred] | None in code; the file is `~/Library/Application Support/Suns/config.yaml` |
| `-y`/`--yes` and `--deathstar` | Skip the confirmation gate in one keystroke (O-12) | Trash stays the default unless `--jarjar obliterate` |
| Weakening the deny floor, or deleting by path string | Could delete system or iCloud data | §1.3 tests; `.claude/rules/safety.md` |
| Renaming `HistoryEntry` JSON fields | Restore can no longer read existing `history.jsonl` entries | None (§6) |
| `suns schedule install` | Creates a daily unattended agent | Scheduled runs are trash-only and allowlist-only (§1.3) |
| Editing code | `gofmt` is checked after every edit | `~/.claude/hooks/gofmt-check.sh` (PostToolUse) |

### 1.3 Invariants and guards

| Invariant | Guard |
|---|---|
| With no flags and no config, nothing is deleted permanently: defaults are `deletion_mode: trash`, `confirm_mode` off (`internal/config/session.go`) | Guard: none (`internal/config` has no tests; O-15) |
| A sealed plan is a pointer-free value copy; nothing can change it after the gate shows it | `pkg/plan`: `TestSeal_PanicsOnPointerReceiverOp`, `TestSeal_FreshSliceImmuneToSourceMutation`, `TestSeal_ValueCopiesPointerBackedOps` |
| Every op is re-validated at execution; a failed re-check is skipped, never run, and never stops the batch | `TestExecute_FullFlowWithSkip` (`pkg/safety`); per op: `TestFileDeleteOp_RefusesIdentityMismatch`, `TestProcessKillOp_RefusesStaleIdentity`, `TestRepoMaintenance_DirtySkippedAtExec` |
| The deny floor refuses `/`, the home root, iCloud Drive, `/System`, `/usr` (not `/usr/local`), `/bin`, `/sbin`, `/Library/Apple`, `/private/var/db`, `/cores`, and any read-only volume; it only ever grows | `pkg/safety/floor` `TestCheck_*`; re-checked at exec: `TestFileDeleteOp_FloorRecheckedAtExec` |
| Permanent delete never follows a symlink out of its tree | `TestObliterate_NeverEscapesViaSymlink`, `TestObliterate_RefusesIdentityMismatch` (`pkg/safety/fsdelete`) |
| The trash/obliterate axis affects only file deletes | `TestBuildGroups_DeletionAxisOnlyAffectsFileDelete`, `TestDNSFlush_ModeIsInert` |
| External commands: allowlisted absolute path, no shell, `LC_ALL=C`, output capped at 8 MiB | `pkg/syscmd` `TestRun_RejectsNonAllowlisted`, `TestRun_NoShellInjection`, `TestRun_SetsDeterministicLocale`, `TestRun_BoundsOutput` |
| Privileged ops without a sudo ticket are skipped, not failed or escalated | `TestDNSFlush_NoPrivilegeSkips`, `TestServiceUnload_NoPrivilegeSkips`, `TestReceiptForget_NoPrivilegeSkips`, `TestProcessKillOp_PrivilegedSkippedByDefault` |
| The chokepoint runs only allowlisted actions | `TestRun_RejectsNonAllowlistedAction` (`pkg/privilege`) |
| History is mode 0600; redacted export never changes the canonical file | `TestFileModeIs0600`, `TestExportRedacted_TransformsCopyNotCanonical` (`pkg/history`) |
| `nuke` keeps any file claimed by more than one package | `TestPlan_SharedDependencyGuardAndOrdering` (`internal/uninstaller`) |
| Restore offers only reversible, trashed file deletes | `TestCandidates_FiltersToReversibleTrashed`, `TestCandidates_ExcludesPhase2OpKinds` |
| Scheduled cleans are trash-only and allowlist-only (`runScheduledClean`, `internal/cli/clean.go`) | Guard: none (`internal/cli` has no tests; O-15) |
| `pkg/*` never imports `internal/*` | Guard: none (true on 2026-11-07: `go list -f '{{.Imports}}' ./pkg/...`) |
| `productionAllowlist` and `doctor` `pinnedTools` list the same tools | Guard: none, and false today (L-5) |

## 2. Open work

IDs are stable and never reused. Closed IDs move to the Changelog entry that closed them.

| ID | Item | State | Who | Next step |
|---|---|---|---|---|
| O-1 | Native Cgo trash (`NSFileManager trashItemAtURL:`) in `pkg/trash/native.go`; the pure-Go `~/.Trash` path is the only one today | Open | Agent, then owner on device | Implement behind the existing seam; owner checks on device |
| O-2 | Killable volume probe for hung volumes (`pkg/trash/probe.go`) | Open | Agent + owner | Helper subprocess and an integration test |
| O-3 | IOKit thermals in Cgo (`internal/telemetry/iokit.go`, `assets/sensors.json`) | Open | Agent + owner | Bounded IOKit reads; owner checks on device |
| O-4 | Live `sudo -n powermetrics` stream checked end to end (`internal/tui/powerlaunch.go`) | Open | Owner | Run `get-coffee`, press `e`, confirm GPU/thermal/power tiles |
| O-5 | Battery wattage, cycles and health via `ioreg` (`internal/telemetry/battery.go`) | Open | Agent | Pin `ioreg` (rule 3), fixture test |
| O-6 | Real `docker system prune` on a machine with Docker (`internal/docker`) | Open | Owner | Owner runs `suns docker-prune` with Docker present |
| O-7 | launchd `bootstrap`/`bootout` checked on device (`internal/scheduler`) | Open | Owner | Owner runs `schedule install`, `status`, `uninstall` |
| O-8 | `suns config show/get/set` (today the YAML is edited by hand) | Open | Agent | Spec in master TD; show the two safety axes |
| O-9 | History rotation: archive old entries to gzip files by age or size (L-3) | Open | Agent | Rotation that keeps restore working |
| O-10 | Onboarding and Full Disk Access panel in the TUI (L-1) | Open | Agent | Detect via `doctor`'s probe, explain, open System Settings |
| O-11 | Scheduled clean must report skipped protected targets instead of `ok` (L-2) | Open | Agent | Count discovery skips; distinct status such as `skipped_fda` |
| O-12 | Make bypassing the gate harder: drop `-y` for a long flag | Open | Owner decides | Owner rules yes or no |
| O-13 | TUI: Restore only lists (no select-and-restore); Audit menu is a placeholder; menu still says "(later phase)" for Get Coffee and Audit (`internal/tui/app.go`) | Open | Agent | Wire restore and audit screens; fix labels |
| O-14 | Add `killall` and `mdutil` to `doctor` `pinnedTools`, plus a test that it matches `productionAllowlist` (L-5) | Open | Agent | Small code change |
| O-15 | Direct tests for `internal/cli`, `internal/config`, `internal/doctor`, `internal/procadmin`, `pkg/procctl`, `pkg/safety/identity`, `pkg/safety/firmlink` | Open | Agent | Start with config defaults and scheduled mode (§1.3 "Guard: none") |
| O-16 | Cosmetic TUI backlog: braille charts (ntcharts) with block fallback, harmonica easing, bubblezone mouse, truecolor detection | Open | Agent | Low priority |
| O-17 | `README.md` status is stale (Phase 4 "in progress", `scheduler` called a stub) | Open | Agent | Rewrite the status part, point to this doc |
| O-18 | `nettopLauncher` comment says the stream is CSV; it uses `-l 0`, which is fixed-width (§11.3) | Open | Agent | Fix the comment, or switch to `-L 0` (CSV) and keep both parsers |

**Parked: don't propose these.**

| Item | Why parked | Reopen when |
|---|---|---|
| Release pipeline: universal binary, signing, notarization, `.pkg`, Homebrew (`Docs/Phases.md` §4.8) | Owner ruling R-1 | Owner says so |
| `suns lang-strip <app>` localization stripper (`Docs/Phases.md` §4.7) | Most dangerous feature: deleting `.lproj` breaks code signatures; designed to ship last, opt-in per app, with `codesign --verify` after [doc] | After the release pipeline, by owner decision |

## 3. What it is

Suns ("Super User Nutcase Sessions — Burn It All") is a macOS maintenance utility for developers and advanced users.
It cleans developer caches, uninstalls apps with their leftovers, removes duplicate files, empty folders and broken
symlinks, garbage-collects git repos, prunes Docker, flushes DNS, audits security and the network, shows a live
system dashboard, and schedules unattended cleanups. Its promise: **a first run with no flags (and no config file)
can never permanently destroy data**. Everything destructive is previewed, confirmed, moved to the Trash by default,
and undoable with `restore`.

**Scope:** macOS only (darwin; Apple Silicon and Intel). **Non-goals:** a GUI app; cleaning anything outside a
curated allowlist or an explicitly named path; claiming a "complete" uninstall.

**Commands** (`internal/cli`, one file per command). `suns` with no subcommand opens the TUI.

| Command | Changes the Mac? | Reversibility | What it does |
|---|---|---|---|
| `doctor` | No | — | Environment and capability self-check, including Full Disk Access |
| `clean` | Yes | Reversible (irreversible with obliterate) | Trash dev caches from the safe-cache allowlist (§6); `--include-optin` |
| `clean empty-dirs <path>` | Yes | Reversible | Empty folders (`.DS_Store`-only counts as empty); always confirms the scope |
| `clean broken-symlinks <path...>` | Yes | Reversible | Dangling symlinks; always confirms the scope |
| `clean --scheduled` | Yes | Reversible | Unattended mode used by the launchd agent (§7.10) |
| `scan <path...>` | No | — | Report broken symlinks and empty folders |
| `ashen [path...]` (alias `dedup`) | Yes | Reversible (irreversible with obliterate) | Duplicate finder |
| `nuke <app>` | Yes | Recoverable and irreversible parts | Precision uninstall: bundle, support files, agents, receipts |
| `orphans` | Yes | Reversible | Launch agents whose program is gone |
| `restore` | Undo | — | List and restore trashed file deletes |
| `maintain [path...]` | Yes | Recoverable | `git gc` on clean repos; `--aggressive`, `--prune-now` opt-in |
| `docker-prune` | Yes | Irreversible | `docker system prune -a -f`; `--volumes` opt-in |
| `dns-flush` | Yes, root | Irreversible | Flush the DNS cache and HUP mDNSResponder |
| `schedule install/uninstall/status` | Authoring | — | Manage the daily launchd cleanup agent |
| `audit` (alias `secure`), `audit logs` | No (`logs` needs root) | — | Security posture; sudo/auth log timeline |
| `net`, `net lan`, `net bw` | No (`lan --probe` sends packets) | — | Sockets and open ports; LAN devices; bandwidth |
| `ps` | Optional kill | Irreversible | Process list; `--kill` goes through the gate |
| `get-coffee` | No | — | Live telemetry dashboard (TUI) |

**Flags on destructive commands:** `--dry-run` prints the sealed plan and changes nothing; `-y`/`--yes` skips the
confirmation; `--deathstar` sets `confirm_mode` on (execute with no gate; it does **not** make deletion permanent);
`--jarjar trash|obliterate` sets the deletion mode. Read-only commands take `--json`. Without a flag, the gate shows
the plan grouped by reversibility and asks `[y/N]`, defaulting to No. Root is asked for once, at execution, after
the gate.

**The owner's first run on a real machine** (owner only; sessions may not run these, §1.2): `suns doctor`; then the
read-only `audit`, `net`, `get-coffee`; then `--dry-run` on `clean`, `ashen`, `maintain`, `docker-prune`; then
`ashen` on a sandbox folder of fake duplicates; then a real `suns clean`, which still asks first.

## 4. How it works

**Governing principle:** safe by default, dangerous on demand. Every potentially destructive action is a typed
`Operation` value, frozen into an immutable plan, shown through a gate graded by reversibility (reversible,
recoverable, irreversible), re-checked against the real target at the moment of use, executed, and recorded in an
append-only history. A path-based deny floor underneath can never be bypassed by any flag, mode, config or schedule.
Design source: `Docs/SunsMasterTD.md` (master TD): TD §4 operation model, TD §5.3 deny floor, TD §6 privilege.

**Layers.** `cmd/suns` → `internal/cli` (Cobra) or `internal/tui` (Bubble Tea) → engines in `internal/*` that only
**discover** and return ops → the safety spine in `pkg/*` that executes them. `pkg/*` never imports `internal/*`.

**The pipeline** (all in `pkg/`):

1. **Operation** (`pkg/operation/operation.go`): methods `Kind`, `Describe`, `Reversibility`, `ValidateAtPlan`,
   `ValidateAtExec`, `Execute`, `HistoryRecord`. Ops are pure value types so they can be sealed.
2. **Plan** (`pkg/plan`): `plan.New(ops).Seal()` deep-copies the ops into a pointer-free form with a sortable ID, and
   panics on a pointer-receiver op. What the gate shows is exactly what runs.
3. **Gate** (`pkg/safety/gate.go`): `BuildGroups(plan, mode)` groups by kind with the effective reversibility (a file
   delete is irreversible in obliterate mode); `Render` and `Summarize` print it.
4. **Revalidate and execute** (`pkg/safety/revalidate.go`): `Execute` runs `ValidateAtExec` on every op; a failed
   check is recorded `skipped:revalidation`. One bad target never stops the batch (master TD §6.1).
5. **Deny floor** (`pkg/safety/floor`): `Check(path)` at plan and exec time; list in §1.3.
6. **Identity** (`pkg/safety/identity`): small files by SHA-256; directories, symlinks and large files by size,
   mtime, device and inode. `firmlink.IsBoundary` tells a firmlink crossing from a swap.
7. **Permanent delete** (`pkg/safety/fsdelete`): `Obliterate` uses `openat`/`unlinkat` with no-follow, re-checking
   identity at the descriptor. Trash mode never reaches it.
8. **Trash** (`pkg/trash`): pure-Go move into `~/.Trash`; a target that cannot be trashed safely is skipped, never
   deleted. A per-volume circuit breaker (`breaker.go`) stops a failing volume stalling the batch. Native Cgo path
   and volume probe are seams (O-1, O-2).
9. **History** (`pkg/history`): every result appended as one JSON line (§6).

**Op kinds** (`pkg/operation/kind.go`):

| Kind | Type | Reversibility | Reaches the world through |
|---|---|---|---|
| `file_delete` | `FileDeleteOp` | Reversible (irreversible in obliterate) | `Trasher` or `fsdelete` |
| `process_kill` | `ProcessKillOp` | Irreversible | `ProcessKiller` (`internal/procadmin`) |
| `service_unload` | `ServiceUnloadOp` | Recoverable | `SystemRunner` → `launchctl` |
| `receipt_forget` | `ReceiptForgetOp` | Irreversible | `SystemRunner` → `pkgutil` |
| `dns_flush` | `DNSFlushOp` | Irreversible | `SystemRunner` → `dscacheutil`, `killall` |
| `repo_maintenance` | `RepoMaintenanceOp` | Recoverable | `SystemRunner` → `git` |
| `container_prune` | `ContainerPruneOp` | Irreversible | `ContainerPruner` → `docker` |
| `cache_reset` | none (reserved) | — | — |
| `scheduled_run` | none (history record only) | — | — |

**Injection seams** keep ops pure: `UseTrasher`, `UseProcessKiller`, `UseSystemRunner`, `UseContainerPruner`. The
defaults refuse (`ErrPrivilegeRequired`, `ErrContainerEngineUnavailable`), so an unwired op cannot mutate anything.
The CLI root wires `procadmin.New()` and `newElevatingRunner()` (`internal/cli/system_runner.go`) at startup.

**External commands** go through `pkg/syscmd`: absolute pinned paths, scrubbed environment with `LC_ALL=C`, no
shell, compile-time `productionAllowlist` (18 tools), 8 MiB output cap, context deadline. `NewWithAllowlist` serves
tests and the discovered `docker` binary, so a variable path never enters the production list. Exceptions: the two
long-running streams (`nettop`, `powermetrics` in `internal/tui/powerlaunch.go`) and the sudo calls below use
`os/exec` directly with pinned paths and `LC_ALL=C` (§10.2).

**Privilege.** `pkg/privilege` `Chokepoint` holds a per-workflow sudo ticket: `Acquire` prompts once, `Run` executes
only allowlisted actions; cancel or expiry map to `ErrPrivilegeRequired`, which ops treat as skip. The CLI's
`elevatingRunner` sends unprivileged reads straight to `syscmd` and privileged mutations through the chokepoint. Other
places use the same cached sudo ticket (§10.2): the TUI `e` key (`sudo -v` via `tea.ExecProcess`), the root process
killer (`sudo -n suns __killproc …`, which validates and kills as root in one process), and the `powermetrics`
stream (`sudo -n`).

**TUI** (`internal/tui`): Elm architecture (`model`, `Update`, `View`). The telemetry poller publishes one immutable
`SystemSnapshot` through an `atomic.Pointer` that every widget reads; `components/` draws sparklines and gauges,
`theme/` holds the palette and `NO_COLOR` handling.

## 5. Where things live

Paths are relative to the repo root. Recount packages: `go list ./... | wc -l` (36 as of 2026-11-07).

| Path | What |
|---|---|
| `cmd/suns` | Entry point: signals → `cli.Execute` |
| `assets/` | `go:embed` data: `safe_cache.json`, `oui.csv`, `sensors.json` (§6) |
| `pkg/operation` | `Operation` interface, op kinds, concrete ops, `HistoryEntry`, injection seams |
| `pkg/plan` | Sealed plans |
| `pkg/safety` | Gate, revalidate/execute; `floor/`, `identity/`, `firmlink/`, `fsdelete/` |
| `pkg/trash` | Trasher, circuit breaker, native and probe seams |
| `pkg/syscmd` | Hardened executor and `productionAllowlist` |
| `pkg/privilege` | Sudo chokepoint |
| `pkg/history` | `history.jsonl` writer, reader, redacted export |
| `pkg/plist` | Binary and XML plist reading; `powermetrics` stream tokenizer |
| `pkg/procctl` | Process identity and signalling |
| `internal/cli` | One Cobra file per command; `system_runner.go` (elevating runner); `killproc.go` (hidden `__killproc`) |
| `internal/config` | `SessionState` and `Load` (Viper, config file + flag overrides) |
| `internal/scanner`, `dedup`, `uninstaller`, `orphans`, `restore`, `purge`, `maintain`, `docker`, `scheduler` | Engines behind the destructive commands (§7) |
| `internal/audit`, `internal/net`, `internal/telemetry` | Read-only engines: audit, network, dashboard data |
| `internal/procadmin`, `internal/doctor` | Privileged process killer; self-check |
| `internal/optimizer` | Doc-only stub (§10.2) |
| `internal/tui` | TUI app, dashboard, `powerlaunch.go` stream launchers; `components/`, `theme/`, `views/` |

**Where do I look for…**

| I want to… | Open | Run |
|---|---|---|
| Add a destructive command | A new op in `pkg/operation` (copy `dnsflush.go`), an engine in `internal/<name>`, a command in `internal/cli` | `go test -race ./pkg/operation/ ./internal/<name>/` |
| Add or change a safe-cache target | `assets/safe_cache.json` | `go test -race ./internal/scanner/` |
| Protect another path | `pkg/safety/floor/floor.go` (add only) | `go test -race ./pkg/safety/...` |
| Use a new external tool | `pkg/syscmd/syscmd.go`, `internal/doctor/doctor.go`, a fixture test in the engine | `go test -race ./pkg/syscmd/ ./internal/<engine>/` |
| Fix a parser for tool output | The engine's parse function and its test fixture (e.g. `internal/net/bandwidth.go` and `bandwidth_test.go`) | That package's tests |
| Change a dashboard tile | `internal/tui/dashboard.go`; data in `internal/telemetry` | `go test -race ./internal/tui/... ./internal/telemetry/` |
| Change restore | `internal/restore`, `pkg/history` | `go test -race ./internal/restore/ ./pkg/history/` |
| Change the gate's look | `pkg/safety/gate.go`, `internal/tui/theme` | `go test -race ./pkg/safety/ ./internal/tui/...` |

## 6. Data, formats and limits

| Data | Where | Format and rule |
|---|---|---|
| Operation history | `~/Library/Application Support/Suns/history.jsonl` | Append-only JSON Lines, mode 0600, one `operation.HistoryEntry` per line, real paths (restore needs them). **No version field: add fields with `omitempty`, never rename or remove one**, or restore cannot read old entries. No rotation (L-3) |
| Config | `~/Library/Application Support/Suns/config.yaml` | Keys `confirm_mode` (bool, default off) and `deletion_mode` (`trash` default, or `obliterate`). Precedence: flags > file > defaults (`internal/config`) |
| Safe-cache allowlist | `assets/safe_cache.json` | `version` 1; 12 targets: npm, yarn, pnpm, go-mod, pip, derived-data, cocoapods, carthage, swiftpm, gradle, homebrew, and opt-in ios-device-support. Each has a `rebuild_cost`. The only paths `clean` ever touches |
| MAC vendors | `assets/oui.csv` | 56 curated rows (`AA:BB:CC,vendor`); unknown prefixes show "unknown vendor" |
| Sensor map | `assets/sensors.json` | `version` 1; board ID → sensor keys, for the IOKit seam (O-3) |
| Scheduled agent | `~/Library/LaunchAgents/com.suns.scheduled-clean.plist`, logs in `~/Library/Logs/Suns/` | Generated by `internal/scheduler` `GeneratePlist`: daily, `RunAtLoad` false, runs `clean --scheduled` |

Recount: `jq '.targets|length' assets/safe_cache.json`; `grep -cE '^[0-9A-Fa-f]{2}:' assets/oui.csv`.

**Fixed numbers:** syscmd output cap 8 MiB (`DefaultMaxOutput`); `audit logs` flags a burst at 3 or more failures in
60 s per user; dedup reads a 4 KB head before hashing. Bump `safe_cache.json` `version` when the target schema
changes (the field is parsed in `internal/scanner` but not checked yet).

## 7. Features

Every engine discovers and returns typed results plus ops; the CLI renders the gate and calls `safety.Execute`.
Engines that scrape a tool take an injected runner, so tests use captured fixtures.

### 7.1 clean (`internal/scanner`)

`LoadSafeCacheManifest` parses the allowlist; `Discover` stats each target, sizes it, computes its identity and
emits a `FileDeleteOp`. Opt-in targets need `--include-optin`; floor-protected targets are skipped. A target whose
size reads as 0 (empty, or unreadable without Full Disk Access) is not offered.

### 7.2 ashen (`internal/dedup`)

`Find` in three passes: size, 4 KB head, SHA-256. Hardlinks are never offered, `.app` bundles are units, symlinks
are never followed, APFS clones are not excluded, cosmetic xattr differences are ignored, and a keeper heuristic
picks the copy to keep. Emits `FileDeleteOp`s for the other copies.

### 7.3 nuke (`internal/uninstaller`)

`Plan` resolves the app and its bundle ID, traces `~/Library` support files and launch agents, and for `.pkg` apps
harvests `pkgutil --files`, keeping any file `pkgutil --file-info` shows another package owns. Emits in order:
`ServiceUnload` → `FileDelete` → `ReceiptForget`. Reports its scope; never claims a complete uninstall.

### 7.4 orphans (`internal/orphans`)

Resolves each LaunchAgent's `Program`/`ProgramArguments` (unwraps `env`, declines shell wrappers and relative
paths, skips Apple-managed) and flags only absolute executables that are gone. Boots them out, then trashes the plist.

### 7.5 restore (`internal/restore`)

`Candidates` keeps only reversible, trashed file deletes from history. `Restore` moves the item back, recreating
missing parents, restoring alongside on a name clash, and refusing a tampered trash entry.

### 7.6 empty-dirs, broken-symlinks, scan (`internal/purge`)

`EmptyDirs` works bottom-up and emits one op per largest empty subtree; never removes the named root.
`BrokenSymlinks` walks without following and flags links whose target is missing. `scan` reports both, read-only.

### 7.7 maintain (`internal/maintain`)

Finds `.git` repos, estimates savings (`git count-objects -v`), checks cleanliness (`git status --porcelain` and
merge/rebase/cherry-pick/revert/bisect markers) and emits `RepoMaintenanceOp` only for clean repos.
`ValidateAtExec` re-checks cleanliness just before `git gc`.

### 7.8 docker-prune (`internal/docker`)

`Detect` probes Docker Desktop, Colima, OrbStack and Rancher sockets and CLI paths, confirms the daemon and reads
`docker system df`; absent or stopped is a clean no-op. `NewPruner` runs the prune through
`syscmd.NewWithAllowlist`. The real prune is unchecked on a device (O-6).

### 7.9 dns-flush (`pkg/operation/dnsflush.go`)

One `DNSFlushOp`, root: `dscacheutil -flushcache` then `killall -HUP mDNSResponder` through the chokepoint; skips
cleanly without privilege; a failure of the second command is a failure.

### 7.10 schedule (`internal/scheduler`, `internal/cli/clean.go`)

`GeneratePlist` writes the agent (§6); `Install`/`Uninstall`/`CheckStatus` use `launchctl bootstrap`/`bootout`/
`print` in `gui/<uid>` (unchecked on device, O-7). `clean --scheduled` (`runScheduledClean`) forces trash, uses
only non-opt-in allowlist targets, ignores flags and config, and writes a `scheduled_run` record with status `ok`,
`partial` or `failed` (see L-2).

### 7.11 audit (`internal/audit`)

`Posture`: SIP (`csrutil`), Gatekeeper (`spctl`), FileVault (`fdesetup`), XProtect version, each ok/warn/risk/unknown
with a worst-of overall. `AuthLog`: `log show` for `sudo`, classified success/failure/denied, with burst detection.
Parsers degrade to unknown.

### 7.12 net (`internal/net`)

`Sockets` parses `lsof -i -n -P -F` into process → address:port and flags externally reachable listeners. `LANScan`
parses `arp -a -n`, adds vendor and reverse DNS, with an optional warned TCP probe. `Bandwidth`: per-interface rates
from gopsutil (reliable) plus **experimental** per-process rates from two `nettop -P -x -L 2` (CSV) samples.

### 7.13 get-coffee dashboard (`internal/telemetry`, `internal/tui/dashboard.go`)

The poller samples CPU per core, memory and swap, all mounted physical volumes, disk I/O, uptime/load, top processes
and battery (pmset). Tiles: CPU, MEMORY, GPU, THERMAL, DISK I/O, DISK (fullest volume), BATTERY, POWER, each with a
health badge (live, stale, N/A) so dead data never looks live. GPU, thermal and power come from the `powermetrics`
stream after `e` elevates. The **experimental** NET widget diffs a supervised `nettop -P -x -l 0` stream
(`NetSource`, fixed-width output, §11.3). Keys: arrows select, `k` kill (through the gate), `e` elevate, `p` pause,
`esc` back.

### 7.14 ps and process kill (`internal/cli/ps.go`, `internal/procadmin`, `pkg/procctl`)

`ProcessKillOp` captures PID, birth time and executable; own-user processes are validated and signalled directly,
others through `sudo -n suns __killproc`, which re-validates as root. Zombies are reported, never killed.

### 7.15 doctor (`internal/doctor`)

Checks platform, Go runtime, home/trash/history folders, pinned tools present (missing two, L-5), and Full Disk Access
by probing one protected path.

### 7.16 TUI menu (`internal/tui/app.go`)

Menu: Clean, Restore, Get Coffee, Audit, Doctor, Quit, with chips showing the deletion and confirm modes. Restore
only lists; Audit shows a placeholder (O-13).

## 8. Build, release and deploy

**Toolchain:** Go 1.25 language, toolchain `go1.26.4` (`go.mod`); macOS. Direct dependencies: cobra, viper,
bubbletea, bubbles, lipgloss, gopsutil v4, `howett.net/plist`, `golang.org/x/sys`. Listed in `go.mod` comments but
not required yet: harmonica, ntcharts, bubblezone (O-16).

**Verify (must be green):** `go build ./... && go vet ./... && go test -race ./...`. Add `-count=1` to bypass the
test cache. `gofmt` is checked by a hook after edits.

**Run without installing:** `go run ./cmd/suns --help` (only help, `--dry-run` or `--version` from a session).

**Release:** not released; the pipeline is parked (R-1). The version is `0.0.0-dev`, set at release through
`-ldflags` on `Version` in `internal/cli/root.go`. No signing material exists in the repo. Distribution options
without a Developer ID, noted 2026-06-26 for when it is reopened [doc]: source build, an unsigned GitHub release
binary (users clear quarantine with `xattr`), or a Homebrew tap that builds from source. A `.pkg` is impractical
unsigned.

## 9. Testing

| Suite | Guards | Run |
|---|---|---|
| `pkg/*` unit tests | The safety spine: sealing, floor, identity checks, fd-anchored delete, trash fallback and breaker, allowlist, chokepoint, history (§1.3) | `go test -race ./pkg/...` |
| Engine tests (`internal/*`) | Discovery logic and tool parsers against captured fixtures (lsof, nettop, `log show` JSON, git porcelain, `docker system df`, pkgutil) via injected runners | `go test -race ./internal/...` |
| TUI tests (`internal/tui`) | Menu, toggles, modal, dashboard render, kill flow | `go test -race ./internal/tui/...` |

All tests run under `-race`, deletion tests use only `t.TempDir()`, and none runs a real destructive or privileged
command. A new test must fail without the fix it covers.

**Not covered, and why:** `internal/cli`, `internal/config`, `internal/doctor`, `internal/procadmin`,
`internal/optimizer`, `internal/tui/theme`, `internal/tui/views`, `pkg/procctl`, `pkg/safety/identity` and
`pkg/safety/firmlink` have no test files; identity and firmlink are exercised through the op and floor tests, the CLI
through its engines (O-15). Live paths that need a device, root or Docker (O-1 to O-7) cannot run headless.

## 10. Decisions

### 10.1 Owner rulings

| ID | Ruling | Date or source | Why |
|---|---|---|---|
| R-1 | The signed and notarized release pipeline stays the plan, deferred for now; `lang-strip` waits with it | 2026-11-07 [owner]; earlier recorded in the old stats.md §9 [doc] | Needs Apple Developer credentials and on-device signing |
| R-2 | `lang-strip` is quarantined and ships last, opt-in per app, verified with `codesign --verify` | `Docs/Phases.md` §4.7, master TD §12.4 | Deleting `.lproj` invalidates code signatures |
| R-3 | Scheduled runs are trash-only, allowlist-only, ignore flags and config, and record `ok`/`partial`/`failed` | Master TD §12.20 | Unattended deletion is the most dangerous mode |
| R-4 | Per-process network attribution ships labelled experimental and degrades to "unavailable" | Master TD §7 | `nettop` output is fragile; never show wrong numbers confidently |

### 10.2 Deviations from the spec

| Spec (`Docs/SunsMasterTD.md`) | Code |
|---|---|
| TD §6: all elevation through one chokepoint | Four places share the sudo ticket: `pkg/privilege`, TUI `elevateCmd` (`sudo -v`), `internal/procadmin` (`sudo -n suns __killproc`), `powermetricsLauncher` (`sudo -n`) [inferred] |
| TD §6.3: every external tool through `pkg/syscmd` | The `nettop` and `powermetrics` streams and the sudo calls use `os/exec` directly (streams need a live pipe, not a capped buffer), with pinned paths and `LC_ALL=C` |
| TD §8: `optimizer` holds DNS flush and `mdutil` actions | `internal/optimizer` is a doc stub; DNS flush is `DNSFlushOp`; `mdutil` is allowlisted but unused |
| TD §4.2: `CacheReset` op kind | Constant reserved, no concrete op |
| TD §12.1: `dedup` command | Named `ashen`; `dedup` is a hidden alias |
| TD §4.4: bounded Cgo trash | Pure-Go fallback only (O-1) |

### 10.3 Accepted, don't chase

| Item | Reopen if |
|---|---|
| `oui.csv` is a 56-row curated subset, so many vendors show "unknown vendor" | Owner wants full vendor names |
| Per-process bandwidth (`net bw`, NET widget) may show "unavailable" | A macOS update breaks both nettop formats |

## 11. Limitations, traps and diagnoses

### 11.1 Known limitations

| ID | Limitation | Tracked |
|---|---|---|
| L-1 | Without Full Disk Access, protected caches in `~/Library` are skipped; `doctor` warns, but nothing explains it on first run | O-10 |
| L-2 | A scheduled clean without Full Disk Access records `ok`, "nothing to clean": unreadable targets size as 0 and are not offered, and discovery skips are not counted in the status [inferred] | O-11 |
| L-3 | `history.jsonl` grows without limit | O-9 |
| L-4 | TUI Restore cannot restore and TUI Audit is a placeholder | O-13 |
| L-5 | `doctor` checks 16 of the 18 allowlisted tools (`killall`, `mdutil` missing), so it can report all present when they are not | O-14 |
| L-6 | GPU, thermal and power tiles stay N/A until `e`; IOKit thermals are not built | O-3, O-4 |

### 11.2 Traps

| Trap | Fix |
|---|---|
| `nettop -l` prints fixed-width columns; `-L` prints CSV (`man nettop`). The two code paths use different flags | Parse by header: `parseNettopHeader` (`internal/telemetry/netsource.go`) accepts both; `net bw` uses `-L` |
| A second package comment outside `doc.go` | `go vet` fails; keep package docs in `doc.go` |
| `suns version` is not a subcommand | Use `suns --version` (Cobra `Version`) |
| Trying to check a fix by running `suns` | The hook blocks it; write a fixture test, or ask the owner to run it with `!` |

### 11.3 Diagnoses

**Live NET widget empty or stuck warming** (fixed 2026-06-26, commit "Fixed Nettop Parsing Issues").
Symptom: the experimental NET widget in `get-coffee` never shows talkers [inferred]. Cause: the stream runs
`nettop -P -x -l 0`, which prints fixed-width frames whose header starts `time ` (spaces), while the parser only
accepted a CSV header starting `time,`. Recognise it: captured stream output with space-aligned columns, and
`NetSource` never setting its columns. Fix: `parseNettopHeader` detects CSV or fixed-width and `parseNetRowFixed`
reads columns at the header's byte offsets; tests `TestParseNettopHeaderFixed`, `TestParseNetRowFixed`.

## 12. Documents

| Doc | Status | Read it before touching | Trust it for |
|---|---|---|---|
| `Docs/SunsMasterTD.md` | Current spec (v6) | Any new feature or safety change | Intent and rationale; the code deviates in §10.2 |
| `Docs/Phases.md` | Historical plan, current for parked items | `lang-strip`, the release pipeline, on-device seams | Implementation notes per phase; its §N cite the master TD |
| `Docs/Changelog.md` | Current | — | All history, including the old phase table |
| `README.md` | Partly stale (O-17) | — | Overview and layout; not status |
| `.claude/rules/safety.md` | Current, loaded for `pkg/` safety packages | The safety spine | The rules of §1.1 rule 6 |
| `Docs/Legacy/stats-2026-11-07.md` | Historical | — | The pre-overhaul stats.md, word for word |

## 13. How this doc is verified

**Evidence tags:** untagged claims were read from the code on the *Verified* date. `[build]` seen in a build or test
run · `[device]` seen on a Mac running `suns` · `[console]` seen in an outside console · `[doc]` reported by another
doc, not re-checked · `[inferred]` worked out from the code, not run · `[owner]` reported by the owner.

**How facts were checked (2026-11-07):** commands, flags and aliases from `internal/cli`; op kinds and
reversibility from `pkg/operation`; floor list from `pkg/safety/floor/floor.go`; tools from `productionAllowlist`
and `pinnedTools`; invariant guards by test name; asset counts with the commands below.

**Re-verify:**

```bash
go build ./... && go vet ./... && go test -race -count=1 ./...
grep -rhE '^func (Test|Fuzz)' --include='*_test.go' . | wc -l        # test functions (172)
go test ./... 2>&1 | grep -c '^ok'                                    # packages with tests (24)
go list ./... | wc -l                                                 # packages (36)
jq '.targets|length' assets/safe_cache.json                           # safe-cache targets (12)
grep -cE '^[0-9A-Fa-f]{2}:' assets/oui.csv                            # OUI rows (56)
go list -f '{{.ImportPath}}: {{.Imports}}' ./pkg/... | grep -c 'Suns/internal'   # must be 0
```

**Cannot be checked from a session:** anything that needs `suns` to run (§1.2), root, Docker, or the owner's Mac
state (O-1 to O-7). Those facts are tagged or listed as open.

- Not repo paths: `~/Library/Application Support/Suns/history.jsonl`, `~/.Trash`, `~/Library/Logs/Suns/`
- Not repo paths: `~/Library/Application Support/Suns/config.yaml`
- Not repo paths: `~/Library/LaunchAgents/com.suns.scheduled-clean.plist`
- Not repo paths: `~/.claude/hooks/suns-guard.sh`, `~/.claude/hooks/gofmt-check.sh`

## 14. Glossary

| Term | Meaning |
|---|---|
| Burn | Suns' word for clean or delete; scheduled cleans are "scheduled burns" |
| ashen | The duplicate finder ("burnt carbon copies") |
| nuke | The precision uninstaller |
| get-coffee | The live telemetry dashboard |
| Jarjar | The deletion-mode axis: `trash` (default) or `obliterate` (permanent); `--jarjar` |
| Deathstar | `confirm_mode` on: execute without the gate; `--deathstar`. Does not change trash vs obliterate |
| Gate | The grouped, reversibility-graded preview and `[y/N]` confirmation |
| Deny floor | The path rules in `pkg/safety/floor` no flag can bypass |
| Seal | `plan.Seal()`: freeze a plan into a pointer-free value copy |
| Seam | An interface or file left for a later or on-device implementation (`native.go`, `iokit.go`) |
| Chokepoint | `pkg/privilege`, the sudo ticket holder |
| Reversible / Recoverable / Irreversible | Undo with `restore` / rebuilds or recoverable by other means / gone |
| Master TD | `Docs/SunsMasterTD.md`; its §N differ from this doc's §N |
