package safety

// Revalidate performs identity-aware execution validation (§4.7). It is tiered
// for files (large files by size+mtime+inode, small high-risk files by full
// SHA-256, all hashing backgrounded and cancellable) and privilege-correct for
// processes (ProcessKill validation is delegated to the privilege chokepoint,
// which validates and signals atomically under elevation to defeat PID reuse).
//
// The deny floor and firmlink-aware anchoring are also rechecked here.
type Revalidate struct{}

// TODO(phase0): per-kind ValidateAtExec returning operation.Identity.
