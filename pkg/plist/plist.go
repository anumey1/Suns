// Package plist handles property-list parsing and the powermetrics stream
// tokenizer (§2.4, §7.3).
//
// Many modern Info.plist files are binary, which encoding/xml cannot parse;
// howett.net/plist handles both binary and XML, with a `plutil` shell fallback.
//
// Separately, `powermetrics -f plist` emits a CONCATENATED sequence of discrete
// plist documents, not one well-formed document; feeding that to a
// single-document decoder fails at the first boundary. The tokenizer scans for
// document delimiters, buffers one complete <?xml … </plist> block at a time,
// and feeds each to the decoder individually. Critically it sets a READ-DEADLINE
// (≈ the poll interval): if a complete document does not arrive in time
// (powermetrics can stall mid-flush under load, leaving a partial document with
// no closing tag and no EOF), the tokenizer flushes its buffer, marks the
// source stale, and signals the supervisor to restart the subprocess. "Stalled"
// is a first-class state, distinct from "EOF/dead".
package plist

// The binary-safe property-list reader lives in reader.go. The deadline-guarded
// powermetrics stream tokenizer is implemented in tokenizer.go.
