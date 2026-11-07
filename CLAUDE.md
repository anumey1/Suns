# Suns

"Super User Nutcase Sessions — Burn It All": a safety-obsessed macOS maintenance utility in Go (module
`github.com/anumey1/Suns`, Go 1.25, toolchain 1.26). Cobra CLI + Bubble Tea TUI. Cleans dev caches, uninstalls
apps, de-duplicates files, audits security and the network, live telemetry dashboard (`get-coffee`), scheduled
cleanups. Core promise: a first run with no flags can never permanently destroy data.

## Status doc
- `Docs/stats.md` is the status doc. Load it with `/stats` (header + section index), then read only the
  sections the task needs. Read `stats.md §1 Rules and hazards` before changing anything.
- Every change updates the sections it touched and the *Updated* row, and adds an entry at the top of
  `Docs/Changelog.md`, in the same change.

## Where things are
- `Docs/SunsMasterTD.md` — the v6 master design (single source of truth). `Docs/Phases.md` — the completion log.
- `cmd/suns` entry · `internal/*` engines, CLI, TUI · `pkg/*` the reusable spine (operation, plan, safety, trash,
  history, privilege, syscmd, plist) · `assets/` (go:embed).

## Build and test
- `go build ./... && go vet ./... && go test -race ./...` (all green, ~13 s cold). `gofmt` is checked by a hook after edits.

## Hard rules
- **Never run Suns itself** except with `--dry-run`, `--help` or `help`/`version` — enforced by a hook. That
  includes the read-only commands (`doctor`, `audit`, `net`, `scan`, `get-coffee`). If the owner wants one run, ask
  them to type it with `!` so it runs as them. Prove behaviour with unit tests instead.
- Tests: deletion tests use only `t.TempDir()`; never run a real destructive or privileged command in a test.
- New destructive action: model it as an `Operation` value type in `pkg/operation`, route it through
  `plan.New().Seal()` → `safety.BuildGroups/Render` → `safety.Execute`, re-verify the target in `ValidateAtExec`
  (TOCTOU), and record a typed `HistoryEntry`.
- New external tool: pin it in `pkg/syscmd` (`productionAllowlist`) and `internal/doctor`'s `pinnedTools`, with a
  parsing contract tested against a captured fixture through an injected runner.
- Package docs live in `doc.go`.
