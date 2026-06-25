# Suns — Project Status & Complete Technical Breakdown (`stats.md`)

> **What this document is.** A single, exhaustive reference for anyone working on
> Suns: the architecture, every package and file, the safety model, every CLI
> command and how to use it, the data assets, the test suite, the current
> phase-by-phase status, and what remains. It is written to be read top-to-bottom
> by a newcomer or used as a lookup by a contributor. It reflects the repository
> as of the most recent commit on `main` (`fa81f5f Phase 4: Maintenance`).

---

## 0. TL;DR — what Suns is and where it stands

**Suns** ("Super User Nutcase Sessions — Burn It All") is a safety-obsessed macOS
system-maintenance utility written in Go. It cleans dev caches, uninstalls apps,
de-duplicates files, purges filesystem cruft, audits security posture and the
network, runs a live telemetry dashboard, and schedules unattended cleanups —
all behind a uniform safety apparatus where **a first run with no flags can never
permanently destroy data**.

| Metric | Value |
| --- | --- |
| Language / module | Go 1.25 (toolchain 1.26) · `github.com/anumey1/Suns` |
| Total Go source | ~15,600 lines across 34 packages |
| `pkg/` (reusable spine) | ~4,770 LOC |
| `internal/` (engines, CLI, TUI) | ~10,790 LOC |
| Test functions | **170** across 24 test packages |
| Test result | **all green** under `go test -race ./...` (~13 s cold) |
| Platform | macOS (darwin), Apple Silicon + Intel |
| Phases complete | **0, 1, 2, 3, and Phase 4 in-scope** (see §9) |
| Deferred | `lang-strip` (§4.7) + the release pipeline (§4.8) |

**Design north star:** every potentially-destructive action is a typed
`Operation`, frozen into an immutable plan, shown through a colour-graded **gate**
(🟢 reversible / 🟡 recoverable / 🔴 irreversible), executed only after a
time-of-use identity re-check, and recorded in an append-only history. Privileged
actions funnel through a single audited sudo chokepoint. External tools run
through a hardened, allow-listed executor.

---

## 1. Repository layout

```
Suns/
├── cmd/suns/main.go            Entry point (wires signals → cli.Execute)
├── assets/                     Embedded data (go:embed)
│   ├── assets.go               embed directives
│   ├── safe_cache.json         curated safe-cache allowlist (12 targets)
│   ├── oui.csv                 curated MAC-vendor subset (~76 rows)
│   └── sensors.json            board-ID → sensor-key map (IOKit, carry-forward)
├── pkg/                        Reusable, app-agnostic safety spine
│   ├── operation/              the typed Operation model + concrete ops
│   ├── plan/                   immutable sealed plans
│   ├── safety/                 deny-floor, gate, revalidate/execute
│   │   ├── floor/  fsdelete/  identity/  firmlink/
│   ├── trash/                  macOS Trash with pure-Go fallback + circuit breaker
│   ├── syscmd/                 hardened external-command executor (allowlist)
│   ├── privilege/              the single sudo chokepoint
│   ├── plist/                  binary+XML plist reader + streaming tokenizer
│   ├── history/                append-only JSONL operation history
│   └── procctl/                low-level process identity + signal
├── internal/                   App-specific engines, CLI, TUI
│   ├── cli/                    Cobra command tree (one file per command)
│   ├── config/                 SessionState + config load (viper)
│   ├── scanner/                safe-cache discovery (clean)
│   ├── dedup/                  the "ashen" duplicate finder
│   ├── uninstaller/            "nuke" precision uninstaller
│   ├── orphans/                orphaned launch-agent finder
│   ├── restore/                undo trashed FileDeletes
│   ├── purge/                  empty-dir + broken-symlink engines
│   ├── maintain/               git-repo GC discovery
│   ├── docker/                 container-engine detect + prune
│   ├── scheduler/              launchd LaunchAgent authoring
│   ├── optimizer/              (doc stub — reset-style ops live as concrete ops)
│   ├── audit/                  security posture + auth-log analyzer
│   ├── net/                    sockets, ports, LAN scan, bandwidth
│   ├── telemetry/              the get-coffee data poller + supervised streams
│   ├── procadmin/              privilege-delegated process killer
│   ├── doctor/                 environment/capability self-check
│   └── tui/                    Bubble Tea TUI (menu, dashboard, modals)
│       ├── components/  theme/  views/
└── Docs/
    ├── SunsMasterTD.md         the v6 master technical design (source of truth)
    ├── Phases.md               phase-by-phase plan + completion log
    └── stats.md                ← this document
```

