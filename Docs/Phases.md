# Suns — Remaining Work & Phase Plan

**Companion to `Docs/SunsMasterTD.md` (the v6 master technical document).**
This file tracks **what is left to build**, in implementation detail, so the
project can be finished without re-deriving decisions. Section references
(§N) point at the master document, which remains the authoritative spec.

> Conventions used here
> - **Op kinds** are the typed `operation.Operation` kinds (§4.2). Constants
>   already exist in `pkg/operation/kind.go`: `FileDelete`, `ProcessKill`,
>   `CacheReset`, `ServiceUnload`, `RepoMaintenance`, `ContainerPrune`,
>   `ReceiptForget`, `DNSFlush`. Kinds without a concrete value-type yet are
>   flagged **“needs concrete op.”**
> - **Reversibility**: 🟢 Reversible · 🟡 Recoverable · 🔴 Irreversible (§4.2).
> - Every destructive feature MUST: emit typed ops into a **value-sealed Plan**
>   (`plan.Seal`, §4.5), pass the **gate** (`safety.BuildGroups`/`Render`, §4.4),
>   be **revalidated at exec** (`safety.Revalidate`, §4.7), respect the
>   **deny floor** (`pkg/safety/floor`, §5.3, unbypassable), and write a typed
>   **history** record (`pkg/history`, §4.10).
> - Every scraped external tool goes through hardened **`pkg/syscmd`** (pinned
>   path, `LC_ALL=C`, no shell, allowlist, bounded output, timeout, §6.3) and
>   must gain a **parsing contract** (§13.1).

---

## 0. Status snapshot — what is already DONE

These are complete, building, and unit-tested under `-race` (do not redo):

**Phase 0 — the safety spine (`pkg/`):**
- `operation` — `Operation` interface + value-type kinds + tiered `Identity`.
  Concrete ops implemented: **`FileDeleteOp`**, **`ProcessKillOp`**.
- `plan` — value-sealing `Seal()` (pointer-free deep copy; fails loud on
  pointer-receiver ops); time-sortable IDs.
- `safety/floor` — never-touch deny floor (plan/exec/descent; unbypassable).
- `safety/firmlink` — OS firmlink boundary detection.
- `safety/identity` — tiered file identity (size+mtime+inode vs SHA-256) +
  `VerifyContent` (restore, ignores dev/inode).
- `safety/fsdelete` — fd-anchored obliterate (`openat`/`unlinkat`, `O_NOFOLLOW`,
  post-order; never escapes the subtree).
- `safety` — `gate` (mode-aware per-kind grouping + reversibility badge) and
  `revalidate`/`Execute` orchestration.
- `trash` — bounded-and-degrading: pure-Go fallback (rename / copy-then-unlink,
  never permanent-delete), semaphore + circuit breaker, `MovePath`.
- `history` — `0600` JSONL + export-only redaction.
- `syscmd` — hardened executor + allowlist.
- `privilege` — per-workflow `sudo` chokepoint (Prompter seam, ticket logic,
  allowlisted actions, canceled/expired states; no persistent root).
- `procctl` — process introspection + atomic validate-and-signal.

**Phase 1 — the trustworthy v1:**
- `internal/config` — Viper read-once → `SessionState` (flags > file > defaults).
- `internal/scanner` — safe-cache manifest load + concurrent discovery → ops.
- `internal/restore` — identity-checked undo, fail-loud collisions.
- `internal/doctor` — environment/permission/tool self-check.
- `internal/tui` — Bubble Tea shell: theme + mode recoloring, three-zone layout,
  help overlay, frozen-plan confirmation modal, menu router.
- `internal/telemetry` — supervised poller (cheap gopsutil sources, atomic
  snapshots, ring buffers), **battery via pmset (live)**, **powermetrics
  decoder + supervised `PowerSource`** (decode/stream tested), per-source health.
- `internal/tui` get-coffee dashboard — 9-widget layout, block-element
  sparklines/gauges, staleness badges, process selection.
- `internal/procadmin` + hidden `__killproc` — privilege-capable killer.
- CLI commands: `doctor`, `version`, `clean`, `restore`, `get-coffee`, `ps`.

