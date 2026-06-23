// Package identity holds the tiered, execution-time identity primitives used to
// prove a target is still exactly what was planned (§4.7).
//
// Files are identified in a tiered way so verification never blocks on huge
// objects: files above a size threshold (default 50 MB) are identified by
// size + mtime + inode (cheap, no full read), while small files get a full
// SHA-256 content hash. Process and service identities are value types
// populated by their respective engines in later phases.
//
// This is a leaf package: it imports only the standard library, so the
// Operation model and the higher-level safety package can both depend on it
// without an import cycle.
package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

// DefaultLargeThreshold is the size at or above which a file is identified by
// metadata (size+mtime+inode) rather than a full content hash (§4.7).
const DefaultLargeThreshold int64 = 50 << 20 // 50 MB

// Tier records which identification strategy was used for a file.
type Tier string

const (
	TierLarge Tier = "large" // size + mtime + inode
	TierSmall Tier = "small" // full content hash
)

// FileIdent is the recorded identity of a file target.
type FileIdent struct {
	Tier  Tier      `json:"tier"`
	Size  int64     `json:"size"`
	Mtime time.Time `json:"mtime"`
	Dev   uint64    `json:"dev"`
	Inode uint64    `json:"inode"`
	Hash  string    `json:"hash,omitempty"` // hex SHA-256, only for the small tier
}

// ProcessIdent is the recorded identity of a process target (PID reuse defense,
// §4.7). Populated by the process engine in Phase 1.
type ProcessIdent struct {
	PID   int       `json:"pid"`
	Birth time.Time `json:"birth"`
	Exec  string    `json:"exec"`
}

// ServiceIdent is the recorded identity of a launchd service target.
type ServiceIdent struct {
	Domain string `json:"domain"`
	Label  string `json:"label"`
	Plist  string `json:"plist"`
}

// ErrIdentityMismatch is returned by VerifyFile when the current identity does
// not match the one recorded at plan time (the target was renamed, replaced,
// or modified).
var ErrIdentityMismatch = errors.New("identity: target changed since it was planned")

// ComputeFile reads the identity of the file at path using no-follow semantics,
// applying the tiered strategy with the given threshold (use
// DefaultLargeThreshold for the default). A threshold <= 0 means "always hash".
func ComputeFile(path string, threshold int64) (FileIdent, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return FileIdent{}, err
	}
	dev, inode := devInode(fi)
	id := FileIdent{
		Size:  fi.Size(),
		Mtime: fi.ModTime(),
		Dev:   dev,
		Inode: inode,
	}
	// Directories and large regular files use the metadata tier. Small regular
	// files are hashed. Symlinks are never followed and are treated as small
	// (their identity is metadata only; there is nothing to hash safely).
	if fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 || (threshold > 0 && fi.Size() >= threshold) {
		id.Tier = TierLarge
		return id, nil
	}
	h, err := hashFile(path)
	if err != nil {
		return FileIdent{}, err
	}
	id.Tier = TierSmall
	id.Hash = h
	return id, nil
}

// VerifyFile checks that current matches expected. When allowDevChange is true
// (an OS-synthesized firmlink boundary, decided by the caller via the firmlink
// package, §4.6) a differing device number is permitted; the inode and the
// rest of the identity must still match.
func VerifyFile(expected, current FileIdent, allowDevChange bool) error {
	if expected.Tier != current.Tier {
		return fmt.Errorf("%w: tier %q != %q", ErrIdentityMismatch, current.Tier, expected.Tier)
	}
	if !allowDevChange && expected.Dev != current.Dev {
		return fmt.Errorf("%w: device %d != %d", ErrIdentityMismatch, current.Dev, expected.Dev)
	}
	if expected.Inode != current.Inode {
		return fmt.Errorf("%w: inode %d != %d", ErrIdentityMismatch, current.Inode, expected.Inode)
	}
	if expected.Size != current.Size {
		return fmt.Errorf("%w: size %d != %d", ErrIdentityMismatch, current.Size, expected.Size)
	}
	switch expected.Tier {
	case TierLarge:
		if !expected.Mtime.Equal(current.Mtime) {
			return fmt.Errorf("%w: mtime changed", ErrIdentityMismatch)
		}
	case TierSmall:
		if expected.Hash != current.Hash {
			return fmt.Errorf("%w: content hash changed", ErrIdentityMismatch)
		}
	}
	return nil
}

// VerifyContent checks that current still has the same CONTENT as expected,
// ignoring device and inode. It is used by restore (§4.8): trashing can move a
// file across volumes (changing dev+inode legitimately), so a restore must
// detect tampering by content — size+mtime for large objects, hash for small —
// not by inode identity. A mismatch means the Trash entry was modified or
// swapped after deletion.
func VerifyContent(expected, current FileIdent) error {
	if expected.Tier != current.Tier {
		return fmt.Errorf("%w: tier %q != %q", ErrIdentityMismatch, current.Tier, expected.Tier)
	}
	if expected.Size != current.Size {
		return fmt.Errorf("%w: size %d != %d", ErrIdentityMismatch, current.Size, expected.Size)
	}
	switch expected.Tier {
	case TierLarge:
		if !expected.Mtime.Equal(current.Mtime) {
			return fmt.Errorf("%w: mtime changed", ErrIdentityMismatch)
		}
	case TierSmall:
		if expected.Hash != current.Hash {
			return fmt.Errorf("%w: content hash changed", ErrIdentityMismatch)
		}
	}
	return nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func devInode(fi os.FileInfo) (dev, inode uint64) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0
	}
	return uint64(st.Dev), st.Ino
}
