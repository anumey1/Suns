---
paths:
  - "pkg/safety/**"
  - "pkg/trash/**"
  - "pkg/privilege/**"
  - "pkg/operation/**"
  - "pkg/plan/**"
  - "pkg/syscmd/**"
  - "pkg/history/**"
---
# The dangerous core (Phase 0 spine)

These packages are what make Suns safe. Changes here need the most care and the strongest tests.

- Plans are frozen value copies (`plan.Seal()`): nothing may mutate a plan after it is shown at the gate.
- Deletion is fd-anchored and firmlink-aware; never replace it with path-string deletes.
- Every target is re-validated at execution time (tiered identity checks, privilege-correct) — TOCTOU matters.
- The never-touch deny floor can only grow; never weaken it (master TD §5.3, §13.7 safety-floor invariance).
- Trash is bounded and degrades to the pure-Go fallback; restore is honest and fails loudly.
- All privilege escalation goes through the single `pkg/privilege` chokepoint; all external tools through the
  hardened `pkg/syscmd` executor and its allowlist.
- Detail: `Docs/stats.md` §3 and `Docs/SunsMasterTD.md` §4 — use `/stats safety` for line numbers.
