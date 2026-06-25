package operation

import "fmt"

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
	KindScheduledRun    OpKind = "scheduled_run"
)

// Reversibility is the honest, per-operation classification surfaced in the
// gate as 🟢 / 🟡 / 🔴 (§4.2).
type Reversibility int

const (
	Reversible   Reversibility = iota // e.g. a trashed file → restore
	Recoverable                       // e.g. git gc within reflog window; a cache that rebuilds
	Irreversible                      // e.g. a killed process; an obliterated file; a DNS flush
)

// String returns the neutral machine key for the reversibility class, used in
// the operation history (§4.10, §13.3).
func (r Reversibility) String() string {
	switch r {
	case Reversible:
		return "reversible"
	case Recoverable:
		return "recoverable"
	case Irreversible:
		return "irreversible"
	default:
		return "unknown"
	}
}

// MarshalText renders the reversibility as its machine key in JSON.
func (r Reversibility) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// UnmarshalText parses the machine key back into a Reversibility.
func (r *Reversibility) UnmarshalText(b []byte) error {
	switch string(b) {
	case "reversible":
		*r = Reversible
	case "recoverable":
		*r = Recoverable
	case "irreversible":
		*r = Irreversible
	default:
		return fmt.Errorf("operation: unknown reversibility %q", b)
	}
	return nil
}

// Mode is the deletion axis (Jarjar). It applies to FileDelete only (§4.3);
// non-file operations ignore it entirely.
type Mode int

const (
	ModeTrash      Mode = iota // move files to the macOS Trash (recoverable)
	ModeObliterate             // permanently delete
)

// String returns the neutral machine key for the deletion mode.
func (m Mode) String() string {
	switch m {
	case ModeTrash:
		return "trash"
	case ModeObliterate:
		return "obliterate"
	default:
		return "unknown"
	}
}
