// Package restore implements `suns restore`: undo from the operation history,
// acting only on records classified as Reversible file_delete (§4.8).
//
// Restoration uses Suns's own recorded original path (not the OS "Put Back"
// facility), so it behaves identically whether the file was trashed via the
// native API or the pure-Go fallback. Before restoring, the trashed object is
// verified against its recorded tiered identity by CONTENT (size+mtime for
// large, hash for small) — a mismatch means the Trash entry was modified or
// swapped and the restore is refused. Collisions fail loud rather than silently
// relocating to an inert holding folder.
package restore

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"github.com/anumey1/Suns/pkg/trash"
)

// Outcome reports the result of attempting one restore.
type Outcome struct {
	Entry    operation.HistoryEntry
	Restored bool
	Path     string // where it ended up (may differ from OrigPath on collision)
	Reason   string // why it was skipped or failed
}

// Candidates returns the history entries eligible for restore (Reversible,
// trashed file_delete with both a trash path and an original path), most-recent
// first.
func Candidates(entries []operation.HistoryEntry) []operation.HistoryEntry {
	var out []operation.HistoryEntry
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Op == operation.KindFileDelete &&
			e.Reversible == operation.Reversible &&
			e.Fate == "trashed" &&
			e.TrashPath != "" && e.OrigPath != "" {
			out = append(out, e)
		}
	}
	return out
}

// Restore verifies and moves one trashed entry back to its original path,
// applying the fail-loud collision rules (§4.8).
func Restore(e operation.HistoryEntry) Outcome {
	out := Outcome{Entry: e}

	if _, err := os.Lstat(e.TrashPath); err != nil {
		out.Reason = fmt.Sprintf("trash entry missing: %v", err)
		return out
	}

	// Verify the trashed object was not modified or swapped after deletion.
	if e.Identity == nil {
		out.Reason = "no recorded identity; refusing to restore unverifiable entry"
		return out
	}
	cur, err := identity.ComputeFile(e.TrashPath, identity.DefaultLargeThreshold)
	if err != nil {
		out.Reason = fmt.Sprintf("cannot stat trash entry: %v", err)
		return out
	}
	if err := identity.VerifyContent(*e.Identity, cur); err != nil {
		out.Reason = "Trash entry was modified since deletion; refusing to restore"
		return out
	}

	dest := e.OrigPath

	// Collision: the original path is occupied. Restore alongside rather than
	// overwrite, and report the alternate name.
	if _, err := os.Lstat(dest); err == nil {
		dest = alongside(dest)
	} else {
		// Parent directory gone but recreatable: recreate it and restore.
		parent := filepath.Dir(dest)
		if _, perr := os.Stat(parent); os.IsNotExist(perr) {
			if mkerr := os.MkdirAll(parent, 0o755); mkerr != nil {
				out.Reason = fmt.Sprintf("parent directory %q is gone and not recreatable: %v", parent, mkerr)
				return out
			}
		}
	}

	if err := trash.MovePath(e.TrashPath, dest); err != nil {
		out.Reason = fmt.Sprintf("could not move back to %q: %v", dest, err)
		return out
	}
	out.Restored = true
	out.Path = dest
	return out
}

// alongside returns "name (restored <timestamp>).ext" in the same directory as
// the occupied original path.
func alongside(occupied string) string {
	dir := filepath.Dir(occupied)
	base := filepath.Base(occupied)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	stamp := time.Now().Format("2006-01-02 15-04-05")
	return filepath.Join(dir, fmt.Sprintf("%s (restored %s)%s", stem, stamp, ext))
}