**Phase 2 — earned breadth (in progress):**
- `internal/dedup` — APFS-conservative duplicate finder backing **`suns ashen`**
  (the `dedup` command, renamed; `dedup` kept as a hidden alias). Three-pass
  detection (size → 4 KB head hash → SHA-256), hardlink collapse, bundle-atomic,
  no-follow symlinks, floor-guarded, keeper heuristic, cosmetic-xattr note, and
  the APFS correction (shared-block files are **never** excluded; a conservative
  space caveat is surfaced instead). Emits `FileDeleteOp`s into a sealed plan
  through the existing gate/history/restore spine. Unit-tested under `-race`
  (incl. real `clonefile`/`setxattr` fixtures). **2.3 below is now DONE.**
- New concrete ops **`ServiceUnloadOp`** (🟡, `launchctl bootout` via the
  `operation.SystemRunner` seam) and **`ReceiptForgetOp`** (🔴, `pkgutil
  --forget`, privileged). Both pure value types; `HistoryEntry` gained
  domain/label/plist/package-id fields. Unit-tested with a fake runner.
- `pkg/plist` implemented: binary-safe `Decode`/`ReadInfo`/`BundleIdentifier`
  (`howett.net/plist`), tested against binary + XML + malformed fixtures.
- `internal/uninstaller` + **`suns nuke <app>`**: bundle-ID tracing, ~/Library +
  launch-agent tracing, `pkgutil` harvest with the **`--file-info`
  shared-dependency guard** (files claimed by >1 package are retained, not
  deleted), ordered plan (ServiceUnload → FileDelete → ReceiptForget), explicit
  UI scope bounds. Privileged execution wired through the chokepoint in
  `root.go` (`elevatingRunner`). Engine unit-tested (guard + ordering) under
  `-race`. **2.1 and 2.2 below are now DONE.**
- `internal/orphans` + **`suns orphans`**: orphaned launch-agent purge. Scans the
  user + system launchd dirs, resolves each job's executable (env-launchers
  unwrapped; shell wrappers and relative/PATH programs declined as
  unresolvable), skips Apple-managed jobs, and flags only when the resolved
  absolute executable is genuinely absent. Emits bootout-before-plist-delete
  ops; conservative bounds stated in the UI. Unit-tested under `-race`.
- Restore hardening (§2.4): confirmed `restore` is FileDelete-only (the Phase 2
  op kinds are excluded from `Candidates`) and added a real >50 MB sparse
  large-object (TierLarge size+mtime) round-trip test. **2.4 below is now DONE.**

**Phase 3 — read-only inspection (complete):**
- `internal/audit` + **`suns audit`** (alias `secure`): SIP/Gatekeeper/FileVault
  posture from `csrutil`/`spctl`/`fdesetup` + XProtect version, per-finding +
  overall severity, `--json`, parsing contracts that degrade to "unknown".
  Unit-tested under `-race`. **§3.2a is now DONE.**
- `internal/net` + **`suns net`**: per-app socket mapper + open-port auditor.
  Parses `lsof -F` field output, classifies listening ports by reachability
  (wildcard/loopback/specific), bounded cached reverse-DNS, `--json`/`--no-dns`.
  Unit-tested under `-race`. **§3.1a and §3.1b are now DONE.**
- **`suns audit logs`** (`internal/audit.AuthLog`): sudo/auth timeline from the
  unified log (root via chokepoint), success/failure/denied classification,
  per-user rapid-failure burst detection, `--json`/`--since`. Unit-tested under
  `-race`. **§3.2b is now DONE.**
- **`suns net lan`** (`internal/net.LANScan`): passive ARP-cache LAN discovery
  with curated-subset OUI vendor + reverse-DNS hostname, and a warn-gated active
  port probe (`--probe`). `arp` allowlisted. Unit-tested under `-race`. **§3.1c is
  now DONE.**
- **`suns net bw`** (`internal/net.Bandwidth`): per-interface throughput from the
  kernel interface counters (reliable core) plus EXPERIMENTAL per-process tx/rx by
  differencing a two-sample `nettop -P -x` capture; degrades to "unavailable"
  cleanly when nettop can't be read, `--json`/`--interval`/`--top`. Unit-tested
  under `-race`. **§3.1d is now DONE.**
- **Multi-volume disk** (`internal/telemetry/poller.go` `readVolumes` + dashboard):
  the poller now enumerates every mounted physical (`/dev`-backed) volume; the
  get-coffee DISK tile shows the fullest volume by used-%. **§3.3 multi-volume is
  now DONE.**