**Dependency direction:** `internal/*` may import `pkg/*`; `pkg/*` never imports
`internal/*`. Concrete ops live in `pkg/operation`; the engines that *produce*
ops live in `internal/*`; the CLI wires them together.

---

## 2. Dependencies (`go.mod`)

Direct requires (all else is indirect):

| Module | Role |
| --- | --- |
| `github.com/spf13/cobra` | command & flag routing |
| `github.com/spf13/viper` | config file load (once, into `SessionState`) |
| `github.com/charmbracelet/bubbletea` | TUI runtime (Elm architecture) |
| `github.com/charmbracelet/bubbles` | prebuilt TUI widgets (spinner, help) |
| `github.com/charmbracelet/lipgloss` | styling & layout |
| `github.com/shirou/gopsutil/v4` | in-process CPU/mem/disk/net/process stats |
| `howett.net/plist` | binary + XML plist decode/encode |
| `golang.org/x/sys` | `openat`/`unlinkat`/`statfs` for the fd-anchored deleter |

Planned-but-not-yet-wired (listed in `go.mod` comments): `harmonica` (spring
easing), `ntcharts` (braille charts), `bubblezone` (mouse). These are cosmetic
backlog items (§10).

---

## 3. The safety architecture (the heart of Suns)

Everything destructive flows through one pipeline. Understanding this pipeline is
understanding the project.

### 3.1 The `Operation` model (`pkg/operation`)

Every auditable, potentially-destructive action is a **typed value-type**
implementing the `Operation` interface (`operation.go`):

```go
type Operation interface {
    Kind() OpKind
    Describe() Preview                                  // gate render payload
    Reversibility() Reversibility                       // 🟢/🟡/🔴 intrinsic
    ValidateAtPlan(ctx) error                           // sane at discovery?
    ValidateAtExec(ctx) (Identity, error)               // still the same target? (TOCTOU)
    Execute(ctx, mode Mode, id Identity) (Receipt, error)
    HistoryRecord(Receipt) HistoryEntry
}
```

Ops are **pure value types** (no pointers/slices/maps in fields) so that
`plan.Seal()` can deep-copy them into a pointer-free form retaining no reference
into live state. `plan.Seal()` panics on a pointer-receiver op — a compile-of-intent
guard.

**Op kinds** (`kind.go`):

| `OpKind` | Concrete type | Default badge | Reaches the world via |
| --- | --- | --- | --- |
| `file_delete` | `FileDeleteOp` | 🟢 Reversible (🔴 in obliterate) | `Trasher` / `fsdelete` |
| `process_kill` | `ProcessKillOp` | 🔴 Irreversible | `ProcessKiller` |
| `service_unload` | `ServiceUnloadOp` | 🟡 Recoverable | `SystemRunner` → launchctl |
| `receipt_forget` | `ReceiptForgetOp` | 🔴 Irreversible | `SystemRunner` → pkgutil |
| `dns_flush` | `DNSFlushOp` | 🔴 Irreversible | `SystemRunner` → dscacheutil/killall |
| `repo_maintenance` | `RepoMaintenanceOp` | 🟡 Recoverable | `SystemRunner` → git |
| `container_prune` | `ContainerPruneOp` | 🔴 Irreversible | `ContainerPruner` → docker |
| `cache_reset` | *(reserved)* | — | — |
| `scheduled_run` | *(history summary record, not an op)* | — | — |

**Reversibility** ∈ `Reversible` / `Recoverable` / `Irreversible`. **Mode** (the
"Jarjar" deletion axis) ∈ `ModeTrash` / `ModeObliterate`; it applies **only** to
`FileDelete` and is inert for every other kind.

**Injection seams** (package-level, mirror each other, keep ops pure value types):

- `UseTrasher(*trash.Trasher)` — FileDelete trashing (shared per batch so the
  volume circuit-breaker spans the run).
- `UseProcessKiller(ProcessKiller)` — own-user direct / root-or-other-user
  delegated under sudo.
- `UseSystemRunner(SystemRunner)` — `Run(ctx, privileged bool, name, args...)`;
  the default refuses privileged calls (`ErrPrivilegeRequired`), so accidental
  unelevated mutation is impossible.
- `UseContainerPruner(ContainerPruner)` — docker prune; default refuses
  (`ErrContainerEngineUnavailable`).

`Receipt` (Execute's outcome) carries `Fate` (`trashed`/`obliterated`/`killed`/
`flushed`/`unloaded`/`forgotten`/`collected`/`pruned`/`skipped`) and `Status`
(`ok`/`skipped:<reason>`/`failed`). `HistoryEntry` is the typed JSONL record (one
struct, per-kind fields populated with `omitempty`).

