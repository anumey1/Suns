// Package maintain is the Git Repository Garbage Collector engine (§12.17).
//
// It discovers git repositories under the given roots, estimates the space each
// would reclaim, and reports the cleanliness of each working tree, emitting a
// RepoMaintenanceOp (🟡) only for repositories that are clean and have no
// merge/rebase in progress. Repositories with uncommitted changes or in-progress
// operations are listed with a reason and never collected — the de-fanged
// default is a plain `git gc` (two-week reflog window); aggressive collection and
// immediate pruning are explicit opt-ins surfaced with a per-repo warning. All
// discovery is read-only and routes git through the hardened executor.
package maintain