- **Per-PID net in the live dashboard** (`internal/telemetry/netsource.go` +
  dashboard `dashNet`): a supervised, unprivileged `nettop -P -x -l 0` stream
  diffs consecutive frames into top-N per-process tx/rx, merged into the snapshot
  and rendered in the NET widget with an EXPERIMENTAL label and honest health.
  **§3.3 per-PID net is now DONE.** Phase 3 is feature-complete.

---

## 1. Carry-forward — staged seams to finish ON-DEVICE

These were implemented behind honest seams (build-correct, degrade gracefully)
because they require a real device, root, or hardware Suns can’t exercise in a
headless test. They are **not** new phases; finish them opportunistically.

| Item | Where | What remains | Spec |
| ---- | ----- | ------------ | ---- |
| Native Trash (Cgo) | `pkg/trash/native.go` | Implement `nativeTrash` via Cgo `NSFileManager trashItemAtURL:` with a per-call timeout; enable it ahead of the pure-Go fallback. | §2.4, §4.4 |
| Killable volume probe | `pkg/trash/probe.go` | Implement the short-lived **helper subprocess** that runs `statfs`/access and is killable on hang; gate `tryNative` on it. Add the hung-volume integration test. | §4.4, §15 |
| IOKit thermals (Cgo) | `internal/telemetry/iokit.go` | Implement bounded IOKit `IOHIDEventSystemClient` reads (+ Intel SMC) using `assets/sensors.json`; per-call timeout in a bounded goroutine; supervise. Populate `sensors.json` board-ID → key map. | §4.4, §5.2, §12.9 |
| Live powermetrics | `internal/tui/powerlaunch.go` | Validate the `sudo -n powermetrics` stream end-to-end; confirm the field mapping in `DecodePowerMetrics` against captured output; refine samplers. | §7.2, §7.3 |
| Battery extras | `internal/telemetry/battery.go` | Add wattage / cycle count / health via `ioreg AppleSmartBattery` (pmset gives %/state/time only). | §11 widget 6 |
| ULID for plan IDs | `pkg/plan/plan.go` | Optionally swap the home-grown time-sortable ID for a canonical ULID lib. | §4.5 |

---

## 2. PHASE 2 — Uninstaller & Dedup (“earned breadth”)

Goal: the two highest-correctness-surface destructive features, each explicitly
bounded and UI-honest. Build on the existing spine; add two op kinds.

### 2.1 `suns nuke <app>` — Precision Uninstaller ✅ DONE
- **Destructive · gated.** Op kinds: **`FileDelete`** + **`ServiceUnload`** +
  **`ReceiptForget`** (all three concrete ops now exist). Reversibility:
  files 🟢/🔴 by mode; `ServiceUnload` 🟡 (re-loadable); `ReceiptForget` 🔴.
- **Engine:** `internal/uninstaller` (implemented; `Plan` returns an ordered op
  set + retained shared deps + scope bounds). CLI in `internal/cli/nuke.go`.
- Carry-forward: privileged removal of system-location payload files (under
  `/Library`, `/usr/local`) currently executes as the invoking user via Trash;
  elevated payload deletion (beyond the already-privileged `ReceiptForget` and
  system `ServiceUnload`) is a future on-device enhancement.
- **Mechanism (the corrected teardown order — §12.15-uninstaller):**
  1. Find the app in `/Applications`, `~/Applications`, or subfolders.
  2. Read `Info.plist` **binary-safe** (`howett.net/plist`, already a dep) for
     `CFBundleIdentifier`.
  3. Trace `~/Library` support files by **bundle-ID match** — a heuristic,
     **best-effort** (apps don’t always name support files by bundle ID).
  4. For `.pkg`-installed apps, **in this exact order**:
     a. `pkgutil --pkgs` → find receipts.
     b. **`pkgutil --files <id>`** → harvest the payload (because `--forget`
        removes only the *receipt*, orphaning daemons/binaries otherwise).
     c. **`pkgutil --file-info <path>`** for each harvested absolute path; if a
        file is **claimed by more than one installed package ID**, **EXCLUDE it**
        and record it as `retained_shared_dependency` in history — this is the
        guard that prevents bricking unrelated apps by deleting a shared
        `dylib`/Audio Unit.
     d. Generate `FileDelete` + `ServiceUnload` ops for the **sole-owned**
        payload only.
     e. **Only then** run `pkgutil --forget` (the `ReceiptForget` op).
  5. Also surface `~/Library` remnants and related launch agents.
