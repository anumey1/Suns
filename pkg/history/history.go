// Package history implements the operation-history log (§4.10, §13.3).
//
// The canonical record lives at ~/Library/Application Support/Suns/history.jsonl
// (mode 0600, append-only, crash-safe JSON Lines). Each operation writes a
// typed, per-kind record carrying the Plan.ID, which makes the reversibility
// claim honest: the log records what kind of action happened and whether it can
// be undone, instead of pretending everything is a restorable file.
//
// It is an operational record, NOT a tamper-evident forensic trail — there is
// no cryptographic chaining or signing in v1, and it is described as such.
//
// The canonical log is full fidelity (real absolute paths and identities)
// because restore and debugging depend on it; redaction is an export-only
// transform (`suns history export --redact`), never applied in place.
package history

// TODO(phase0): Append(entry operation.HistoryEntry), the 0600 JSONL writer,
// rotation/size-cap, and the export-redaction transform.
