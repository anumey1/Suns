# Suns: stats.md overhaul work file

Job: MakeStats Overhaul (Part D) of `Docs/stats.md`, started 2026-11-07. A session resuming this job reads this
file first and continues at the first open step. Ground rules: MakeStats A7 (stop after each step; no git writes;
never run `suns` except `--dry-run`/`--help`/`help`/`version`; change no code except stats-citing comment lines).

Project kind (B6): **Tool with destructive actions** — §1.2 says what must never be run; §4 opens with the safety
model.

## Tracker

| Step | State | Notes |
|---|---|---|
| Pre-D1 | Done | `git status --short Docs/ CLAUDE.md .claude/` clean; branch `main` (= WorkMap §3) |
| D1 Back up and inventory | Done | Backup `Docs/Legacy/stats-2026-11-07.md` (byte-identical, `cmp`) |
| D2 Map every old section | Done | Map, empty sections and number map below |
| D3 Move history to the changelog | Open | |
| D4 Rewrite into the template | Open | |
| D5 Fix every reference | Open | |
| D6 Checks | Open | |

## D1. Old doc inventory

783 lines, longest line 324 characters. Under 1,200, so a single-session rewrite is fine.

| Old section | Lines | Size |
|---|---|---|
| Header (title + "What this document is" banner) | 1–11 | 11 |
| `## 0. TL;DR — what Suns is and where it stands` | 12–41 | 30 |
| `## 1. Repository layout` | 42–94 | 53 |
| `## 2. Dependencies (go.mod)` | 95–115 | 21 |
| `## 3. The safety architecture (the heart of Suns)` (§3.1–§3.11) | 116–284 | 169 |
| `## 4. Engines (internal/*) — feature by feature` (§4.1–§4.13) | 285–408 | 124 |
| `## 5. The CLI (internal/cli) — every command` | 409–452 | 44 |
| `## 6. The TUI (internal/tui)` | 453–474 | 22 |
| `## 7. Data assets (assets/, go:embed)` | 475–487 | 13 |
| `## 8. Testing` | 488–521 | 34 |
| `## 9. Phase-by-phase status` | 522–549 | 28 |
| `## 10. Carry-forward seams & backlog` | 550–571 | 22 |
| `## 11. How to build, run, and use` | 572–608 | 37 |
| `## 12. Conventions for contributors` | 609–628 | 20 |
| `## 13. Current working state` | 629–641 | 13 |
| `## 14. Other Features, Work & Safety Considerations` (§14.1–§14.5) | 642–783 | 142 |

Subsections: §3.1–§3.11 (lines 121, 178, 185, 193, 202, 217, 230, 237, 249, 259, 275); §4.1–§4.13 (292, 299, 307,
316, 322, 329, 338, 346, 357, 363, 373, 384, 398); §14.1–§14.5 (644, 670, 698, 725, 752). Lines 758–776 that start
with `# ` are shell comments inside a fenced block, not headings.

### Surprises noted at D1

1. **Section-number collision:** §0, §9 and §13 call the deferred work "`lang-strip` (§4.7)" and "release pipeline
   (§4.8)". Those are Phase 4 increment numbers (master TD / `Docs/Phases.md`), but in this doc §4.7 is `maintain`
   and §4.8 is `docker`. A reader following "§4.7" lands on the wrong thing.
2. **Commit state is stale:** lines 8 and 631 say the latest commit is `fa81f5f`; two commits followed (`5add435`
   nettop parsing fix, `abaff0d` CLAUDE.md docs). Whether the nettop fix changed anything the doc describes is checked
   in D4.
3. **Hard-coded counts with no recount command:** §0 gives ~15,600 LOC, 34 packages, 170 test functions, 24 test
   packages (AP10).
4. **No IDs on open work:** §10's on-device seams and backlog items, and §14.2's feature gaps, have no stable IDs.
5. **Self-assessment:** §14.3 "What is already rock-solid" (AP14).
6. No date anywhere in the doc (AP9); no `Docs/Changelog.md` exists.

## D2. Section map

Kinds: *Current* (re-checked against the code and rewritten in D4) · *History* (moves word for word to the Changelog
in D3) · *Copy of code* (dropped, AP3) · *Elsewhere* (pointer to another doc) · *Dropped* (reason given).