- **Privilege:** `pkgutil --forget` / `--file-info` and removing files under
  `/Library` need the **chokepoint** (§6.4). Discovery stays unprivileged.
- **Explicit bounds (UI must state, §10.7):** does **not** comprehensively chase
  Mac App Store containers, app-group containers, shared frameworks used by other
  apps, login items, helper tools, or all LaunchServices registrations. UI never
  says “complete uninstall.”
- **Packages/work:** `internal/uninstaller` engine; concrete `ServiceUnloadOp`
  and `ReceiptForgetOp` in `pkg/operation`; `pkgutil`/`launchctl` parsing
  contracts in `syscmd`; add `pkgutil`/`launchctl` to privileged-action allowlist
  (already in `privilege.privilegedActions` for `launchctl`/`pkgutil`).
- **Tests (§15):** payload file claimed by two packages → excluded + recorded;
  sole-owned → deleted; binary + XML + malformed plist fuzz; harvest-then-forget
  ordering; bounded-scope assertions.
- **Done when:** a `.app` and a `.pkg`-installed app are previewed with a sealed
  plan that excludes shared deps, executes in order, and records typed history;
  shared-dependency guard test passes.

### 2.2 `ServiceUnload` op — Orphaned Launch Agent Purge ✅ DONE
- **Destructive · gated.** 🟡 best-effort. Op: `ServiceUnload` + `FileDelete`.
- `ServiceUnloadOp` exists, is used by `nuke`, and now backs a standalone
  **`suns orphans`** sweep (`internal/orphans`): resolves wrappers/env-launchers,
  declines shell-scripted/relative programs as unresolvable, skips Apple-managed
  jobs, requires the resolved executable to be genuinely absent, and runs
  bootout before the plist `FileDelete`.
- **Mechanism (§12.3):** scan `~/Library/LaunchAgents`, `/Library/LaunchAgents`,
  `/Library/LaunchDaemons`; parse each plist **binary-safe** for `Program` /
  `ProgramArguments`; **resolve wrappers, shell scripts, relative program paths,
  and env-launchers** before flagging orphaned; **skip Apple-managed and
  app-regenerated** jobs; require the *resolved* executable to be genuinely
  absent. Run `launchctl bootout` **before** removing the plist. Identity at exec
  = domain + label + source plist path.
- **Explicit bounds:** documents that launchd domains, disabled states, and
  update-time regeneration can race bootout-then-remove; reports, not guarantees.

### 2.3 `suns ashen [path...]` — Hash-Based Duplicate Immolator ✅ DONE
- **Shipped as `suns ashen`** (renamed from `dedup`; `dedup` is a hidden alias).
- **Destructive · gated.** Op: **`FileDelete`** · 🟢 trash / 🔴 obliterate.
- **Engine:** `internal/dedup` (implemented; `Find` returns a `Report` of groups
  + `FileDeleteOp`s). CLI in `internal/cli/ashen.go`, registered in `root.go`.
- All mechanism/correctness points below are implemented and unit-tested under
  `-race`. Remaining for the TUI phase: interactive per-group keeper adjustment
  (the CLI shows the keep/burn split and confirms at the gate).
- **Mechanism (§12.1):** three passes — (1) group by exact byte size; (2) hash
  **first 4 KB** as a cheap discriminator; (3) full **SHA-256** only on survivors.
  Delete identity is **tiered** (§4.7).
- **APFS correctness (the key correction):** block sharing via reference counts
  does **not** identify clone lineage. Therefore **do NOT exclude** shared-block
  files from dedup (that would skip real duplicates that merely share blocks with
  unrelated objects, e.g. Time Machine snapshots). Detect sharing only to
  **estimate reclaimable space conservatively** and **flag** that freed space for
  clone-participating files may be lower than nominal.
- **Hardlinks** (same inode) = the same file → never offered as deletable.
- **Bundles** (`.app`, `.rtfd`, `.photoslibrary`) treated **atomically**; interior
  files never deduped.
- **Xattr policy:** compare **data forks**, ignoring cosmetic xattrs (quarantine,
  Finder tags, where-from); surface a note when ignored xattrs differ.
- **Keeper heuristic:** prefer a non-cache, non-trash, user-document location;
  deprioritize `Caches`/`Downloads`/temp; never auto-select across bundles;
  operator reviews/adjusts before the gate.
