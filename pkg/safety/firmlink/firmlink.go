// Package firmlink provides firmlink-aware identity handling (§4.6).
//
// macOS binds the read-only System volume and the writable Data volume with
// firmlinks (e.g. /var → /private/var, /Users → /System/Volumes/Data/Users).
// Firmlinks are NOT symlinks, so O_NOFOLLOW does not reject them — but crossing
// one changes the device number, which would otherwise trip the device+inode
// identity check and wrongly abort a legitimate path. This package detects
// OS-synthesized firmlink boundaries so the device-number change can be
// permitted across exactly those boundaries, while user-space symlink
// redirections are still rejected.
//
// The allowance is constrained to the known OS map; it is NOT a general
// "ignore device changes" escape hatch.
//
// Leaf package: standard library only.
package firmlink

import (
	"path/filepath"
	"strings"
)

// knownBoundaries is the set of OS-synthesized firmlink roots on modern macOS.
// A path at or under one of these may legitimately change device number.
// This map is conservative and versioned with releases (§5.2 philosophy: the
// manifest ships embedded, never fetched).
var knownBoundaries = []string{
	"/var",                 // → /private/var
	"/tmp",                 // → /private/tmp
	"/etc",                 // → /private/etc
	"/Users",               // → /System/Volumes/Data/Users
	"/Volumes",             // → /System/Volumes/Data/Volumes
	"/Applications",        // → /System/Volumes/Data/Applications
	"/Library",             // → /System/Volumes/Data/Library
	"/System/Volumes/Data", // the Data volume mount itself
	"/private/var",         // the firmlink targets, for paths expressed post-resolution
	"/private/tmp",
	"/private/etc",
}

// IsBoundary reports whether path is at or under a known OS firmlink boundary,
// meaning a device-number change there is legitimate rather than a symlink
// redirection.
func IsBoundary(path string) bool {
	p := filepath.Clean(path)
	for _, b := range knownBoundaries {
		if p == b || strings.HasPrefix(p, b+"/") {
			return true
		}
	}
	return false
}
