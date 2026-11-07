# Suns: changelog

Newest first. One entry per change that touched the code or `stats.md`. The status doc is `Docs/stats.md`.

## 2026-11-07: stats.md overhauled to the MakeStats template

- **What:** `Docs/stats.md` rebuilt into the 15 fixed sections, with every fact re-checked against the code. A
  session now gets the project, its hazards, the verify command and open work from the header and §0–§2. Open work,
  rulings and limitations now have stable IDs (O-1 to O-18, R-1 to R-4, L-1 to L-6).
- **Why:** owner request (`/makestats overhaul`); the old doc had no dates, stale commit state and no IDs.
- **Files:** `Docs/stats.md`, `Docs/Changelog.md` (new), `Docs/Legacy/stats-2026-11-07.md` (backup),
  `Docs/Legacy/stats-work-2026-11-07.md` (work record), `CLAUDE.md` (status-doc block), `.claude/rules/safety.md`
  (section reference); in Res: `Docs/WorkMap.md`, `Docs/stats.md` (Suns rows)
- **Verified:** `go build ./... && go vet ./... && go test -race -count=1 ./...` → all pass (172 test functions,
  24 packages with tests); no code changed; not checked on a device.
- **History moved:** the old §9 phase table, word for word, to "Older history" below.
- **Corrections found in the old doc:**
  - `ashen --deathstar` does not delete permanently; `--deathstar` only skips the gate. Only `--jarjar obliterate` or
    `deletion_mode: obliterate` deletes permanently.
  - The deny floor also protects the home directory root itself.
  - "Every external command goes through `syscmd`" and "the single place Suns elevates" have exceptions: the
    `nettop`/`powermetrics` streams and three sudo paths (stats.md §10.2).
  - `doctor` checks 16 tools, not all 18 pinned ones (`killall`, `mdutil` missing; L-5).
  - `oui.csv` has 56 vendor rows, not about 76 (that was the line count with comments).
  - 172 test functions and 36 packages, not 170 and 34 (the nettop fix added tests).
  - Packages without tests also include `internal/config`, `internal/optimizer`, `internal/tui/views`.
  - The deferred work was cited as "§4.7/§4.8", which in the old doc meant `maintain` and `docker`; it is
    `Docs/Phases.md` §4.7/§4.8.
  - New facts the old doc lacked: a scheduled clean without Full Disk Access records `ok` (L-2); the TUI Audit
    menu is a placeholder and Restore only lists (L-4).
- **Closed:** none · **stats.md:** all sections

---

## Older history

### From stats.md, moved 2026-11-07

#### Old `## 9. Phase-by-phase status` (stats.md lines 522–546, backup `Docs/Legacy/stats-2026-11-07.md`)

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