- **Tests (§15):** hardlink, clone/shared-block (assert **not** wrongly excluded),
  bundle, xattr-normalization fixtures.

### 2.4 Restore hardening ✅ DONE
- No code change needed: `restore` is **FileDelete-only** by construction —
  `Candidates` admits only `KindFileDelete` + Reversible + trashed, so the Phase 2
  op kinds (`ServiceUnload`, `ReceiptForget`) are excluded (regression-tested).
- Added a real >50 MB sparse fixture exercising the large-object
  (size+mtime+inode, TierLarge) restore path end-to-end.

**Phase 2 dependencies:** none new beyond `howett.net/plist` (present).
**Phase 2 definition of done:** ✅ `ashen` (dedup) + `nuke` (uninstaller) +
`orphans` (launch-agent purge) shippable behind the gate with the
shared-dependency guard and APFS-conservative dedup; restore hardened; all
engine §15 tests green under `-race`. **PHASE 2 COMPLETE.**

---

## 3. PHASE 3 — Inspection Suites (Network & Security Audit)

Goal: read-only inspection. **No gate, no prompts** (except the auth-log query’s
root). Mature the parsing-contract layer (§13.1). All support `--json`.

### 3.1 `suns net` — Network suite (`internal/net`)
- **3.1a Per-App Socket Mapper (§12.5)** ✅ DONE — `net.Sockets` parses
  `lsof -i -n -P -F pcnPtT` (field mode, robust contract in `lsof.go`) → PID →
  app → local/remote addr+port table, with **async cached, bounded reverse-DNS**.
  CLI `suns net`, read-only, `--json`, `--no-dns`.
- **3.1b Open Port Auditor (§12.12)** ✅ DONE — listening sockets classified by
  scope (wildcard `0.0.0.0`/`::` → exposed; loopback; specific), severity-marked
  in the listing. Parsers + scope classification unit-tested under `-race` with an
  lsof fixture (wildcard/loopback/IPv6/established/UDP).
- **3.1c LAN Scan (§12.6)** ✅ DONE — `net.LANScan` parses `arp -a -n` (IP + MAC +
  interface, skipping incomplete/broadcast/multicast), adds a **curated-subset
  OUI** vendor (`assets/oui.csv`, unknown → "unknown vendor") and a bounded
  cached reverse-DNS hostname. Optional active TCP port probe is **gated behind a
  warn-and-confirm** (`--probe`, ack with `--yes`). CLI `suns net lan`, passive by
  default, `--json`/`--no-dns`. `arp` added to the syscmd allowlist + doctor.
  Parser/OUI/MAC-normalization unit-tested under `-race`. (Explicit mDNS via
  `dns-sd` is a future refinement; reverse DNS already resolves `.local` on macOS.)
- **3.1d Bandwidth Hog Detector (§12.7) — EXPERIMENTAL.** ✅ DONE —
  `internal/net.Bandwidth`. Interface totals are the **reliable core**: gopsutil
  per-interface byte counters differenced over one short window. Per-process tx/rx
  is **EXPERIMENTAL**: a two-sample `nettop -P -x -L 2` capture differenced by
  PID using the rows' own timestamps; `parseNettop` finds the `bytes_in`/
  `bytes_out` columns from the header and drops any process seen only once. It
  degrades to `ProcessesAvailable:false` rather than show wrong numbers (§7.5).
  The interface sample and nettop run **concurrently** so the whole call takes ~one
  window. CLI `suns net bw` (alias `bandwidth`), `--json`/`--interval`/`--top`.
  `parseNettop`/`splitNamePID` unit-tested under `-race` with a two-sample fixture.

### 3.2 `suns audit` / `suns secure` — Security posture (`internal/audit`)
- **3.2a SIP / Gatekeeper / FileVault (§12.13)** ✅ DONE — `internal/audit.Posture`
  parses `csrutil status`, `spctl --status`, `fdesetup status` into one posture
  view with per-finding + overall severity, plus the XProtect version (read
  binary-safe via `pkg/plist`). CLI `suns audit` (alias `secure`), read-only,
  `--json`. Parsing contracts in `internal/audit/parse.go` degrade to "unknown"
  on unexpected output; parsers + engine unit-tested under `-race`. (secure-boot
  via `bputil` needs root — deferred to the §3.2b root slice.)