| Old (lines) | Goes to | Kind | Note |
|---|---|---|---|
| Title (1) | Header | Dropped | Replaced by `# Suns: status` |
| "What this document is" banner (3–8) | Header | Dropped | AP5; line 8 is commit state (AP1) |
| §0 prose: what Suns is (14–19) | §3 What it is | Current | |
| §0 metrics: language, module, platform (23, 29) | Header *Identity*, *Stack* | Current | Re-check `go.mod`, version string |
| §0 metrics: LOC, packages, `pkg/` and `internal/` LOC (24–26) | Dropped | Dropped | AP10: drifting counts; not needed by a session |
| §0 metrics: test count and result (27–28) | §0 Current state | Current | Re-run in D4; count with recount command |
| §0 metrics: phases complete, deferred (30–31) | §0 (one line) and §2 | Current | Wrong "§4.7/§4.8" refs fixed: they are `Docs/Phases.md` §4.7/§4.8 |
| §0 design north star (33–38) | §4 How it works (governing principle) | Current | Emoji to words |
| §1 repository tree (44–87) | §5 Where things live (module map) | Current | Tree dropped (AP3); one line per package, re-checked with `go list ./...` |
| §1 dependency direction (89–91) | §1.3 Invariants and §4 | Current | Find the guard, else `Guard: none` |
| §2 direct requires (97–108) | Header *Stack* and §8 | Current | Re-check `go.mod` |
| §2 planned deps (110–112) | §2 Open work | Current | Merge with backlog item |
| §3 intro (118–119) | §4 | Current | |
| §3.1 Operation model, op kinds, seams, Receipt (121–176) | §4 | Current | Interface body dropped (AP3, point to `pkg/operation/operation.go`); op-kind table kept, re-checked against `kind.go` |
| §3.2 plan (178–183) | §4 and §1.3 | Current | Seal invariant with its test |
| §3.3 gate (185–191) | §4 | Current | |
| §3.4 revalidate + execute (193–200) | §4 and §1.3 | Current | "§6.1" is master TD §6.1; name the doc |
| §3.5 deny floor (202–215) | §1.3 and §4 | Current | "§5.3" is master TD §5.3; floor list re-checked in `pkg/safety/floor` |
| §3.6 tiered identity (217–228) | §4 | Current | |
| §3.7 fd-anchored deleter (230–235) | §4 and §1.3 | Current | |
| §3.8 Trasher (237–247) | §4; seams to §2 | Current | |
| §3.9 privilege chokepoint (249–257) | §4 and §1.3 | Current | |
| §3.10 syscmd executor, pinned tools (259–273) | §4 and §1.3 | Current | Pinned list re-checked against `productionAllowlist` (nettop fix may have touched it) |
| §3.11 history (275–281) | §6 Data, formats and limits | Current | File path, mode, format version |
| §4 intro (287–290) | §4 | Current | |
| §4.1 scanner (292–297) | §7.1 clean | Current | |
| §4.2 dedup (299–305) | §7.2 ashen | Current | Test count dropped (AP10) |
| §4.3 uninstaller (307–314) | §7.3 nuke | Current | |
| §4.4 orphans (316–320) | §7.4 orphans | Current | |
| §4.5 restore (322–327) | §7.5 restore | Current | Test count dropped |
| §4.6 purge (329–336) | §7.6 empty-dirs, broken-symlinks, scan | Current | Test count dropped |
| §4.7 maintain (338–344) | §7.7 maintain | Current | |
| §4.8 docker (346–355) | §7.8 docker-prune | Current | Socket paths are runtime home paths (§13 *Not repo paths*) |
| §4.9 scheduler (357–361) | §7.10 schedule | Current | `~/Library/Logs/Suns/` to *Not repo paths* |
| §4.10 audit (363–371) | §7.11 audit | Current | |
| §4.11 net (373–382) | §7.12 net | Current | Re-check nettop parsing after `5add435` |
| §4.12 telemetry (384–396) | §7.13 get-coffee | Current | Re-check nettop stream after `5add435` |
| §4.13 supporting engines (398–405) | §7.14 ps/kill, §7.15 doctor; config to §6 | Current | |
| §5 CLI: root wiring (411–413) | §4 | Current | |
| §5 CLI: command table (415–438) | §3 (how a person uses it) | Current | Badges to words; re-check against `internal/cli` |
| §5 CLI: common flags, gate behaviour (440–449) | §3 and §4 | Current | `-y` concern goes to §2 |
| §6 TUI (453–471) | §7.13 get-coffee and §7.16 TUI menu | Current | Emoji badges to words |
| §7 data assets (475–484) | §6 | Current | 12 targets and ~76 OUI rows re-counted, with recount command |
| §8 testing: count, conventions (488–496) | §9 Testing; conventions to §1.1 | Current | |
| §8 per-package test count table (498–510) | Dropped | Dropped | AP10 + AP3; replaced by a recount command |
| §8 packages without tests (512–516) | §9 (not covered) and §2 | Current | Re-check with `go test` output |
| §8 how to run (518) | §8 | Current | |
| §9 phase-by-phase status (522–548) | Changelog | History | Word for word; `Docs/Phases.md` already holds the detail |
| §10 on-device seams (552–565) | §2 Open work | Current | New IDs O-1… |
| §10 cross-cutting backlog (567–570) | §2 Open work | Current | Split into one ID each |
| §11 build/verify commands (574–580) | §8 | Current | |
| §11 safe ways to explore (582–595) | §3 (owner's use) | Current | Marked owner-only: sessions may not run these (§1.2) |
| §11 destructive and privileged actions (597–605) | §3 | Current | |
| §12 conventions (611–625) | §1.1 Working rules, §1.3 | Current | Same as `CLAUDE.md` hard rules, kept here with reasons (A4); "§13.1" is master TD §13.1 |
| §13 commit-state line (631–632) | Dropped | Dropped | AP1 |
| §13 phases implemented (633–634) | §0 | Current | Re-run verify |
| §13 next work (635–638) | §2 | Current | lang-strip danger note goes with its item |
| §14.1a FDA requirement (646–653) | §11.1 limitation + §2 (onboarding panel) | Current | Check `doctor` FDA detection |
| §14.1b scheduled run silently skips without FDA (655–662) | §11.1 limitation + §2 | Current | Check `scheduled_run` statuses in code |
| §14.1c `-y` brevity (664–668) | §2 (proposal, owner decides) | Current | |
| §14.2a `suns config` (672–684) | §2 | Current | Same item as backlog `config get/set/show` |
| §14.2b history rotation (686–691) | §2 + §11.1 (history grows without bound) | Current | Same item as backlog rotation |
| §14.2c restore in the TUI (693–696) | §2 | Current | Re-check TUI restore screen |
| §14.3 what is already rock-solid (698–723) | §1.3 Invariants (facts only) | Dropped | AP14 as prose; each claim becomes an invariant with its test, if one exists |
| §14.4 distribution without Developer ID (725–750) | §8 (release options) and §10.3 or §2 | Current | Conflicts with §9's "deferred: needs Apple Developer credentials": owner question |
| §14.5 first-run safety sequence (752–782) | §3 (owner's first run) | Current | Condensed; owner-only commands |

### Template sections nothing maps to (filled from the code or the owner in D4)

- **§1.2 Hazards** — from `CLAUDE.md`, `.claude/settings.json` hooks (`suns-guard.sh`, `gofmt-check.sh`), WorkMap §4.
- **§10 Decisions** — only the lang-strip/release deferral exists; rulings will come from `CLAUDE.md`, master TD and
  the owner. §10.2 deviations from the master TD (e.g. `optimizer` as a doc stub, `CacheReset` reserved) from code.
- **§11.2 Traps, §11.3 Diagnoses** — likely the nettop parsing fix (`5add435`); else `None yet.`
- **§12 Documents** — `Docs/SunsMasterTD.md`, `Docs/Phases.md`, `README.md`, `.claude/rules/safety.md`.
- **§13 How this doc is verified** — new.
- **§14 Glossary** — ashen, nuke, jarjar, deathstar, get-coffee, obliterate, gate, floor, seam, burn.

### Old → new section numbers (for D5)

| Old | New |
|---|---|
| §0 | §0 (state), §3 (what it is), §4 (north star) |
| §1 | §5 Where things live |
| §2 | §8 Build, release and deploy |
| §3, §3.1–§3.11 | §4 How it works (floor, seal, revalidate also §1.3) |
| §4, §4.1–§4.13 | §7 Features (§4.N → §7.N per the table above) |
| §5 | §3 What it is |
| §6 | §7.13 / §7.16 |
| §7 | §6 Data, formats and limits |
| §8 | §9 Testing |
| §9 | `Docs/Changelog.md` |
| §10 | §2 Open work |
| §11 | §8 (commands), §3 (use) |
| §12 | §1.1 Working rules |
| §13 | §0, §2 |
| §14.1, §14.2 | §2, §11.1 |
| §14.3 | §1.3 |
| §14.4 | §8 |
| §14.5 | §3 |

Old in-doc references that point at other docs, to name in D4: §6.1, §5.3, §13.1 → `SunsMasterTD` §6.1, §5.3,
§13.1; §4.7, §4.8 (deferred work) → `Docs/Phases.md` §4.7, §4.8.
