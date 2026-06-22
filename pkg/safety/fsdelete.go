package safety

// fsdelete performs fd-anchored recursive deletion — used only in obliterate
// mode (§4.6). It descends with openat(parentfd, name,
// O_NOFOLLOW|O_DIRECTORY|O_CLOEXEC) relative to directory file descriptors,
// never re-resolving full paths, so a concurrent directory-to-symlink swap of a
// higher component cannot redirect the descent. Each entry is fstatat'd with
// no-follow semantics, its device+inode verified against the planned identity,
// and removed with unlinkat in post-order. A swapped or replaced component is
// skipped and reported — deletion can never escape the intended subtree.
//
// In trash mode the fd-anchored walker is NOT used: the approved root is moved
// atomically as a unit by pkg/trash.

// TODO(phase0): Obliterate(rootfd, name, identity) descending with openat/
// unlinkat in post-order; skip-and-report on identity/no-follow failure.