### 3.2 The plan (`pkg/plan`)

`plan.New(ops).Seal()` returns an immutable `*Plan` with a time-sortable `ID`
(`NewID()`) and a pointer-free deep copy of the ops. The sealed plan is what the
gate renders and the executor runs — nothing can mutate the target set between
preview and execution.

### 3.3 The gate (`pkg/safety/gate.go`)

`BuildGroups(plan, mode)` groups ops by `OpKind`, computing the **effective**
reversibility (e.g. FileDelete becomes 🔴 in obliterate mode). `Render(groups,
maxLines)` prints the grouped preview with a single badge per group and a totals
line. `Badge(r)` maps reversibility → `🟢 Reversible` / `🟡 Recoverable` /
`🔴 Irreversible`. `Summarize(groups)` totals counts and reclaimable bytes.

### 3.4 Revalidate + execute (`pkg/safety/revalidate.go`)

`Execute(ctx, plan, mode)`:
1. `Revalidate` runs `ValidateAtExec` on **every** op — the TOCTOU / PID-reuse
   defense. A failed re-check is recorded as `skipped:revalidation`, never run.
2. Surviving ops `Execute`; each `ExecResult{Op, Receipt, Entry, Skipped, Err}`
   is returned. One bad target never aborts the batch (§6.1 partial-batch
   semantics).

### 3.5 The deny-floor (`pkg/safety/floor`)

`floor.Check(path)` is the immovable backstop — **purely path-and-volume based**,
checked at both plan and exec time by every FileDelete-ish op. It denies:

- Prefix families: `/System`, `/usr` (except `/usr/local`), `/bin`, `/sbin`,
  `/Library/Apple`, `/private/var/db`, `/cores`.
- The filesystem root `/`.
- The iCloud Drive mobile-documents container.
- **Anything on a read-only System Volume** (`unix.Statfs` + `MNT_RDONLY`) —
  closes the §5.3 "anything on a System Volume" gap structurally.

`floor.Permits(path)` is the boolean form. The manifest should never contain a
floor-protected target; the scanner skips defensively if it ever does.

### 3.6 Tiered identity (`pkg/safety/identity`)

`ComputeFile(path, threshold)` builds a `FileIdent`:
- **Small regular files** → SHA-256 content hash (`TierSmall`).
- **Directories, symlinks (never followed), and files ≥ threshold** → metadata
  tier (size + mtime + dev + inode) (`TierLarge`).

`VerifyFile(expected, current, allowDevChange)` is the exec-time gate: tier, dev
(unless a known firmlink boundary permits it), inode, size, and either mtime
(large) or hash (small) must all match — defeating swap/replace/PID-reuse style
attacks. `firmlink.IsBoundary(path)` decides when a device-number change is an
OS-synthesized firmlink crossing rather than a swap.

### 3.7 The fd-anchored deleter (`pkg/safety/fsdelete`)

`Obliterate(path, expected)` performs permanent deletion using **no-follow,
fd-anchored** syscalls (`openat`/`unlinkat` via `x/sys/unix`), re-verifying
identity at the descriptor so a symlink swapped in mid-operation cannot redirect
the delete. Trash mode never reaches here; obliterate does.

### 3.8 The Trasher (`pkg/trash`)

`trash.New()` returns a `Trasher` that moves files to the macOS Trash. Layers:
- A **native Cgo path** (`native.go`) — `NSFileManager trashItemAtURL:` — is the
  intended fast path (currently a carry-forward seam).
- A **pure-Go `~/.Trash` fallback** that never permanently deletes; if a target's
  trashability cannot be guaranteed it is **skipped**, never escalated to delete.
- A **volume-scoped circuit breaker** (`breaker.go`): repeated failures on a
  volume trip it so a hung/failing volume can't stall a whole batch.
- A **killable volume probe** (`probe.go`) — carry-forward seam for hang
  detection.

### 3.9 The privilege chokepoint (`pkg/privilege`)

`Chokepoint` is the **single** place Suns elevates. Per-workflow sudo ticket;
`Acquire` prompts once via a `Prompter` (`TerminalPrompter` in the CLI); `Run`
executes an allow-listed pinned command under sudo. Errors are typed
(`ErrCanceled`, `ErrExpired`, `ErrActionNotAllowed`). The CLI's `elevatingRunner`
(`internal/cli/system_runner.go`) adapts it to the op `SystemRunner` seam:
unprivileged reads go direct through `syscmd`, privileged mutations go through the
chokepoint, and cancel/expire map to `ErrPrivilegeRequired` (graceful skip).

