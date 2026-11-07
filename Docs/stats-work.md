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
| D2 Map every old section | Open | |
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