- **3.2b Sudo / Auth-Log Analyzer (§12.14) — root.** ✅ DONE — `audit.AuthLog`
  queries the **unified log** (`log show --style json --predicate 'process ==
  "sudo"' --last 1d --info`) — **not** the deprecated `/var/log/system.log` —
  classifies each entry (success / failure / denied), extracts the actor, and
  detects per-user **rapid-failure bursts** (≥3 within 60s). CLI `suns audit
  logs`, `--json`, `--since`; root acquired once via the **chokepoint** (`log` is
  allowlisted). Parser + burst detection unit-tested under `-race` with a log JSON
  fixture. (Broadening the predicate to `Authorization`/`opendirectoryd` and
  secure-boot via `bputil` are future refinements.)

### 3.3 Telemetry: finish per-PID net + multi-volume
- **Multi-volume disk** ✅ DONE — `internal/telemetry/poller.go` `readVolumes`
  enumerates every mounted physical (`/dev`-backed) volume via
  `disk.Partitions`, dedupes mountpoints, sorts root-first, and falls back to `/`.
  The get-coffee DISK tile (`internal/tui/dashboard.go`) now shows the **fullest**
  volume (highest used-%), labelled with its short mountpoint.
- **Per-PID net in the live dashboard** ✅ DONE (EXPERIMENTAL) —
  `internal/telemetry/netsource.go` `NetSource` consumes a long-lived,
  unprivileged `nettop -P -x -l 0` stream under the same supervisor/watchdog as
  powermetrics (launch → diff consecutive frames → on stall/EOF relaunch with
  backoff). The poller merges its latest top-N into the snapshot; the get-coffee
  NET widget (`internal/tui/dashboard.go` `dashNet`) lists the top talkers with an
  explicit "experimental" label, and shows the honest health reason (warming /
  stalled) rather than fake zeros until two frames are differenced. nettop needs
  no elevation, so it starts as soon as the dashboard opens. Streaming parser +
  frame-diff + stall behaviour unit-tested under `-race` with fixtures.

**Phase 3 definition of done:** `net` + `audit` read-only suites with `--json`,
parsing contracts with version detection + graceful degradation (§13.1),
fixtures captured per macOS version, all parsers unit-tested.

---

## 4. PHASE 4 — Power Features, Scheduling & Release

Goal: the remaining destructive actions (each modeled as a typed op), unattended
scheduling, the quarantined stripper, and the **release pipeline as a
first-class task**.

### 4.1 `suns maintain` — Git Repository GC (§12.17)
- **Destructive · gated.** Op: **`RepoMaintenance`** (needs concrete op) · 🟡.
- **Correction:** `git gc --aggressive --prune=now` across all repos drops
  recoverable work (dropped stashes, reset commits, abandoned branches). Default
  is **plain `git gc`** with the normal **two-week prune window**; `--aggressive`
  + immediate prune are explicit opt-ins with a **per-repo warning**.
- Discover repos; **list each with estimated savings before acting**; **skip
  repos with uncommitted changes or in-progress merge/rebase**; gate the batch.
- Identity at exec = re-confirm repo cleanliness immediately before acting.

### 4.2 Docker Environment Nuke (§12.15)
- **Destructive · gated.** Op: **`ContainerPrune`** (needs concrete op) · 🔴.
- **Correction:** Docker runs in a Linux VM on macOS; the default Unix socket
  often doesn’t exist. **Probe** `~/.docker/run/docker.sock`,
  `~/.colima/default/docker.sock`, and **OrbStack**’s endpoint before
  initializing the SDK client (fall back to the CLI). Detect daemon-running first;
  clear no-op message if Docker absent. Preview reclaimable space, then
  `system prune -a --volumes` **only after the gate confirms**.

### 4.3 DNS Cache Incinerator (§12.11)
- **Destructive(action) · gated.** Op: **`CacheReset`** (kind exists; needs
  concrete op) · 🔴 (no undo) · **root**. `optimizer` stub exists.
- Run `dscacheutil -flushcache; killall -HUP mDNSResponder` via the **chokepoint**
  (`dscacheutil`/`killall` already in the privileged allowlist). The **Jarjar
  deletion axis is inert** for it (it’s a reset, not a deletion); carries the 🔐
  root badge.

### 4.4 Empty Directory Purger (§12.19)
- **Destructive · gated.** Op: `FileDelete` · 🟢. **Post-order** (bottom-up)
  traversal so directories emptied by removing children are caught in one pass.
  **Always confirm the target root scope first** (“Purge empty dirs under
  `<path>`? [y/N]”) independent of `confirm_mode`. Decide consistently whether a
  dir containing only `.DS_Store` counts as empty.