### 3.10 The hardened executor (`pkg/syscmd`)

Every external command — privileged or not — goes through a `Runner`:
- **Absolute pinned paths** (no `$PATH` lookup → no hijack).
- **Scrubbed env** with `LC_ALL=C` (locale-deterministic, parseable output).
- **No shell** (`exec.Command` with an arg slice → no shell injection).
- **Compile-time allowlist** (`productionAllowlist`) — a non-listed command is a
  programming error (`ErrNotAllowed`).
- **Bounded output** (8 MiB cap, truncation flagged) and a context deadline.

`New()` uses the production allowlist; `NewWithAllowlist(map)` is used for tests
and for the **discovered docker binary** (so a variable path never enters the
production allowlist). Pinned tools: `lsof, arp, nettop, powermetrics, pkgutil,
dscacheutil, killall, log, csrutil, spctl, fdesetup, mdutil, pmset, vm_stat,
sysctl, launchctl, sudo, git`.

### 3.11 The history (`pkg/history`)

Canonical record at `~/Library/Application Support/Suns/history.jsonl` (mode
0600, append-only JSON Lines, one self-contained object per line). `Open`/`Append`/
`ReadAll`. **Full fidelity** (real paths) because restore depends on it;
`ExportRedacted(src,dst)` produces a redacted copy on demand (home-relative path
hashing, process-name elision) without weakening the canonical record.

---

## 4. Engines (`internal/*`) — feature by feature

Each engine is read-only at heart: it **discovers** and returns typed structs +
the `Operation`s to run; the CLI renders the gate and calls `safety.Execute`.
Every engine takes an injectable runner where it scrapes a tool, so tests use
fixtures.

### 4.1 `scanner` — safe-cache discovery (powers `clean`)
- `LoadSafeCacheManifest()` parses the embedded `assets/safe_cache.json`.
- `Discover(ctx, targets, opts)` stats each allowlist target, computes
  reclaimable size + tiered identity, and emits `FileDeleteOp`s. Opt-in targets
  (e.g. iOS DeviceSupport) excluded unless `IncludeOptIn`. Floor-protected
  targets skipped defensively. **Read-only.**

### 4.2 `dedup` — the "ashen" duplicate finder (powers `ashen`)
- `Find(ctx, roots, opts) → Report`. Three-pass: size → 4 KB head → SHA-256.
  Collapses hardlinks (never offered), bundle-atomic (`.app` treated as a unit),
  never follows symlinks, floor-guarded, APFS shared-block clones never excluded,
  keeper heuristic picks which copy to retain, cosmetic xattr differences
  normalized. Emits `FileDeleteOp`s for the redundant copies. Theme: "ash is
  largely carbon; burnt carbon copies lie in an ashen state." **9 tests.**

### 4.3 `uninstaller` — "nuke" precision uninstaller
- `Plan(ctx, target, opts) → Result`. Resolves the app, reads its bundle ID
  (binary-safe plist), traces `~/Library` support files + launch agents, and for
  `.pkg`-installed apps harvests the payload via `pkgutil --files`, runs
  `pkgutil --file-info` per path and **retains files claimed by >1 package**
  (shared-dependency guard, prevents bricking other apps). Emits ordered ops:
  `ServiceUnload` → `FileDelete` → `ReceiptForget`. Reports explicit scope
  bounds; never claims a "complete uninstall."

### 4.4 `orphans` — orphaned launch-agent finder
- `Find(ctx, opts) → Report`. Resolves each LaunchAgent's `Program`/
  `ProgramArguments` (env-launchers unwrapped, shell wrappers + relative paths
  declined, Apple-managed skipped) and flags only **genuinely-absent absolute
  executables**. Emits `FileDelete`s for the dead plists.

### 4.5 `restore` — undo trashed FileDeletes
- `Candidates(entries)` filters history to **reversible, trashed FileDeletes
  only** (Phase-2 op kinds explicitly excluded). `Restore(entry) → Outcome`
  moves the item back, recreating missing parents and restoring alongside on a
  name collision; refuses a tampered trash entry. **7 tests** incl. a >50 MB
  sparse round-trip.

### 4.6 `purge` — empty-dir + broken-symlink engines
- `EmptyDirs(ctx, root, opts)` — collapsibility computed **bottom-up**; a dir is
  empty when every entry is a `.DS_Store` or an already-collapsible subdir
  (`.DS_Store`-only counts as empty). Emits **one FileDeleteOp per maximal empty
  subtree** (trashing it subsumes the rest). Never removes the named root; no-follow.
