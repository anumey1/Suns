// Package safety holds the polymorphic safety machinery shared by the CLI text
// preview and the TUI confirmation modal: the gate, the never-touch deny floor,
// identity-aware execution revalidation, the fd-anchored recursive deleter, and
// firmlink-aware identity handling (§4). It has no knowledge of how it is
// presented.
package safety

// Gate confirms a frozen, value-sealed plan.Plan. It groups operations by kind
// and renders the appropriate per-kind preview and reversibility badge per
// group, so the operator sees exactly what kind of harm each group can do
// before confirming (§4.2, §4.5). Only when confirm_mode is off (the default)
// is the gate shown; confirm_mode on means immediate execution with a post-hoc
// history record.
type Gate struct{}

// TODO(phase0): Confirm(p *plan.Plan) renders the sealed plan and returns the
// operator's decision; the executor consumes only the confirmed sealed plan.