### 4.5 Broken Symlink Auditor (§12.18)
- Read-only audit half lives in `suns scan`; destroy half in `suns clean`
  (`FileDelete` · 🟢) using the **fd-anchored, no-follow deleter** (§4.6).

### 4.6 `suns schedule` — Scheduled Burn Daemon (§12.20)
- **Destructive(authoring) · constrained.** Engine: `internal/scheduler` (stub).
- **Correction:** user cron is deprecated → author a **`launchd` user
  LaunchAgent** running `suns clean --scheduled`.
- **Most-constrained mode:** locked to the **curated safe-target allowlist**;
  `deletion_mode` **forced to `trash`** (never obliterate unattended); interactive
  flags + config **ignored**; full history logging; note that the agent’s context
  must have Full Disk Access or it silently skips protected paths.
- **Failure semantics:** repeated permission denials / unavailable target / stale
  exclusions → write a `scheduled_run` history record with status
  `partial`/`failed`, **surfaced prominently** in CLI + TUI (never silent).
- **No-GUI-session behavior:** with no Aqua login session, still **force trash**
  (pure-Go `~/.Trash` fallback if the Cocoa API is unavailable headless, **never**
  permanent delete), queue a notification for next login, and **skip** any target
  whose trashability can’t be guaranteed rather than escalating to permanent
  deletion.
- Add `--scheduled` handling to the `clean` command (lock settings, ignore flags).

### 4.7 `suns lang-strip <app>` — Localization Stripper (§12.4) — QUARANTINED, SHIPS LAST
- **Destructive · gated.** Op: `FileDelete` · 🔴. **Most dangerous feature.**
- Deleting `.lproj` from a `.app` **invalidates the code signature**; on modern
  macOS this can make the app refuse to launch (“damaged”), trip Gatekeeper, and
  break Sparkle/delta updates. Therefore: **opt-in, per-app, strongest warning in
  the app, never part of bulk `clean`**, and **re-run `codesign --verify`
  afterward** and warn loudly if the bundle is now broken. Recommended default:
  leave bundles intact.

### 4.8 Release Engineering (first-class — §5.4, §14)
- **Universal binary**: build `arm64` + `amd64`, `lipo` into one binary.
- **Cgo build** for the native Trash + IOKit paths (carry-forward §1).
- **Code signing** with Developer ID; **notarization**; **hardened runtime**.
- Signed **`.pkg`** installer + **Homebrew tap**.
- `SMAppService` registration **only if** an app-bundle distribution is pursued
  (optional future; the XPC helper slots behind the same chokepoint — §6.2).

**Phase 4 definition of done:** maintain / docker-prune / dns-flush / empty-dir /
broken-symlink / schedule shipping behind the gate with their corrections;
`lang-strip` quarantined with codesign re-verify; signed+notarized universal
binary distributable via `.pkg` and Homebrew.

---

## 5. Cross-cutting backlog (do alongside the phases)

| Area | Work | Spec |
| ---- | ---- | ---- |
| Parsing contracts | For **every** scraped tool: expected format, version detection, graceful degradation (“telemetry unavailable on this OS version”), fixtures per macOS version. Prefer structured output (`-f plist`, `--style json`, `-json`). | §13.1 |
| `suns config (get\|set\|show)` | Meta command: read/write persisted config; `show` prints the effective merged config **and the source of each value**. | §9.3 |
| History rotation | Size-cap rotation; old logs gzip into dated archives. | §13.3 |
| `--json` everywhere | All read-only commands emit structured output. | §9.4 |
| Onboarding / FDA panel | TUI panel when limited-mode detected; opens the FDA settings pane via the `x-apple.systempreferences:` URL. | §5.1, §10.7 |
| Charts | Optional **ntcharts** Braille charts with the **Apple_Terminal block-element fallback** (currently block-only everywhere via `components.Sparkline`). | §2.2, §10.4 |
| Motion | **harmonica** spring easing for gauges + a brief “burn” animation, all behind `--reduce-motion`; ≤250 ms budget. | §10.5 |
| Mouse | **bubblezone** clickable tiles/menu items (keyboard stays primary). | §10.6 |
| Color detection | Truecolor vs 256 vs 16-color detection; refine `NO_COLOR` degradation. | §10.2 |
| Scriptability | `--dry-run` parity = dry-run plan equals the **planned** set; revalidation skips reported (not a parity violation). | §13.4 |

