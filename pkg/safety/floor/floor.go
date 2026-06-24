// Package floor implements the never-touch deny floor (§5.3, §13.7).
//
// It is a versioned manifest of paths that nothing in the program may ever
// delete, enforced at plan time, execution time, and every directory descent.
// No flag, no mode, no config value, and no scheduled job can bypass it —
// confirm_mode=on and deletion_mode=obliterate do not override it. The check
// consults only the path and the volume the path resides on — never a mode,
// flag, or config value — so it is mode-independent by construction.
//
// Two layers: a versioned path-prefix deny list (fast, deterministic), and a
// volume-residence guard that denies anything on a read-only mount. The macOS
// System Volume is mounted read-only, so the residence guard is how the §5.3
// "anything on a System Volume" clause is enforced — by where a path physically
// lives, not by how it is spelled.
//
// Leaf package: standard library + golang.org/x/sys/unix only.
package floor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// ErrDenied is returned by Check when a path is protected by the floor.
var ErrDenied = errors.New("floor: path is protected and may never be deleted")

// deniedPrefixes are roots that may never be deleted (the path itself or
// anything beneath it). See §5.3. /usr is denied except for /usr/local, which
// is handled as an explicit carve-out in Check.
var deniedPrefixes = []string{
	"/System",
	"/usr",
	"/bin",
	"/sbin",
	"/Library/Apple",
	"/private/var/db", // SIP-protected system databases
	"/cores",
}

// allowedUnderUsr are the carve-outs beneath an otherwise-denied prefix.
var allowedUnderUsr = []string{
	"/usr/local",
}

// Check returns ErrDenied if path is protected by the floor, else nil. The path
// is cleaned and made absolute before comparison; relative paths are resolved
// against the current working directory. The user's home root itself is
// protected (only specific subpaths are ever targeted, never ~ directly), as is
// the iCloud Mobile Documents container.
func Check(path string) error {
	p, err := normalize(path)
	if err != nil {
		return fmt.Errorf("floor: %w", err)
	}

	if p == "/" {
		return fmt.Errorf("%w: filesystem root", ErrDenied)
	}

	// The home directory root itself, and the iCloud container, are protected.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		hp := filepath.Clean(home)
		if p == hp {
			return fmt.Errorf("%w: home directory root", ErrDenied)
		}
		icloud := filepath.Join(hp, "Library", "Mobile Documents")
		if p == icloud || strings.HasPrefix(p, icloud+"/") {
			return fmt.Errorf("%w: iCloud Drive container", ErrDenied)
		}
	}

	for _, prefix := range deniedPrefixes {
		if !under(p, prefix) {
			continue
		}
		if prefix == "/usr" && allowedUnder(p, allowedUnderUsr) {
			continue
		}
		return fmt.Errorf("%w: %s", ErrDenied, prefix)
	}

	// §5.3 "anything on a System Volume": the System Volume is mounted read-only,
	// so deny any path that physically resides on a read-only mount. This is
	// residence-based, catching System-Volume paths that the prefix list does not
	// enumerate (and any other read-only mount, which is correct — nothing there
	// is deletable anyway).
	if onReadOnlyVolume(p) {
		return fmt.Errorf("%w: read-only system volume", ErrDenied)
	}
	return nil
}

// onReadOnlyVolume reports whether path resides on a read-only mount. A path that
// cannot be stat'd (e.g. it is already gone) is treated as not-on-a-read-only
// volume: the prefix list above and exec-time revalidation cover vanished
// targets, and a delete of a missing path is a no-op regardless.
func onReadOnlyVolume(path string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return false
	}
	return st.Flags&unix.MNT_RDONLY != 0
}

// Permits is the boolean form of Check.
func Permits(path string) bool { return Check(path) == nil }

func normalize(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		path = abs
	}
	return filepath.Clean(path), nil
}

// under reports whether p is prefix itself or lies beneath it.
func under(p, prefix string) bool {
	return p == prefix || strings.HasPrefix(p, prefix+"/")
}

func allowedUnder(p string, allow []string) bool {
	for _, a := range allow {
		if under(p, a) {
			return true
		}
	}
	return false
}
