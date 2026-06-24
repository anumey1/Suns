package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/anumey1/Suns/internal/audit"
	"github.com/anumey1/Suns/pkg/privilege"
	"github.com/anumey1/Suns/pkg/syscmd"
	"github.com/spf13/cobra"
)

// newAuditCmd builds `suns audit` — the read-only security-posture view.
func newAuditCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "audit",
		Aliases: []string{"secure"},
		Short:   "Security posture: SIP, Gatekeeper, FileVault, XProtect (read-only)",
		Long: `Audit reports your macOS security posture by reading SIP (csrutil), Gatekeeper
(spctl), FileVault (fdesetup), and the XProtect version. It is entirely
read-only — it never prompts and never changes anything — and flags each finding
by severity.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			rep, err := audit.Posture(ctx, syscmd.New())
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			fmt.Fprintf(out, "suns audit — security posture: %s\n", auditMarker(rep.Severity))
			for _, f := range rep.Findings() {
				fmt.Fprintf(out, "  %-5s %-11s %-9s %s\n", auditMarker(f.Severity), f.Name, f.State, f.Detail)
			}
			if rep.XProtectVersion != "" {
				fmt.Fprintf(out, "  %-5s %-11s %s\n", "[ok]", "XProtect", "version "+rep.XProtectVersion)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the posture as JSON")
	cmd.AddCommand(newAuditLogsCmd())
	return cmd
}

// newAuditLogsCmd builds `suns audit logs` — the sudo/auth-log analyzer. It
// queries the unified log and therefore needs root, acquired once via the
// chokepoint (the `log` action is allowlisted, §6).
func newAuditLogsCmd() *cobra.Command {
	var (
		asJSON bool
		since  string
	)
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Sudo / authentication timeline from the unified log (root)",
		Long: `Logs surfaces sudo activity — successes, failed authentications, and denied
escalations — from the macOS unified log (not the deprecated system.log),
highlighting rapid-failure bursts. It is read-only but needs root to read the
auth records, so it asks for your password once.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// Acquire elevation up front; the query then runs under the ticket.
			choke := privilege.New(privilege.TerminalPrompter{})
			if err := choke.Acquire(ctx); err != nil {
				fmt.Fprintln(out, "Elevation canceled; the auth-log query needs root.")
				return nil
			}

			rep, err := audit.AuthLog(ctx, choke, audit.LogOptions{Since: since})
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			fmt.Fprintf(out, "sudo activity (last %s): %d events, %d failures\n", rep.Window, len(rep.Events), rep.Failures)
			for _, b := range rep.Bursts {
				fmt.Fprintf(out, "  [x] RAPID FAILURES: %s — %d in %s (%s–%s)\n",
					b.User, b.Count, b.End.Sub(b.Start).Round(time.Second),
					b.Start.Format("15:04:05"), b.End.Format("15:04:05"))
			}
			for _, e := range rep.Events {
				fmt.Fprintf(out, "  %-5s %-19s %-12s %s\n",
					outcomeMarker(e.Outcome), e.Time.Format("2006-01-02 15:04:05"), e.User, e.Message)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the timeline as JSON")
	cmd.Flags().StringVar(&since, "since", "1d", "how far back to query (log --last value, e.g. 1d, 6h)")
	return cmd
}

// outcomeMarker colors an auth outcome.
func outcomeMarker(outcome string) string {
	switch outcome {
	case "success":
		return "[ok]"
	case "failure", "denied":
		return "[x]"
	default:
		return "[~]"
	}
}

// auditMarker renders a severity as a colorless status tag (consistent with
// `doctor`'s markers).
func auditMarker(s audit.Severity) string {
	switch s {
	case audit.SevOK:
		return "[ok]"
	case audit.SevWarn:
		return "[!]"
	case audit.SevRisk:
		return "[x]"
	default:
		return "[?]"
	}
}
