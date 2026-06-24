package operation

import (
	"context"
	"fmt"
	"time"
)

// ReceiptForgetOp forgets an installer receipt via `pkgutil --forget <id>`
// (§12.15-uninstaller). It is the LAST step of a .pkg teardown: it runs only
// after the payload it accounted for has been turned into FileDelete/
// ServiceUnload ops, so forgetting the receipt cannot orphan live files.
//
// Reversibility is Irreversible (🔴): the receipt is gone and the package is no
// longer known to pkgutil. The op is a pure value type for plan.Seal.
type ReceiptForgetOp struct {
	PackageID string // installed receipt id, e.g. com.vendor.app.pkg
}

var _ Operation = ReceiptForgetOp{}

func (o ReceiptForgetOp) Kind() OpKind { return KindReceiptForget }

func (o ReceiptForgetOp) Reversibility() Reversibility { return Irreversible }

func (o ReceiptForgetOp) Describe() Preview {
	return Preview{
		Kind:          KindReceiptForget,
		Reversibility: Irreversible,
		Line:          fmt.Sprintf("pkgutil --forget %s", o.PackageID),
	}
}

// ValidateAtPlan confirms the receipt is currently installed (an unprivileged
// read), so the plan never offers to forget a package that is already gone.
func (o ReceiptForgetOp) ValidateAtPlan(ctx context.Context) error {
	if !o.installed(ctx) {
		return fmt.Errorf("operation: package %q is not installed", o.PackageID)
	}
	return nil
}

// ValidateAtExec re-confirms the receipt is still installed just before acting.
// The identity for a receipt is simply "this package ID is still installed"
// (§6, op table); there is no file/process/service sub-identity to populate.
func (o ReceiptForgetOp) ValidateAtExec(ctx context.Context) (Identity, error) {
	if !o.installed(ctx) {
		return Identity{}, fmt.Errorf("operation: package %q no longer installed", o.PackageID)
	}
	return Identity{Kind: KindReceiptForget}, nil
}

// Execute runs `pkgutil --forget <id>` through the privileged system runner. The
// deletion mode is inert for this op.
func (o ReceiptForgetOp) Execute(ctx context.Context, _ Mode, _ Identity) (Receipt, error) {
	r := Receipt{Kind: KindReceiptForget, Time: time.Now()}
	res, err := getSystemRunner().Run(ctx, true, "pkgutil", "--forget", o.PackageID)
	switch {
	case err == nil && res.ExitCode == 0:
		r.Fate, r.Status = "forgotten", "ok"
	case err == ErrPrivilegeRequired:
		r.Fate, r.Status = "skipped", "skipped:no-privilege"
	default:
		r.Fate, r.Status, r.Err = "skipped", "failed", err
	}
	return r, nil
}

func (o ReceiptForgetOp) HistoryRecord(r Receipt) HistoryEntry {
	return HistoryEntry{
		TS:         r.Time,
		Op:         KindReceiptForget,
		Reversible: Irreversible,
		Status:     r.Status,
		Cmd:        fmt.Sprintf("pkgutil --forget %s", o.PackageID),
		PackageID:  o.PackageID,
	}
}

// installed reports whether pkgutil still knows the receipt (unprivileged read).
func (o ReceiptForgetOp) installed(ctx context.Context) bool {
	res, err := getSystemRunner().Run(ctx, false, "pkgutil", "--pkg-info", o.PackageID)
	return err == nil && res.ExitCode == 0
}
