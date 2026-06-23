// Package trash implements bounded-and-degrading native Trash (§4.4).
//
// The correct way to move a file to the macOS Trash is the Cocoa call
// NSFileManager trashItemAtURL:, reached via Cgo. But every Cgo call pins an OS
// thread; a hung call (dead SMB mount, stalled FSEvents) loses that thread to
// the Go runtime, and enough of them crash the program. This package therefore
// layers, around the native call:
//
//  1. a volume-reachability probe (see probe.go),
//  2. a concurrency semaphore capping in-flight native calls,
//  3. a circuit breaker that, after K consecutive timeouts, DEGRADES to a
//     pure-Go fallback (rename within the volume, or copy-then-unlink across
//     volumes) — it never refuses and never silently permanent-deletes.
//
// The fallback uses Suns's own ~/.Trash and has no OS "Put Back" metadata,
// which is acceptable because restore uses the operation-history log, not OS
// Put-Back (§4.8). A healthy local volume always succeeds even while a dead
// mount has tripped the breaker.
//
// Phase 0 status: the pure-Go fallback, the semaphore, and the circuit breaker
// are implemented and tested. The native Cgo NSFileManager call and the
// killable-helper volume probe are staged behind tryNative/probe.go and must be
// completed and validated on-device (the §15 hung-volume integration test)
// before the native path is enabled. Until then every trash degrades to the
// proven pure-Go fallback.
package trash

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Method records how an item reached the Trash.
type Method string

const (
	MethodNative     Method = "native"      // NSFileManager trashItemAtURL: (Cgo)
	MethodRename     Method = "rename"      // pure-Go os.Rename within the volume
	MethodCopyUnlink Method = "copy-unlink" // pure-Go cross-volume fallback (EXDEV)
)

// Result is the outcome of trashing one item.
type Result struct {
	Path      string
	Method    Method
	TrashPath string
	Skipped   bool
	Reason    string
}

// Trasher moves items to the Trash, bounding and degrading the native call. It
// holds the per-run circuit-breaker and concurrency state, so create one per
// workflow and reuse it across the batch.
type Trasher struct {
	trashDir      string
	sem           chan struct{}
	breaker       *breaker
	nativeTimeout time.Duration
}

// New returns a Trasher targeting the user's ~/.Trash with default bounds.
func New() (*Trasher, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return NewWithDir(filepath.Join(home, ".Trash"))
}

// NewWithDir returns a Trasher targeting an explicit Trash directory (used by
// tests). The directory is created if missing.
func NewWithDir(dir string) (*Trasher, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	const maxInFlight = 4 // cap simultaneously pinned OS threads (§4.4)
	return &Trasher{
		trashDir:      dir,
		sem:           make(chan struct{}, maxInFlight),
		breaker:       newBreaker(3),
		nativeTimeout: 3 * time.Second,
	}, nil
}

// Trash moves path to the Trash. It attempts the bounded native call when the
// breaker permits, and otherwise (or on native failure) degrades to the pure-Go
// fallback. It never permanently deletes as a fallback.
func (t *Trasher) Trash(ctx context.Context, path string) Result {
	if _, err := os.Lstat(path); err != nil {
		return Result{Path: path, Skipped: true, Reason: fmt.Sprintf("stat: %v", err)}
	}

	if t.breaker.allow() {
		err := t.tryNative(ctx, path)
		switch {
		case err == nil:
			t.breaker.recordSuccess()
			return Result{Path: path, Method: MethodNative, TrashPath: filepath.Join(t.trashDir, filepath.Base(path))}
		case errors.Is(err, errNativeTimeout):
			t.breaker.recordTimeout()
			// fall through to degrade
		case errors.Is(err, errNativeUnavailable):
			// native path not yet enabled; degrade silently
		default:
			// a definite native error; degrade rather than fail
		}
	}
	return t.fallback(path)
}

// tryNative attempts the bounded Cgo NSFileManager call under the semaphore.
// See the package doc for Phase 0 status: currently always returns
// errNativeUnavailable so callers degrade to the proven fallback.
func (t *Trasher) tryNative(ctx context.Context, path string) error {
	select {
	case t.sem <- struct{}{}:
		defer func() { <-t.sem }()
	case <-ctx.Done():
		return ctx.Err()
	}
	return nativeTrash(path, t.nativeTimeout)
}

// fallback moves path into the Trash without the Cocoa API (§4.4). It renames
// within the volume, copies-then-unlinks across volumes (EXDEV), and never
// performs a permanent delete.
func (t *Trasher) fallback(path string) Result {
	dest := uniqueDest(t.trashDir, filepath.Base(path))

	err := os.Rename(path, dest)
	if err == nil {
		return Result{Path: path, Method: MethodRename, TrashPath: dest}
	}
	if errors.Is(err, syscall.EXDEV) {
		if cerr := copyThenUnlink(path, dest); cerr != nil {
			return Result{Path: path, Skipped: true, Reason: fmt.Sprintf("cross-volume copy: %v", cerr)}
		}
		return Result{Path: path, Method: MethodCopyUnlink, TrashPath: dest}
	}
	// Any other error: skip and report. We never escalate to a permanent delete.
	return Result{Path: path, Skipped: true, Reason: fmt.Sprintf("rename: %v", err)}
}

// uniqueDest returns a destination path inside dir for base, appending a
// timestamp if base is already present (the Trash collision rule).
func uniqueDest(dir, base string) string {
	dest := filepath.Join(dir, base)
	if _, err := os.Lstat(dest); err != nil {
		return dest
	}
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	stamp := time.Now().Format("15-04-05.000")
	return filepath.Join(dir, fmt.Sprintf("%s %s%s", stem, stamp, ext))
}

// MovePath moves src to dst, renaming within a volume and falling back to
// copy-then-unlink across volumes (EXDEV). It never deletes src without a
// successful copy. It is exported for restore (§4.8), which moves an item back
// out of the Trash using the same rename-or-copy mechanics.
func MovePath(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EXDEV) {
		return copyThenUnlink(src, dst)
	}
	return err
}

// copyThenUnlink recursively copies src to dst, then removes src. It is used
// only for the cross-volume (EXDEV) fallback, where os.Rename cannot work.
func copyThenUnlink(src, dst string) error {
	if err := copyPath(src, dst); err != nil {
		// Clean up a partial copy; leave the source intact (never delete it on
		// a failed copy).
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}

func copyPath(src, dst string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case fi.IsDir():
		if err := os.MkdirAll(dst, fi.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		return copyFile(src, dst, fi.Mode().Perm())
	}
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
