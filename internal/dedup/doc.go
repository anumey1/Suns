// Package dedup is the APFS-conservative duplicate finder backing the
// `suns ashen` command (§12.1). Destructive · gated · FileDelete.
//
// Naming: ash is largely carbon, and burnt "Carbon Copies" come to rest in an
// ashen state. Duplicate files are carbon copies; `ashen` burns the redundant
// ones down to a single keeper. (The engine package keeps the technical name
// `dedup` for §12.1 traceability; the user-facing command is `ashen`, with a
// hidden `dedup` alias.)
//
// Find runs three passes: group by exact byte size → hash the first 4 KB as a
// cheap discriminator → full SHA-256 on the survivors; the eventual delete uses
// the existing FileDeleteOp with tiered identity (§4.7), so the gate, history,
// and restore handle it unchanged.
//
// Correctness rules (§12.1):
//   - APFS block sharing does NOT identify clone lineage, so shared-block files
//     are NOT excluded (excluding them would skip real duplicates). Sharing is
//     surfaced only as a conservative-space caveat: freed space may be lower
//     than the nominal total.
//   - Hardlinks (same dev+inode) are the same file and are never offered.
//   - Bundles (.app, .rtfd, .photoslibrary, …) are atomic — interior files are
//     never deduped.
//   - Comparison is over data forks, so cosmetic xattrs (quarantine, Finder
//     tags, where-from) are ignored; a note is surfaced when they differ.
//   - The keeper heuristic prefers non-cache, non-trash, user-document
//     locations and never auto-selects across bundles.
//
// Find itself performs no deletion: it is read-only and returns a Report whose
// ops are sealed into a plan and confirmed at the gate before anything happens.
package dedup
