package dedup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/floor"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

// headProbe is the cheap discriminator size hashed in pass 2 before any full
// content hash is computed (§12.1).
const headProbe = 4 << 10 // 4 KB

// bundleExts are directory extensions treated atomically: their interior files
// are never deduped, because a bundle presents to the user as a single document
// (§12.1).
var bundleExts = map[string]bool{
	".app":           true,
	".rtfd":          true,
	".photoslibrary": true,
	".bundle":        true,
	".framework":     true,
	".plugin":        true,
	".kext":          true,
}

// cosmeticXattrs are extended attributes that are metadata, not content. Data-
// fork hashing already disregards them, so two files that differ only in these
// are still genuine duplicates; we read them solely to surface an informational
// note when the duplicates differ cosmetically (§12.1).
var cosmeticXattrs = []string{
	"com.apple.quarantine",
	"com.apple.metadata:kMDItemWhereFroms",
	"com.apple.FinderInfo",
	"com.apple.metadata:_kMDItemUserTags",
}

// Options controls a dedup scan.
type Options struct {
	// MinSize ignores files smaller than this many bytes. Values < 1 are raised
	// to 1, so zero-byte files are never offered (nothing to reclaim).
	MinSize int64
	// Threshold is the tiered-identity size cutoff; 0 uses
	// identity.DefaultLargeThreshold (§4.7).
	Threshold int64
}

// Group is a set of byte-identical files discovered by Find.
type Group struct {
	Size        int64    // size of each member, in bytes
	Hash        string   // full SHA-256 of the shared content, hex
	Keeper      string   // the member retained (chosen by the keeper heuristic)
	Deletable   []string // the other members, offered for deletion (sorted)
	XattrDiffer bool     // members differ in cosmetic xattrs (informational only)
}

// Report is the read-only outcome of Find. It performs no deletion.
type Report struct {
	Groups []Group
	// Ops is one FileDeleteOp per deletable member, ready for plan.New — the same
	// op kind the gate, history, and restore already handle.
	Ops []operation.Operation
	// ReclaimableEst is the nominal reclaimable total (Σ size·deletable). It is an
	// upper bound: see CloneCaveat.
	ReclaimableEst int64
	// CloneCaveat is set when the report contains deletions and therefore the
	// space actually freed may be lower than nominal for any members that share
	// APFS clone blocks. Shared-block files are deliberately NOT excluded — block
	// sharing does not prove clone lineage, and excluding them would skip real
	// duplicates (§12.1).
	CloneCaveat bool
}

// Find scans roots for byte-identical duplicate files in three passes — group by
// exact size, then by a 4 KB head hash, then by full SHA-256 — and returns the
// groups plus a FileDeleteOp per deletable member. It is read-only and honors
// ctx cancellation. Hardlinks (same dev+inode) are collapsed to one
// representative and never offered; symlinks are never followed; bundle interiors
// are never descended; floor-protected paths are skipped defensively.
func Find(ctx context.Context, roots []string, opts Options) (Report, error) {
	threshold := opts.Threshold
	if threshold == 0 {
		threshold = identity.DefaultLargeThreshold
	}

	files, err := gather(ctx, roots, opts)
	if err != nil {
		return Report{}, err
	}

	// Pass 1: group by exact byte size.
	bySize := map[int64][]fileInfo{}
	for _, f := range files {
		bySize[f.size] = append(bySize[f.size], f)
	}

	var rep Report
	for size, sizeGroup := range bySize {
		if len(sizeGroup) < 2 {
			continue
		}
		// Pass 2: hash the first 4 KB as a cheap discriminator.
		byHead := map[string][]fileInfo{}
		for _, f := range sizeGroup {
			select {
			case <-ctx.Done():
				return Report{}, ctx.Err()
			default:
			}
			head, herr := hashHead(f.path, headProbe)
			if herr != nil {
				continue // unreadable → drop from candidacy
			}
			byHead[head] = append(byHead[head], f)
		}
		// Pass 3: full SHA-256 only on the survivors.
		for _, headGroup := range byHead {
			if len(headGroup) < 2 {
				continue
			}
			byFull := map[string][]fileInfo{}
			for _, f := range headGroup {
				full, ferr := hashFull(f.path)
				if ferr != nil {
					continue
				}
				byFull[full] = append(byFull[full], f)
			}
			for full, dupes := range byFull {
				if len(dupes) < 2 {
					continue
				}
				rep.Groups = append(rep.Groups, buildGroup(size, full, dupes))
			}
		}
	}

	// Deterministic order: largest groups first, then by keeper path.
	sort.Slice(rep.Groups, func(i, j int) bool {
		if rep.Groups[i].Size != rep.Groups[j].Size {
			return rep.Groups[i].Size > rep.Groups[j].Size
		}
		return rep.Groups[i].Keeper < rep.Groups[j].Keeper
	})

	for _, g := range rep.Groups {
		for _, p := range g.Deletable {
			id, ierr := identity.ComputeFile(p, threshold)
			if ierr != nil {
				continue // vanished/unreadable since the scan → do not offer
			}
			rep.Ops = append(rep.Ops, operation.FileDeleteOp{
				Path:     p,
				Size:     g.Size,
				Category: "duplicate",
				Expected: id,
			})
			rep.ReclaimableEst += g.Size
		}
	}
	rep.CloneCaveat = len(rep.Ops) > 0
	return rep, nil
}

