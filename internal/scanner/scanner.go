package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/anumey1/Suns/assets"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// RebuildCost tags how expensive a cache is to regenerate (§13.5). "Reclaimable"
// is not the same as "cheap to rebuild".
type RebuildCost string

const (
	CostCheap     RebuildCost = "cheap"
	CostModerate  RebuildCost = "moderate"
	CostExpensive RebuildCost = "expensive"
)

// Target is one entry in the curated safe-cache allowlist (§5.3, §12.16).
type Target struct {
	ID          string      `json:"id"`
	Path        string      `json:"path"` // may contain a leading ~
	Category    string      `json:"category"`
	RebuildCost RebuildCost `json:"rebuild_cost"`
	OptIn       bool        `json:"opt_in"`
	Warning     string      `json:"warning,omitempty"`
}

// Manifest is the embedded safe-cache allowlist.
type Manifest struct {
	Version int      `json:"version"`
	Targets []Target `json:"targets"`
}

// LoadSafeCacheManifest parses the embedded safe-cache allowlist (§8).
func LoadSafeCacheManifest() (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(assets.SafeCacheManifest, &m); err != nil {
		return Manifest{}, fmt.Errorf("scanner: parsing safe-cache manifest: %w", err)
	}
	return m, nil
}

// Options controls discovery.
type Options struct {
	IncludeOptIn bool  // include expensive/disruptive opt-in targets (e.g. iOS DeviceSupport)
	Threshold    int64 // tiered-identity threshold; 0 → identity.DefaultLargeThreshold
}

// Found describes a target that exists and will be cleaned.
type Found struct {
	Target Target
	Op     operation.FileDeleteOp
}

// Result is the outcome of discovery.
type Result struct {
	Ops     []operation.Operation // the complete set, ready for plan.New (not a paged view)
	Found   []Found
	Skipped []string // missing or unreadable targets, with reasons
}

// Discover stats each target and builds a FileDeleteOp for those that exist,
// computing the reclaimable size and the tiered identity recorded at plan time.
// It is read-only (no deletion) and honors ctx cancellation between targets.
// Opt-in targets are included only when Options.IncludeOptIn is set. A target on
// the deny floor is skipped defensively (the manifest should never contain one).
func Discover(ctx context.Context, targets []Target, opts Options) (Result, error) {
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = identity.DefaultLargeThreshold
	}
	home, _ := os.UserHomeDir()

	var res Result
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		if t.OptIn && !opts.IncludeOptIn {
			continue
		}
		path := expandTilde(t.Path, home)

		if err := floor.Check(path); err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s (%s): floor-protected, refusing", t.ID, path))
			continue
		}
		fi, err := os.Lstat(path)
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s (%s): %v", t.ID, path, err))
			continue
		}

		var size int64
		if fi.IsDir() {
			size = dirSize(ctx, path)
		} else {
			size = fi.Size()
		}
		if size == 0 {
			// Nothing to reclaim; not worth offering.
			continue
		}

		id, err := identity.ComputeFile(path, threshold)
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("%s (%s): identity: %v", t.ID, path, err))
			continue
		}
		op := operation.FileDeleteOp{Path: path, Size: size, Category: t.Category, Expected: id}
		res.Ops = append(res.Ops, op)
		res.Found = append(res.Found, Found{Target: t, Op: op})
	}
	return res, nil
}

// dirSize sums the sizes of regular files beneath dir (read-only). Per-entry
// errors (permission, vanished) are tolerated so a partial tree still yields a
// useful estimate; TCC denials surface as a smaller number, never a crash.
func dirSize(ctx context.Context, dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.Type().IsRegular() {
			if info, ierr := d.Info(); ierr == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

// expandTilde resolves a leading ~ to the home directory.
func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") && home != "" {
		return filepath.Join(home, path[2:])
	}
	return path
}