- `BrokenSymlinks(ctx, roots, opts)` — no-follow walk; dangling detected when
  `os.Stat` reports the target missing (other stat errors left alone). Emits a
  FileDelete per dangling link. **6 tests.**

### 4.7 `maintain` — git-repo garbage collection
- `Discover(ctx, runner, roots, opts) → Result`. Walks for `.git`, estimates
  savings (`git count-objects -v` size + size-garbage), assesses cleanliness
  (`git status --porcelain` + merge/rebase/cherry-pick/revert/bisect marker
  files), and emits a `RepoMaintenanceOp` **only for clean repos** — dirty ones
  are listed with a reason. The op's `ValidateAtExec` re-confirms cleanliness
  immediately before `git gc`. **2 tests.**

### 4.8 `docker` — container-engine detect + prune
- `Detect(ctx) → Status`. Probes sockets (`~/.docker/run`, `~/.colima/default`,
  OrbStack, `/var/run`), locates the docker CLI across Docker Desktop / Colima /
  OrbStack / Rancher install paths, confirms the daemon (`docker version`), and
  estimates reclaimable space from `docker system df`. Absent/stopped → clean
  no-op.
- `NewPruner(bin)` runs `docker system prune -a -f [--volumes]` through
  `syscmd.NewWithAllowlist`. The prune execute path is an **on-device seam**;
  parsers (`parseHumanSize`, `parseReclaimable`, `parseReclaimedLine`) + detect/
  prune cores are unit-tested with an injected runner. **7 tests.**

### 4.9 `scheduler` — launchd LaunchAgent authoring
- `GeneratePlist(cfg) → []byte` (pure, howett XML): `com.suns.scheduled-clean`,
  `RunAtLoad` false, daily `StartCalendarInterval`, args `clean --scheduled`,
  logs under `~/Library/Logs/Suns/`. `Install`/`Uninstall`/`CheckStatus` use
  `launchctl bootstrap`/`bootout`/`print` in `gui/<uid>` (on-device). **4 tests.**

### 4.10 `audit` — security posture + auth-log analyzer
- `Posture(ctx, runner) → Report`. SIP (`csrutil status`), Gatekeeper
  (`spctl --status`), FileVault (`fdesetup status`), XProtect version (plist) →
  per-finding severity (ok/warn/risk/unknown) + worst-of overall. Parsers degrade
  to `unknown`.
- `AuthLog(ctx, runner, opts) → AuthReport`. Queries the unified log
  (`log show --style json --predicate 'process=="sudo"' --last <since> --info`),
  classifies success/failure/denied, detects per-user rapid-failure bursts
  (≥3 in 60 s). **11 tests.**

### 4.11 `net` — sockets, ports, LAN, bandwidth
- `Sockets(ctx, runner, opts) → Report`. Parses `lsof -i -n -P -F` field mode →
  PID→app→addr:port, classifies listening sockets by scope
  (wildcard/loopback/specific) and flags externally-reachable ports.
- `LANScan(ctx, runner, opts) → LANReport`. Parses `arp -a -n` (skips
  incomplete/broadcast/multicast), OUI vendor lookup (curated subset), reverse
  DNS, optional warn-gated active TCP probe.
- `Bandwidth(ctx, runner, opts) → BWReport`. Reliable per-interface rates
  (gopsutil counter delta over a window) + **experimental** per-process tx/rx by
  differencing a two-sample `nettop` capture (degrades cleanly). **14 tests.**

### 4.12 `telemetry` — the get-coffee data layer
- `Poller` samples once per cadence and publishes an immutable `SystemSnapshot`
  via `atomic.Pointer` (every widget reads the same snapshot; no per-widget
  polling). Cheap sources (CPU per-core, memory + swap + pressure, disk usage
  across **all mounted physical volumes**, disk I/O rates, host uptime/load, top
  processes, battery via pmset) are live; heavy/root sources show honest
  staleness/Unavailable badges until elevated.
- `PowerSource` — supervised, deadline-guarded `powermetrics -f plist` stream
  (GPU/thermal/power), launched after `e` elevate.
- `NetSource` — supervised, unprivileged `nettop -P -x -l 0` stream diffed into
  per-process tx/rx for the **experimental** live NET widget; stall → relaunch.
- `iokit.go` / `battery.go` — IOKit thermals (carry-forward Cgo seam) and pmset
  battery parsing. **13 tests.**

### 4.13 Supporting engines
- `procadmin` — `Killer` adapting `pkg/procctl` to the op `ProcessKiller` seam
  (own-user direct, root/other-user delegated under sudo).
