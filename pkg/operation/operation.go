// Package operation defines the generalized Operation model: every auditable,
// potentially-destructive action is a typed Operation, not merely a file
// deletion (§4.1, §4.2). The safety apparatus is polymorphic over OpKind.
//
// Concrete operation types are deliberately designed as VALUE types (no
// pointers, slices, or maps in their fields) so that plan.Seal can deep-copy
// them into a pointer-free form that retains no references into live state
// (§4.5).
package operation

import (
	"context"
	"time"

	"github.com/anumey1/Suns/pkg/safety/identity"
)

// Identity is execution-time proof that a target is still exactly what was
// planned (§4.7). The Kind field selects which sub-identity is populated:
// File for FileDelete, Process for ProcessKill, Service for ServiceUnload.
type Identity struct {
	Kind    OpKind
	File    identity.FileIdent
	Process identity.ProcessIdent
	Service identity.ServiceIdent
}

// Preview is the per-target render payload the gate displays (§4.2). The gate
// groups previews by Kind and renders the appropriate table plus a single
// reversibility badge per group.
type Preview struct {
	Kind          OpKind
	Reversibility Reversibility
	Line          string // human-readable one-line description of this target
	Bytes         int64  // reclaimable bytes (FileDelete); 0 otherwise
}

// Receipt is the outcome of Execute, fed to HistoryRecord.
type Receipt struct {
	Kind      OpKind
	Fate      string // "trashed" | "obliterated" | "skipped" | "killed" | "flushed"
	Status    string // "ok" | "skipped:<reason>" | "failed"
	TrashPath string // for trashed files
	Err       error  // set when Status == "failed"
	Time      time.Time
}

// HistoryEntry is a typed operation-history record (§4.10). Its fields cover all
// op kinds; per-kind records populate only the relevant ones (omitempty keeps
// the JSONL shape clean and matches the documented examples).
type HistoryEntry struct {
	TS         time.Time     `json:"ts"`
	Plan       string        `json:"plan,omitempty"`
	Op         OpKind        `json:"op"`
	Reversible Reversibility `json:"reversible"`
	Cmd        string        `json:"cmd,omitempty"`
	Status     string        `json:"status,omitempty"`

	// file_delete
	Path      string              `json:"path,omitempty"`
	Size      int64               `json:"size,omitempty"`
	Fate      string              `json:"fate,omitempty"`
	TrashPath string              `json:"trash_path,omitempty"`
	OrigPath  string              `json:"orig_path,omitempty"`
	Identity  *identity.FileIdent `json:"identity,omitempty"`

	// process_kill
	PID    int    `json:"pid,omitempty"`
	Name   string `json:"name,omitempty"`
	Birth  string `json:"birth,omitempty"`
	Exec   string `json:"exec,omitempty"`
	Signal string `json:"signal,omitempty"`
}

// Operation is any auditable, potentially-destructive action (§4.2).
type Operation interface {
	Kind() OpKind
	Describe() Preview
	Reversibility() Reversibility
	// ValidateAtPlan checks the operation is sensible at discovery time.
	ValidateAtPlan(ctx context.Context) error
	// ValidateAtExec re-checks, at execution time, that the target is still
	// what was planned, returning a typed Identity that Execute refuses to act
	// without (TOCTOU and PID-reuse defense).
	ValidateAtExec(ctx context.Context) (Identity, error)
	Execute(ctx context.Context, mode Mode, id Identity) (Receipt, error)
	HistoryRecord(Receipt) HistoryEntry
}
