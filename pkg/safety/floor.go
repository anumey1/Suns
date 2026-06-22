package safety

// DenyFloor is the versioned manifest of paths nothing in the program may ever
// delete, enforced at plan time, execution time, and every directory descent
// (§5.3, §13.7). No flag, no mode, no config value, and no scheduled job can
// bypass it — confirm_mode=on and deletion_mode=obliterate do not override it.
//
// It covers /System, /usr (except /usr/local), /bin, /sbin, all SIP-protected
// paths, /Library/Apple, anything on a System Volume, the user's home root
// itself (only specific subpaths are ever targeted, never ~ directly), and
// iCloud-synced containers by default.
type DenyFloor struct{}

// TODO(phase0): Permits(path string) bool, checked at plan/exec/descent.