- `doctor` — `Run(ctx) → Report` of environment checks: platform, Go runtime,
  home/trash/history dirs, pinned external tools present, Full Disk Access.
- `config` — `SessionState` (confirm_mode, deletion_mode) + `Load(overrides)`
  (viper file merge with precedence). `optimizer` is a doc-only stub (its
  reset-style actions ship as concrete ops, e.g. `DNSFlushOp`).

---

## 5. The CLI (`internal/cli`) — every command

Built with Cobra. Running `suns` with **no subcommand** launches the interactive
TUI. The root wires the privilege-capable seams at startup:
`UseProcessKiller(procadmin.New())` and `UseSystemRunner(newElevatingRunner())`.

| Command | Destructive? | Badge | Summary |
| --- | --- | --- | --- |
| `suns` (no args) | — | — | Launch the interactive TUI |
| `suns doctor` | no | — | Environment & capability self-check |
| `suns clean` | yes | 🟢/🔴 | Trash dev caches from the safe-cache allowlist |
| `suns clean empty-dirs <path>` | yes | 🟢 | Purge empty dirs (`.DS_Store`-only = empty), scope-confirmed |
| `suns clean broken-symlinks <path...>` | yes | 🟢 | Remove dangling symlinks, scope-confirmed |
| `suns clean --scheduled` | yes | 🟢 | Constrained unattended mode (launchd agent) |
| `suns scan <path...>` | no | — | Audit broken symlinks + empty dirs (`--json`) |
| `suns ashen [path...]` | yes | 🟢/🔴 | Duplicate finder (Trash default; obliterate opt-in) |
| `suns nuke <app>` | yes | 🟡🔴 | Precision uninstall: bundle + support + agents + receipts |
| `suns orphans` | yes | 🟢 | Purge launch agents whose executable is gone |
| `suns restore` | no (undo) | — | List & restore trashed FileDeletes |
| `suns maintain [path...]` | yes | 🟡 | Safe `git gc` across clean repos |
| `suns docker-prune` | yes | 🔴 | `docker system prune -a` (volumes opt-in) |
| `suns dns-flush` | yes (root) | 🔴 | Flush DNS cache + restart mDNSResponder |
| `suns schedule install\|uninstall\|status` | authoring | — | Manage the launchd cleanup agent |
| `suns audit` (alias `secure`) | no | — | SIP/Gatekeeper/FileVault/XProtect posture (`--json`) |
| `suns audit logs` | no (root) | — | Sudo/auth-log timeline + burst detection (`--json`) |
| `suns net` | no | — | Socket map + open-port reachability audit (`--json`) |
| `suns net lan` | no | — | LAN device discovery (warn-gated `--probe`) |
| `suns net bw` | no | — | Per-interface + experimental per-process bandwidth |
| `suns ps` | no | — | Process listing (kill is gated) |
| `suns get-coffee` | no | — | Live telemetry dashboard (TUI) |

**Common destructive-command flags:** `--dry-run` (print the frozen plan, change
nothing), `-y/--yes` (bypass the confirmation), `--deathstar` (confirm_mode on:
execute immediately), `--jarjar trash|obliterate` (deletion mode). Read-only
commands support `--json`.

**The confirmation gate in the CLI:** by default `clean`/`ashen`/`nuke`/etc. show
the colour-graded gate and require a `[y/N]` confirm (defaulting to No). Scope
purges (`empty-dirs`/`broken-symlinks`) always confirm the named scope
independent of `confirm_mode`. `dns-flush`/`docker-prune`/`maintain` acquire root
or wire their executor only after the gate confirms.

---

## 6. The TUI (`internal/tui`)

Bubble Tea (Elm architecture): `model` holds screen state; `Update` handles
messages; `View` renders. Screens: a **menu** (Clean / Restore / Get Coffee /
Audit / Doctor / Quit), the **get-coffee dashboard**, result and modal screens.

**The dashboard** (`dashboard.go`) renders from the latest `SystemSnapshot`:
tiles for CPU, MEMORY, GPU, THERMAL, DISK I/O, DISK `<fullest volume>`, BATTERY,
POWER — each with a live value + sparkline/gauge and a **health badge** (🟢 live /
⚠ stale / ⚠ N/A) so dead data is never shown as live. Below: an experimental NET
widget (top per-process talkers from the supervised nettop stream, labelled
experimental, honest warming/stalled state) and a top-processes table.

**Keys:** `↑/↓` select · `k` kill (gated through the same op pipeline) · `e`
elevate (starts the supervised powermetrics stream) · `p` pause · `esc` back.