// fileInfo is a gathered duplicate candidate.
type fileInfo struct {
	path  string
	size  int64
	dev   uint64
	inode uint64
}

// gather walks roots collecting regular-file candidates, applying the no-follow,
// bundle-atomic, hardlink-collapse, min-size, and floor filters.
func gather(ctx context.Context, roots []string, opts Options) ([]fileInfo, error) {
	minSize := opts.MinSize
	if minSize < 1 {
		minSize = 1 // never consider zero-byte files
	}
	seenInode := map[[2]uint64]bool{}
	var files []fileInfo

	for _, root := range roots {
		root = filepath.Clean(root)
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			if walkErr != nil {
				return nil // tolerate unreadable entries (TCC, permissions, races)
			}
			if d.IsDir() {
				if isBundle(path) {
					return filepath.SkipDir // atomic: never dedup bundle interiors
				}
				return nil
			}
			if d.Type()&fs.ModeSymlink != 0 {
				return nil // never follow or offer symlinks
			}
			if !d.Type().IsRegular() {
				return nil
			}
			info, ierr := d.Info()
			if ierr != nil || info.Size() < minSize {
				return nil
			}
			if floor.Check(path) != nil {
				return nil // defensive: never offer a floor-protected path
			}
			dev, inode := devInode(info)
			key := [2]uint64{dev, inode}
			if seenInode[key] {
				return nil // hardlink to an already-collected file → the same file
			}
			seenInode[key] = true
			files = append(files, fileInfo{path: path, size: info.Size(), dev: dev, inode: inode})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}

// buildGroup assembles a duplicate group from its members, choosing the keeper
// and detecting cosmetic-xattr differences.
func buildGroup(size int64, hash string, dupes []fileInfo) Group {
	paths := make([]string, len(dupes))
	for i, d := range dupes {
		paths[i] = d.path
	}
	sort.Strings(paths)
	keeper := pickKeeper(paths)
	deletable := make([]string, 0, len(paths)-1)
	for _, p := range paths {
		if p != keeper {
			deletable = append(deletable, p)
		}
	}
	return Group{
		Size:        size,
		Hash:        hash,
		Keeper:      keeper,
		Deletable:   deletable,
		XattrDiffer: cosmeticXattrsDiffer(paths),
	}
}

// pickKeeper chooses the member to retain, preferring user-document locations and
// deprioritizing caches/downloads/temp/trash. Ties break toward the shallower
// then lexically-first path, so selection is deterministic. It never spans
// bundles (interior files are excluded upstream).
func pickKeeper(paths []string) string {
	best := paths[0]
	for _, p := range paths[1:] {
		if betterKeeper(p, best) {
			best = p
		}
	}
	return best
}

func betterKeeper(a, b string) bool {
	sa, sb := keeperScore(a), keeperScore(b)
	if sa != sb {
		return sa > sb
	}
	da, db := strings.Count(a, string(os.PathSeparator)), strings.Count(b, string(os.PathSeparator))
	if da != db {
		return da < db
	}
	return a < b
}

// keeperScore ranks a location: higher is a better keeper.
func keeperScore(p string) int {
	lp := strings.ToLower(p)
	score := 0
	switch {
	case containsSeg(lp, "documents"):
		score += 3
	case containsSeg(lp, "desktop"):
		score += 2
	}
	if containsSeg(lp, "caches") || containsSeg(lp, "cache") {
		score -= 3
	}
	if containsSeg(lp, "downloads") {
		score -= 2
	}
	if containsSeg(lp, ".trash") || strings.HasPrefix(lp, "/tmp/") || strings.HasPrefix(lp, "/private/tmp/") || strings.HasPrefix(lp, "/var/folders/") {
		score -= 4
	}
	return score
}

// containsSeg reports whether lpath contains seg as a whole path segment.
func containsSeg(lpath, seg string) bool {
	return strings.Contains(lpath, "/"+seg+"/") || strings.HasSuffix(lpath, "/"+seg)
}

// cosmeticXattrsDiffer reports whether the members differ in any cosmetic xattr
// (presence or value). The content is identical by construction; this only flags
// an informational note.
func cosmeticXattrsDiffer(paths []string) bool {
	for _, attr := range cosmeticXattrs {
		var first []byte
		var firstHave bool
		for i, p := range paths {
			v, have := readXattr(p, attr)
			if i == 0 {
				first, firstHave = v, have
				continue
			}
			if have != firstHave || !bytes.Equal(v, first) {
				return true
			}
		}
	}
	return false
}

func readXattr(path, attr string) ([]byte, bool) {
	sz, err := unix.Getxattr(path, attr, nil)
	if err != nil || sz <= 0 {
		return nil, false
	}
	buf := make([]byte, sz)
	n, err := unix.Getxattr(path, attr, buf)
	if err != nil {
		return nil, false
	}
	return buf[:n], true
}

func isBundle(path string) bool {
	return bundleExts[strings.ToLower(filepath.Ext(path))]
}

func hashHead(path string, n int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, n); err != nil && err != io.EOF {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFull(path string) (string, error) {
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
