package operation

import (
	"context"
	"time"
)

// DNSFlushOp flushes the system DNS resolver cache and signals mDNSResponder to
// reload (§12.11). It is a pure value type so plan.Seal copies it without
// aliasing, and it carries no target path — a cache flush has no file identity.
//
// Reversibility is Irreversible (🔴): there is nothing to "undo", though the
// action is harmless — the only cost is that the next handful of lookups are
// marginally slower as the cache repopulates. Because it is a *reset* and not a
// deletion, the Jarjar deletion Mode axis is inert for it. It requires root,
// acquired once through the privilege chokepoint behind the injected runner.
type DNSFlushOp struct{}

var _ Operation = DNSFlushOp{}

func (o DNSFlushOp) Kind() OpKind { return KindDNSFlush }

func (o DNSFlushOp) Reversibility() Reversibility { return Irreversible }

func (o DNSFlushOp) Describe() Preview {
	return Preview{
		Kind:          KindDNSFlush,
		Reversibility: Irreversible,
		Line:          "flush DNS cache + restart mDNSResponder  (requires admin)",
	}
}

// ValidateAtPlan has nothing to check: a cache flush has no path or volume target
// and is always sensible to plan.
func (o DNSFlushOp) ValidateAtPlan(ctx context.Context) error { return nil }

// ValidateAtExec returns the trivial flush identity. There is no target whose
// identity could drift between plan and exec, so this never refuses.
func (o DNSFlushOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	return Identity{Kind: KindDNSFlush}, nil
}

// Execute runs `dscacheutil -flushcache` then `killall -HUP mDNSResponder`
// through the injected system runner, both elevated. A missing privilege-capable
// runner (or declined elevation) yields a skipped receipt rather than a hard
// failure, so a workflow that loses privilege degrades gracefully (§6.1). The
// deletion mode is inert for this op.
func (o DNSFlushOp) Execute(ctx context.Context, _ Mode, _ Identity) (Receipt, error) {
	r := Receipt{Kind: KindDNSFlush, Time: time.Now()}
	runner := getSystemRunner()

	// Flush the resolver cache.
	if res, err := runner.Run(ctx, true, "dscacheutil", "-flushcache"); err != nil || res.ExitCode != 0 {
		return flushFailed(r, err), nil
	}
	// Signal mDNSResponder to reload, completing the flush.
	if res, err := runner.Run(ctx, true, "killall", "-HUP", "mDNSResponder"); err != nil || res.ExitCode != 0 {
		return flushFailed(r, err), nil
	}

	r.Fate, r.Status = "flushed", "ok"
	return r, nil
}

func (o DNSFlushOp) HistoryRecord(r Receipt) HistoryEntry {
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindDNSFlush,
		Reversible: Irreversible,
		Status:     r.Status,
		Fate:       r.Fate,
		Cmd:        "dscacheutil -flushcache; killall -HUP mDNSResponder",
	}
}

// flushFailed maps a run error to a skipped receipt: a missing privilege is a
// clean skip (the workflow simply lost elevation), anything else is a failure.
func flushFailed(r Receipt, err error) Receipt {
	if err == ErrPrivilegeRequired {
		r.Fate, r.Status = "skipped", "skipped:no-privilege"
	} else {
		r.Fate, r.Status, r.Err = "skipped", "failed", err
	}
	return r
}