`components/` holds the sparkline/gauge renderers (block-element today;
braille/ntcharts is backlog). `theme/` holds the colour palette + NO_COLOR
handling.

---

## 7. Data assets (`assets/`, `go:embed`)

| File | Contents |
| --- | --- |
| `safe_cache.json` | The curated safe-cache allowlist — **12 targets**: npm, yarn, pnpm, go-mod, pip, Xcode DerivedData, CocoaPods, Carthage, SwiftPM, Gradle, Homebrew (all `opt_in:false`), and iOS DeviceSupport (`opt_in:true`, with a rebuild-cost warning). Each carries a `rebuild_cost` tag (cheap/moderate/expensive). |
| `oui.csv` | ~76-row curated MAC-vendor subset (Apple/Intel/common router+IoT) for the LAN scanner; unknown OUIs render "unknown vendor". |
| `sensors.json` | board-ID → sensor-key map for IOKit thermals (carry-forward seam). |

The safe-cache manifest is the **only** thing `clean` will ever touch — nothing
outside the allowlist, and the deny-floor can't be bypassed.

---

## 8. Testing

- **170 test functions** across 24 test packages; **all green** under
  `go test -race ./...` (≈13 s cold, ms cached).
- Conventions: every test runs under `-race`; deletion/trash tests use only
  `t.TempDir()`; engines that scrape tools are tested with **injected fake
  runners + captured fixtures** (lsof field output, nettop two-sample frames,
  `log show` JSON, git porcelain, docker `system df`); no real destructive or
  privileged command is ever run in a test.

**Per-package test counts (highest first):**

| Package | Tests | Package | Tests |
| --- | --- | --- | --- |
| `pkg/operation` | 31 | `internal/orphans` | 7 |
| `internal/net` | 14 | `internal/restore` | 7 |
| `internal/tui` | 14 | `pkg/plist` | 7 |
| `internal/telemetry` | 13 | `internal/purge` | 6 |
| `internal/audit` | 11 | `pkg/safety/floor` | 6 |
| `internal/dedup` | 9 | `pkg/privilege` | 4 |
| `internal/docker` | 7 | `pkg/plan` / `trash` / `syscmd` | 4 each |
| `internal/scheduler` | 4 | `pkg/history` / `scanner` / `components` | 3 each |
| `internal/maintain` / `uninstaller` / `safety` / `fsdelete` | 2–3 | | |

> **Note:** `internal/cli`, `internal/tui/theme`, `internal/doctor`,
> `internal/procadmin`, `pkg/procctl`, `pkg/safety/identity`, and
> `pkg/safety/firmlink` have no dedicated `_test.go` files yet — they are
> exercised indirectly (CLI via the engines, identity/firmlink via the op and
> floor tests). Direct unit tests for these are the most obvious testing backlog.

**How to run:** `go build ./...` · `go vet ./...` · `go test -race ./...`.

---

## 9. Phase-by-phase status

| Phase | Scope | Status |
| --- | --- | --- |
| **0** | Safety spine proven first (operation/plan/floor/fsdelete/identity/trash/syscmd/history) | ✅ Done |
| **1** | `clean` (safe-cache) + `restore` + `get-coffee` dashboard + gated process kill | ✅ Done |
| **2** | `ashen` (dedup) + `nuke` (uninstaller) + `orphans` + restore hardening | ✅ Done |
| **3** | Read-only inspection: `net` (sockets/ports/LAN/bw) + `audit` (posture/auth-log); multi-volume disk + live experimental NET widget | ✅ Done |
| **4** | Power features + scheduling — see breakdown below | ✅ In-scope done |

**Phase 4 increments:**

| § | Feature | Status |
| --- | --- | --- |
| 4.1 | `suns maintain` — git GC (`RepoMaintenanceOp`, `git` pinned) | ✅ |
| 4.2 | `suns docker-prune` — container nuke (`ContainerPruneOp` + `ContainerPruner` seam) | ✅ |
| 4.3 | `suns dns-flush` — DNS incinerator (`DNSFlushOp`, root) | ✅ |
| 4.4 | `suns clean empty-dirs` — empty-directory purger | ✅ |
| 4.5 | `suns scan` + `clean broken-symlinks` — broken-symlink auditor | ✅ |
| 4.6 | `suns schedule` + `clean --scheduled` — scheduled burn daemon | ✅ |
| 4.7 | `suns lang-strip` — localization stripper (quarantined, ships last) | ⏸ Deferred |
| 4.8 | Release pipeline — universal binary, signing, notarization, `.pkg`, Homebrew | ⏸ Deferred |

