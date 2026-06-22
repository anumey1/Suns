// Package operation defines the generalized Operation model: every auditable,
// potentially-destructive action is a typed Operation, not merely a file
// deletion (§4.1, §4.2). The safety apparatus is polymorphic over OpKind.
//
// Concrete operation types are deliberately designed as value types so that
// plan.Seal can deep-copy them into a pointer-free form (§4.5).
package operation

import "context"

// OpKind identifies the kind of action. The string values are the neutral,
// scriptable machine keys used in config and the operation history (§13.3).
type OpKind string

const (
	KindFileDelete      OpKind = "file_delete"
	KindProcessKill     OpKind = "process_kill"
	KindCacheReset      OpKind = "cache_reset"
	KindServiceUnload   OpKind = "service_unload"
	KindRepoMaintenance OpKind = "repo_maintenance"
	KindContainerPrune  OpKind = "container_prune"
	KindReceiptForget   OpKind = "receipt_forget"
	KindDNSFlush        OpKind = "dns_flush"
)

// Reversibility is the honest, per-operation classification surfaced in the
// gate as 🟢 / 🟡 / 🔴 (§4.2).
type Reversibility int

const (
	Reversible   Reversibility = iota // e.g. a trashed file → restore
	Recoverable                       // e.g. git gc within reflog window; a cache that rebuilds
	Irreversible                      // e.g. a killed process; an obliterated file; a DNS flush
)

// Mode is the deletion axis (Jarjar). It applies to FileDelete only (§4.3);
// non-file operations ignore it entirely.
type Mode int

const (
	ModeTrash      Mode = iota // move files to the macOS Trash (recoverable)
	ModeObliterate             // permanently delete
)

// Identity is execution-time proof that a target is still exactly what was
// planned (§4.7). It is populated per-kind: device+inode for files, PID+birth
// time+executable path for processes, domain+label+plist for services, and a
// content hash for small high-risk files.
type Identity struct{}

// Preview is the per-kind render payload the gate displays (§4.2): a file
// table for FileDelete, a process table for ProcessKill, a one-liner for
// DNSFlush, and so on.
type Preview struct{}

// Receipt is the outcome of Execute, fed to HistoryRecord.
type Receipt struct{}

// HistoryEntry is a typed operation-history record appropriate to the kind
// (§4.10).
type HistoryEntry struct{}

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
