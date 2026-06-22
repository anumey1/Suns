package safety

// firmlink provides firmlink-aware identity handling (§4.6). macOS binds the
// read-only System volume and the writable Data volume with firmlinks (e.g.
// /var → /private/var, /Users → /System/Volumes/Data/Users). Firmlinks are NOT
// symlinks, so O_NOFOLLOW does not reject them — but crossing one changes the
// device number, which would otherwise trip the device+inode identity check.
//
// This layer detects OS-synthesized firmlink boundaries (via statfs/MNT_*
// flags and a known-firmlink map) and permits the device-number change across
// exactly those boundaries, while still rejecting user-space symlink
// redirections. The allowance is constrained to the known OS map; it is not a
// general "ignore device changes" escape hatch.

// TODO(phase0): IsFirmlinkBoundary(path) and the known-firmlink map.
