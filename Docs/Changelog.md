# Suns: changelog

Newest first. One entry per change that touched the code or `stats.md`. The status doc is `Docs/stats.md`.

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