§4.7/§4.8 are deferred by decision: they need Apple Developer credentials +
on-device signing/notarization. They are the natural next pass.

---

## 10. Carry-forward seams & backlog

**On-device seams** (built correct, degrade gracefully, finish opportunistically —
they need real hardware/root/Docker a headless test can't exercise):

| Item | Where | What remains |
| --- | --- | --- |
| Native Cgo Trash | `pkg/trash/native.go` | `NSFileManager trashItemAtURL:` ahead of the pure-Go fallback |
| Killable volume probe | `pkg/trash/probe.go` | Hang-detection helper subprocess + integration test |
| IOKit thermals (Cgo) | `internal/telemetry/iokit.go` | Bounded IOKit reads using `sensors.json` |
| Live powermetrics | `internal/tui/powerlaunch.go` | Validate the `sudo -n powermetrics` stream end-to-end |
| Battery extras | `internal/telemetry/battery.go` | wattage / cycle / health via ioreg |
| Docker prune execute | `internal/docker` | Validate the real prune on a machine with Docker |
| launchd load | `internal/scheduler` | Validate `bootstrap`/`bootout` on-device |

**Cross-cutting backlog** (master §5): `suns config get|set|show`; history
size-cap rotation + gzip archives; onboarding/FDA panel; braille charts
(ntcharts) with block fallback; harmonica motion; bubblezone mouse; truecolor
detection; direct unit tests for `cli`/`doctor`/`identity`/`firmlink`.

---

## 11. How to build, run, and use

**Build / verify:**
```
go build ./...          # compile everything
go vet ./...            # static checks
go test -race ./...     # full suite under the race detector
go run ./cmd/suns ...   # run without installing
```

**Safe ways to explore (read-only, no changes):**
```
suns doctor                         # environment self-check
suns audit                          # security posture (add --json)
suns net                            # who owns which socket; exposed ports
suns net lan                        # LAN devices (passive)
suns net bw                         # bandwidth (per-interface + experimental)
suns scan ~/Projects                # broken symlinks + empty dirs, no deletion
suns clean --dry-run                # preview the cache plan, change nothing
suns ashen --dry-run ~/Downloads    # preview duplicates
suns maintain --dry-run ~/code      # preview git-gc candidates
suns docker-prune --dry-run         # detect + preview (no-op if Docker absent)
suns get-coffee                     # live dashboard (press e to elevate, q to quit)
```

**Destructive actions** always preview a colour-graded gate and ask before
acting (unless `-y`/`--deathstar`). Trash is the default everywhere; permanent
deletion (`--jarjar obliterate` / `ashen --deathstar`) is an explicit opt-in.
`restore` undoes any trashed FileDelete. For an interactive session run `suns`
with no arguments.

**Privileged actions** (`dns-flush`, `audit logs`, `nuke` of `.pkg` daemons, the
scheduled agent) ask for an admin password once, at execution, through the single
chokepoint.

---

## 12. Conventions for contributors

- **New destructive action?** Model it as an `Operation` value type in
  `pkg/operation` (pure fields), reach the world through an injection seam, and
  route it through `plan.New().Seal()` → `safety.BuildGroups/Render` →
  `safety.Execute`. Re-verify the target in `ValidateAtExec` (TOCTOU). Record a
  typed `HistoryEntry`.
- **New external tool?** Pin it in `pkg/syscmd` (`productionAllowlist`) + add it
  to `internal/doctor`'s `pinnedTools`. Write a parsing contract (§13.1): expected
  format, defensive field parsing, degrade to empty/unknown on the unexpected,
  unit-tested against a captured fixture via an injected runner.
- **Package docs** live in `doc.go` (avoid the duplicate-package-comment vet
  error). Match the surrounding comment density and idiom.
- **Tests** run under `-race`; deletion tests use only `t.TempDir()`; never run a
  real destructive or privileged command in a test.
- **Read the source of truth:** `Docs/SunsMasterTD.md` (the v6 master design) and
  `Docs/Phases.md` (the running completion log).

---

## 13. Current working state

- Branch `main`, latest commit `fa81f5f Phase 4: Maintenance`; working tree clean
  at the time of writing.
- All of Phases 0–4 (in-scope) implemented, building, vetting, and passing
  `go test -race ./...`.
- Immediate next work: the deferred `lang-strip` (§4.7) and the release pipeline
  (§4.8), plus the cross-cutting backlog in §10. `lang-strip` is the most
  dangerous feature (deleting `.lproj` invalidates code signatures) and is
  designed to ship last, opt-in/per-app, with a post-strip `codesign --verify`.
```
