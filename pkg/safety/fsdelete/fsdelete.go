// Package fsdelete performs fd-anchored recursive deletion — used only in
// obliterate mode (§4.6).
//
// Go's os.RemoveAll walks by path and re-resolves components as it descends, so
// a concurrent directory-to-symlink swap can redirect it outside the intended
// subtree. fsdelete instead descends using openat relative to directory file
// descriptors with O_NOFOLLOW|O_DIRECTORY, never re-resolving full paths: a swap
// of a higher component cannot redirect the descent. Entries are removed with
// unlinkat in post-order (children before parents). A swapped or replaced
// component fails the no-follow openat (or the root identity check) and is
// skipped and reported — the deletion can never escape the intended subtree.
//
// In trash mode the fd-anchored walker is NOT used: the approved root is moved
// atomically as a unit by pkg/trash. This package is obliterate-only.
package fsdelete

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/anumey1/Suns/pkg/safety/firmlink"
	"github.com/anumey1/Suns/pkg/safety/identity"
	"golang.org/x/sys/unix"
)

// Result reports the outcome of an Obliterate call.
type Result struct {
	Removed int      // entries successfully unlinked
	Skipped []string // entries skipped, with reasons appended
}

// Obliterate permanently removes the file or directory tree at path, after
// verifying that path still matches the identity recorded at plan time. The
// verification permits a device-number change only across a known firmlink
// boundary (§4.6). The descent is anchored to directory file descriptors and
// uses O_NOFOLLOW throughout, so a mid-descent component swap is skipped and
// reported rather than followed.
func Obliterate(path string, expected identity.FileIdent) (Result, error) {
	var res Result
	clean := filepath.Clean(path)
	parent := filepath.Dir(clean)
	base := filepath.Base(clean)
	if base == "." || base == "/" || base == "" {
		return res, fmt.Errorf("fsdelete: refusing to operate on %q", path)
	}

	// Open the parent directory. O_NOFOLLOW here protects the final component
	// (base) from being a symlink we'd traverse; the parent path itself is
	// trusted only as far as the recorded identity check below.
	parentFd, err := unix.Open(parent, unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_RDONLY, 0)
	if err != nil {
		return res, fmt.Errorf("fsdelete: open parent %q: %w", parent, err)
	}
	defer unix.Close(parentFd)

	// Verify the root target's current identity matches what was planned.
	cur, err := identity.ComputeFile(clean, identity.DefaultLargeThreshold)
	if err != nil {
		return res, fmt.Errorf("fsdelete: stat target %q: %w", clean, err)
	}
	if err := identity.VerifyFile(expected, cur, firmlink.IsBoundary(clean)); err != nil {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s (%v)", clean, err))
		return res, nil
	}

	if err := removeAt(parentFd, base, &res); err != nil {
		return res, err
	}
	return res, nil
}

// removeAt removes name within the directory referenced by dirFd, recursing
// into directories with no-follow openat and unlinking in post-order. Errors
// that indicate a swap or a vanished/replaced entry are recorded as skips
// rather than aborting the whole operation.
func removeAt(dirFd int, name string, res *Result) error {
	var st unix.Stat_t
	if err := unix.Fstatat(dirFd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil // already gone
		}
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s (stat: %v)", name, err))
		return nil
	}

	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		// Regular file, symlink, socket, etc. — unlink directly (no follow).
		if err := unix.Unlinkat(dirFd, name, 0); err != nil && !errors.Is(err, unix.ENOENT) {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s (unlink: %v)", name, err))
			return nil
		}
		res.Removed++
		return nil
	}

	// Directory: open it with O_NOFOLLOW|O_DIRECTORY. If it was swapped for a
	// symlink between the fstatat above and now, this open fails — we skip it
	// and never follow the redirection.
	childFd, err := unix.Openat(dirFd, name, unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_RDONLY, 0)
	if err != nil {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s (open dir: %v)", name, err))
		return nil
	}

	// Read child names from the fd, then recurse. os.NewFile takes ownership of
	// the fd, so closing the *os.File closes childFd.
	dir := os.NewFile(uintptr(childFd), name)
	names, readErr := dir.Readdirnames(-1)
	if readErr != nil {
		dir.Close()
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s (readdir: %v)", name, readErr))
		return nil
	}
	for _, child := range names {
		if err := removeAt(int(dir.Fd()), child, res); err != nil {
			dir.Close()
			return err
		}
	}
	dir.Close()

	// Post-order: the directory is now (best-effort) empty; remove it.
	if err := unix.Unlinkat(dirFd, name, unix.AT_REMOVEDIR); err != nil && !errors.Is(err, unix.ENOENT) {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s (rmdir: %v)", name, err))
		return nil
	}
	res.Removed++
	return nil
}
