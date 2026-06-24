package uninstaller

import "strings"

// Parsing contracts for pkgutil (§13.1). pkgutil emits stable, line-oriented
// plain text that has not changed in many macOS releases; we still parse
// defensively (trim, skip blanks, ignore unknown keys) and degrade to empty
// results on unexpected output rather than guessing. Every call is made under
// the hardened executor with LC_ALL=C for locale-deterministic text.

// parseLines splits tool output into trimmed, non-empty lines. It backs the
// `pkgutil --pkgs` and `pkgutil --files <id>` contracts, both of which emit one
// value per line.
func parseLines(b []byte) []string {
	var out []string
	for _, ln := range strings.Split(string(b), "\n") {
		if ln = strings.TrimSpace(ln); ln != "" {
			out = append(out, ln)
		}
	}
	return out
}

// pkgInfo is the subset of `pkgutil --pkg-info <id>` Suns needs to resolve a
// payload's absolute paths: the install volume and (relocatable) location.
type pkgInfo struct {
	Volume   string
	Location string
}

// parsePkgInfo reads `pkgutil --pkg-info <id>` key: value lines. Volume defaults
// to "/" when absent so the common case needs no special handling.
func parsePkgInfo(b []byte) pkgInfo {
	pi := pkgInfo{Volume: "/"}
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if v, ok := strings.CutPrefix(ln, "volume:"); ok {
			pi.Volume = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(ln, "location:"); ok {
			pi.Location = strings.TrimSpace(v)
		}
	}
	if pi.Volume == "" {
		pi.Volume = "/"
	}
	return pi
}

// parseFileInfoPkgIDs extracts every `pkgid:` value from `pkgutil --file-info
// <path>`. A path claimed by MORE THAN ONE package id is a shared dependency:
// the §12.15 guard excludes it so removing one app cannot brick another by
// deleting a shared dylib or Audio Unit.
func parseFileInfoPkgIDs(b []byte) []string {
	var ids []string
	for _, ln := range strings.Split(string(b), "\n") {
		ln = strings.TrimSpace(ln)
		if v, ok := strings.CutPrefix(ln, "pkgid:"); ok {
			if id := strings.TrimSpace(v); id != "" {
				ids = append(ids, id)
			}
		}
	}
	return ids
}

// appPrefix returns the first three reverse-DNS components of an identifier
// (e.g. com.vendor.app), used to relate package receipts to a bundle ID without
// the over-broad two-component vendor match (which would catch com.apple.*).
func appPrefix(id string) string {
	parts := strings.Split(id, ".")
	if len(parts) >= 3 {
		return strings.Join(parts[:3], ".")
	}
	return id
}

// relatedPkg reports whether a package receipt id plausibly belongs to the app
// identified by bundleID. It is a best-effort heuristic; the operator reviews the
// resulting plan at the gate before anything is forgotten.
func relatedPkg(pkgID, bundleID string) bool {
	if pkgID == "" || bundleID == "" {
		return false
	}
	if strings.Contains(pkgID, bundleID) || strings.Contains(bundleID, pkgID) {
		return true
	}
	return appPrefix(pkgID) == appPrefix(bundleID)
}
