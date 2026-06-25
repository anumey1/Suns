package purge

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// dsStore is the Finder metadata file that, per §12.19, does not by itself keep
// a directory from counting as empty — a directory whose only content is a
// .DS_Store is treated as empty and removed along with it.
const dsStore = ".DS_Store"

// Kind labels a finding for rendering and JSON.
const (
	KindEmptyDir      = "empty-dir"
	KindBrokenSymlink = "broken-symlink"
)

// Finding is one discovered target, for the read-only audit view.
type Finding struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

// Result is the outcome of a discovery pass.
type Result struct {
	Ops      []operation.Operation `json:"-"`
	Findings []Finding             `json:"findings"`
	Skipped  []string              `json:"skipped,omitempty"`
}

// Options controls discovery.
type Options struct {
	Threshold int64 // tiered-identity threshold; 0 → identity.DefaultLargeThreshold
}

func (o Options) threshold() int64 {
	if o.Threshold == 0 {
		return identity.DefaultLargeThreshold
	}
	return o.Threshold
}

// EmptyDirs discovers empty directories under root and emits one FileDeleteOp per
// maximal empty subtree (trashing it removes the whole subtree at once). A
// directory counts as empty when every direct entry is either a .DS_Store or a
// subdirectory that is itself empty — computed bottom-up so a directory emptied
// by removing its children is caught in the same pass (§12.19). The named root is
// never itself removed; symlinks are never followed and make a directory
// non-empty. It is read-only and honors ctx cancellation.
func EmptyDirs(ctx context.Context, root string, opts Options) (Result, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}
	fi, err := os.Lstat(abs)
	if err != nil {
		return Result{}, err
	}
	if !fi.IsDir() {
		return Result{}, fmt.Errorf("purge: %s is not a directory", abs)
	}

	var res Result

	// Collect every descendant directory, then process deepest-first so a child's
	// collapsibility is known before its parent is evaluated.
	var dirs []string
	err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %v", p, walkErr))
			return nil // tolerate unreadable subtrees
		}
		if d.IsDir() && p != abs {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], string(os.PathSeparator)) > strings.Count(dirs[j], string(os.PathSeparator))
	})

	// collapsible[d] is true when d is empty-or-only-.DS_Store-and-collapsible.
	collapsible := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		collapsible[d] = isCollapsible(d, collapsible, &res)
	}

	// Emit only maximal collapsible dirs: those whose parent is not itself
	// collapsible (a collapsible parent already subsumes this subtree).
	for _, d := range dirs {
		if !collapsible[d] || collapsible[filepath.Dir(d)] {
			continue
		}
		if err := floor.Check(d); err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: floor-protected, refusing", d))
			continue
		}
		id, err := identity.ComputeFile(d, opts.threshold())
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: identity: %v", d, err))
			continue
		}
		res.Ops = append(res.Ops, operation.FileDeleteOp{Path: d, Category: KindEmptyDir, Expected: id})
		res.Findings = append(res.Findings, Finding{Path: d, Kind: KindEmptyDir})
	}
	return res, nil
}

// isCollapsible reports whether dir contains nothing but a .DS_Store and/or
// subdirectories already known to be collapsible. An unreadable directory is
// treated as non-collapsible (we cannot prove it empty) and noted as skipped.
func isCollapsible(dir string, collapsible map[string]bool, res *Result) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %v", dir, err))
		return false
	}
	for _, e := range entries {
		switch {
		case e.IsDir():
			if !collapsible[filepath.Join(dir, e.Name())] {
				return false
			}
		case e.Name() == dsStore && e.Type().IsRegular():
			// A lone .DS_Store does not keep the directory from being empty.
		default:
			return false // a regular file, symlink, or anything else: not empty
		}
	}
	return true
}

// BrokenSymlinks discovers dangling symlinks under the given roots and emits a
// FileDeleteOp (🟢) for each. Walks are no-follow; a symlink is dangling when
// stat-ing it (which follows the link) reports the target does not exist. Other
// stat errors (e.g. an unreachable volume) are left alone rather than removed. It
// is read-only and honors ctx cancellation.
func BrokenSymlinks(ctx context.Context, roots []string, opts Options) (Result, error) {
	var res Result
	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %v", root, err))
			continue
		}
		walkErr := filepath.WalkDir(abs, func(p string, d fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s: %v", p, walkErr))
				return nil
			}
			if d.Type()&fs.ModeSymlink == 0 {
				return nil
			}
			if _, err := os.Stat(p); err == nil || !errors.Is(err, fs.ErrNotExist) {
				return nil // resolves, or fails for a reason other than a missing target
			}
			if err := floor.Check(p); err != nil {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s: floor-protected, refusing", p))
				return nil
			}
			id, err := identity.ComputeFile(p, opts.threshold())
			if err != nil {
				res.Skipped = append(res.Skipped, fmt.Sprintf("%s: identity: %v", p, err))
				return nil
			}
			target, _ := os.Readlink(p)
			res.Ops = append(res.Ops, operation.FileDeleteOp{Path: p, Category: KindBrokenSymlink, Expected: id})
			res.Findings = append(res.Findings, Finding{Path: p, Kind: KindBrokenSymlink, Detail: "→ " + target})
			return nil
		})
		if walkErr != nil {
			return res, walkErr
		}
	}
	return res, nil
}
