// Package safety holds the polymorphic safety machinery shared by the CLI text
// preview and the TUI confirmation modal: the gate (which confirms a frozen,
// value-sealed plan) and the identity-aware execution orchestration (§4). It
// imports the Operation model and the leaf primitives (floor, identity,
// firmlink, fsdelete, trash via the operations) but is itself imported by no
// lower layer, so there is no import cycle.
package safety

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
)

// Group is the per-kind aggregation the gate renders: one reversibility badge
// and a total per operation kind (§4.2).
type Group struct {
	Kind          operation.OpKind
	Reversibility operation.Reversibility
	Count         int
	Bytes         int64
	Lines         []string
}

// Summary totals a plan across all groups, as shown above the gate's [y/N].
type Summary struct {
	TotalOps   int
	TotalBytes int64
}

// BuildGroups groups a sealed plan's operations by kind and computes totals and
// the EFFECTIVE reversibility. For FileDelete the effective reversibility
// depends on the active deletion mode (obliterate → Irreversible), which the
// gate knows even though the operation itself does not carry the mode (§4.3).
// Non-file operations always show their intrinsic reversibility regardless of
// the deletion axis.
func BuildGroups(p *plan.Plan, deletion operation.Mode) []Group {
	byKind := map[operation.OpKind]*Group{}
	var order []operation.OpKind
	for _, op := range p.Ops {
		pv := op.Describe()
		g, ok := byKind[pv.Kind]
		if !ok {
			g = &Group{Kind: pv.Kind, Reversibility: effectiveReversibility(op, deletion)}
			byKind[pv.Kind] = g
			order = append(order, pv.Kind)
		}
		g.Count++
		g.Bytes += pv.Bytes
		g.Lines = append(g.Lines, pv.Line)
	}
	groups := make([]Group, 0, len(order))
	for _, k := range order {
		groups = append(groups, *byKind[k])
	}
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Kind < groups[j].Kind })
	return groups
}

// Summarize totals the groups.
func Summarize(groups []Group) Summary {
	var s Summary
	for _, g := range groups {
		s.TotalOps += g.Count
		s.TotalBytes += g.Bytes
	}
	return s
}

// effectiveReversibility resolves the badge the gate shows. The deletion axis
// only affects FileDelete; everything else uses its intrinsic class.
func effectiveReversibility(op operation.Operation, deletion operation.Mode) operation.Reversibility {
	if op.Kind() == operation.KindFileDelete {
		if deletion == operation.ModeObliterate {
			return operation.Irreversible
		}
		return operation.Reversible
	}
	return op.Reversibility()
}

// Badge returns the unmistakable reversibility marker (§4.2).
func Badge(r operation.Reversibility) string {
	switch r {
	case operation.Reversible:
		return "🟢 Reversible"
	case operation.Recoverable:
		return "🟡 Recoverable"
	default:
		return "🔴 Irreversible"
	}
}

// Render produces the text preview shown in the CLI; the TUI modal renders the
// same groups (§3.1). At most maxLines targets are listed per group, with an
// overflow count.
func Render(groups []Group, maxLines int) string {
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, "%s  %s  (%d items", Badge(g.Reversibility), g.Kind, g.Count)
		if g.Bytes > 0 {
			fmt.Fprintf(&b, ", %s", humanBytes(g.Bytes))
		}
		b.WriteString(")\n")
		shown := g.Lines
		if maxLines > 0 && len(shown) > maxLines {
			shown = shown[:maxLines]
		}
		for _, line := range shown {
			fmt.Fprintf(&b, "    %s\n", line)
		}
		if extra := len(g.Lines) - len(shown); extra > 0 {
			fmt.Fprintf(&b, "    … and %d more\n", extra)
		}
	}
	s := Summarize(groups)
	fmt.Fprintf(&b, "Total: %d operations", s.TotalOps)
	if s.TotalBytes > 0 {
		fmt.Fprintf(&b, ", %s reclaimable", humanBytes(s.TotalBytes))
	}
	b.WriteString("\n")
	return b.String()
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
