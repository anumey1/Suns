package operation_test

import (
	"context"
	"testing"

	"github.com/anumey1/Suns/pkg/operation"
)

// pkgHandler builds a SystemRunner handler that reports a package as installed
// (or not) for --pkg-info and returns the given forget outcome for --forget.
func pkgHandler(installed bool, forget operation.RunResult, forgetErr error) func(bool, string, []string) (operation.RunResult, error) {
	return func(_ bool, name string, args []string) (operation.RunResult, error) {
		if name == "pkgutil" && len(args) >= 1 && args[0] == "--pkg-info" {
			if installed {
				return operation.RunResult{ExitCode: 0}, nil
			}
			return operation.RunResult{ExitCode: 1}, nil
		}
		if name == "pkgutil" && len(args) >= 1 && args[0] == "--forget" {
			return forget, forgetErr
		}
		return operation.RunResult{}, nil
	}
}

func TestReceiptForget_Success(t *testing.T) {
	f := useRunner(t, pkgHandler(true, operation.RunResult{ExitCode: 0}, nil))
	op := operation.ReceiptForgetOp{PackageID: "com.vendor.app.pkg"}

	if err := op.ValidateAtPlan(context.Background()); err != nil {
		t.Fatalf("ValidateAtPlan: %v", err)
	}
	id, err := op.ValidateAtExec(context.Background())
	if err != nil || id.Kind != operation.KindReceiptForget {
		t.Fatalf("ValidateAtExec = %v, %v", id, err)
	}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, id)
	if r.Fate != "forgotten" || r.Status != "ok" {
		t.Errorf("fate/status = %q/%q, want forgotten/ok", r.Fate, r.Status)
	}
	if got := f.lastCall(); len(got) != 3 || got[0] != "pkgutil" || got[1] != "--forget" || got[2] != "com.vendor.app.pkg" {
		t.Errorf("forget call = %v", got)
	}
}

func TestReceiptForget_NotInstalled(t *testing.T) {
	useRunner(t, pkgHandler(false, operation.RunResult{}, nil))
	op := operation.ReceiptForgetOp{PackageID: "com.absent.pkg"}
	if err := op.ValidateAtPlan(context.Background()); err == nil {
		t.Error("ValidateAtPlan should fail for a package that is not installed")
	}
	if _, err := op.ValidateAtExec(context.Background()); err == nil {
		t.Error("ValidateAtExec should fail for a package that is not installed")
	}
}

func TestReceiptForget_NoPrivilegeSkips(t *testing.T) {
	useRunner(t, pkgHandler(true, operation.RunResult{}, operation.ErrPrivilegeRequired))
	op := operation.ReceiptForgetOp{PackageID: "com.vendor.app.pkg"}
	r, _ := op.Execute(context.Background(), operation.ModeTrash, operation.Identity{Kind: operation.KindReceiptForget})
	if r.Fate != "skipped" || r.Status != "skipped:no-privilege" {
		t.Errorf("fate/status = %q/%q, want skipped/skipped:no-privilege", r.Fate, r.Status)
	}
}

func TestReceiptForget_PrivilegedFlag(t *testing.T) {
	// The forget mutation must be requested with privileged=true.
	var sawPrivileged bool
	useRunner(t, func(privileged bool, name string, args []string) (operation.RunResult, error) {
		if name == "pkgutil" && len(args) >= 1 && args[0] == "--forget" {
			sawPrivileged = privileged
		}
		return operation.RunResult{ExitCode: 0}, nil
	})
	op := operation.ReceiptForgetOp{PackageID: "com.vendor.app.pkg"}
	_, _ = op.Execute(context.Background(), operation.ModeTrash, operation.Identity{})
	if !sawPrivileged {
		t.Error("pkgutil --forget must be run privileged")
	}
}

func TestReceiptForget_HistoryRecord(t *testing.T) {
	op := operation.ReceiptForgetOp{PackageID: "com.vendor.app.pkg"}
	e := op.HistoryRecord(operation.Receipt{Status: "ok"})
	if e.Op != operation.KindReceiptForget || e.Reversible != operation.Irreversible {
		t.Errorf("op/rev = %v/%v", e.Op, e.Reversible)
	}
	if e.PackageID != "com.vendor.app.pkg" || e.Cmd == "" {
		t.Errorf("history fields = %+v", e)
	}
}