### Testing matrix still owed (§15)
- **Hung-volume** integration test for the native Trash + killable probe.
- **goleak** on all engines; race detector already standard.
- **Golden TUI tests** (`teatest`) for key views in both color modes + the
  Apple_Terminal fallback.
- **Plist fuzz** (binary/XML/malformed) once the uninstaller/launch-agent parsers
  land.
- **Privilege**: canceled/expired/partial-batch paths; privileged ProcessKill
  under elevation; assert **no persistent root process** after a workflow.
- **APFS dedup** fixtures (hardlink, clone/shared-block, bundle, xattr).
- CI: `go vet`, `staticcheck`, `golangci-lint`, `-race`.

---

## 6. New concrete operation kinds still to implement

All kinds have string constants in `pkg/operation/kind.go`; these need
value-type structs implementing `Operation` (value receivers, pointer-free, so
`plan.Seal` works — §4.5), each with `Describe`, `Reversibility`,
`ValidateAtPlan`, `ValidateAtExec` (returning a typed `Identity`), `Execute`,
`HistoryRecord`:

| Op | Phase | Reversibility | Identity at exec | Privilege |
| -- | ----- | ------------- | ---------------- | --------- |
| `ServiceUnloadOp` ✅ | 2 | 🟡 | domain + label + source plist path | chokepoint (`launchctl`) |
| `ReceiptForgetOp` ✅ | 2 | 🔴 | package ID still installed | chokepoint (`pkgutil`) |
| `RepoMaintenanceOp` | 4 | 🟡 | repo clean / no in-progress op | none |
| `ContainerPruneOp` | 4 | 🔴 | daemon reachable | none (Docker socket) |
| `CacheResetOp` (DNS) | 4 | 🔴 | n/a (deletion axis inert) | chokepoint (`dscacheutil`,`killall`) |

`FileDeleteOp`, `ProcessKillOp`, `ServiceUnloadOp`, and `ReceiptForgetOp` now
exist. Remaining: `RepoMaintenanceOp`, `ContainerPruneOp`, `CacheResetOp`.

---

## 7. Anticipated dependency additions

- Phase 3: none required beyond stdlib + `x/sys/unix` (already present) for
  sockets/`sysctl`; mDNS may use a small Bonjour/zeroconf lib or shell out to
  `dns-sd` (prefer the contract’d external tool to avoid a heavy dep).
- Phase 4: Docker via the official Go SDK **or** the `docker` CLI behind a
  contract (CLI fallback preferred for fewer deps); git via the `git` CLI behind
  a contract (no libgit2).
- TUI polish: `github.com/NimbleMarkets/ntcharts`,
  `github.com/charmbracelet/harmonica`, `github.com/lrstanley/bubblezone`
  (all named in §2.2).
- Keep the **dependency philosophy** (§2.6): stdlib + `x/sys` + Charm first; Cgo
  only where macOS forces it, always with a non-Cgo fallback.

---

## 8. Build order recap (§14)

1. **Phase 0 — DONE.** Dangerous core proven first.
2. **Phase 1 — DONE.** `clean` (safe-cache) + `restore` + `get-coffee` + gated
   process kill.
3. **Phase 2 — DONE.** Uninstaller (`nuke`) + APFS-conservative `ashen` (dedup) +
   orphaned launch-agent purge (`orphans`) + restore hardening.
4. **Phase 3 — DONE.** `net` (sockets/ports/LAN/bandwidth) + `audit`
   (posture/auth-log) read-only suites; parsing-contract layer matured; per-PID
   `nettop` wired into the live dashboard as a supervised experimental stream.
5. **Phase 4 — NEXT.** `maintain`, Docker prune, DNS flush, empty-dir + broken-symlink
   destroyers, `schedule` (launchd), quarantined `lang-strip` (last), and the
   universal-binary + codesign + notarize + Homebrew **release pipeline**.

Plus the **carry-forward seams** (§1) and the **cross-cutting backlog** (§5)
woven in where each becomes relevant.

---

*Keep this file in sync as phases land: move completed items from the phase
sections up into §0, and tick off carry-forward seams as they’re finished
on-device. The master document (`SunsMasterTD.md`) stays the source of truth for
the “why”; this file is the “what’s left.”*
