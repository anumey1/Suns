package cli

import (
	"fmt"

	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/spf13/cobra"
)

// newDNSFlushCmd builds `suns dns-flush` — the DNS Cache Incinerator (§12.11).
//
// It is destructive(action) but harmless: a single irreversible DNSFlushOp routed
// through the same gate as every other destructive action, so the 🔴 badge shows
// and the operator confirms before anything runs. Root is acquired once, at
// execution, through the privilege chokepoint behind the wired system runner.
func newDNSFlushCmd() *cobra.Command {
	var (
		dryRun bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:     "dns-flush",
		Aliases: []string{"dnsflush"},
		Short:   "Flush the system DNS cache and restart mDNSResponder (requires admin)",
		Long: `Dns-flush clears the system DNS resolver cache by running
"dscacheutil -flushcache; killall -HUP mDNSResponder". It is irreversible (there
is nothing to undo) but harmless — the only cost is that the next handful of
lookups are marginally slower as the cache repopulates.

Useful after editing /etc/hosts, changing DNS records, switching VPNs, or
debugging stale routing. It requires an admin password, asked once at execution.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			// The flush is a single typed op; the deletion Mode axis is inert for
			// it, so the mode passed to the gate is immaterial.
			p := plan.New([]operation.Operation{operation.DNSFlushOp{}}).Seal()
			groups := safety.BuildGroups(p, operation.ModeTrash)
			fmt.Fprint(out, safety.Render(groups, 30))

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}

			if !yes {
				if !confirm(cmd.InOrStdin(), out, "Flush the DNS cache?") {
					fmt.Fprintln(out, "Aborted. Nothing was changed.")
					return nil
				}
			}

			logPath, err := history.DefaultPath()
			if err != nil {
				return err
			}
			log, err := history.Open(logPath)
			if err != nil {
				return err
			}

			for _, r := range safety.Execute(ctx, p, operation.ModeTrash) {
				if appendErr := log.Append(r.Entry); appendErr != nil {
					fmt.Fprintf(out, "warning: could not record history: %v\n", appendErr)
				}
				switch r.Receipt.Status {
				case "ok":
					fmt.Fprintln(out, "DNS cache flushed.")
				case "skipped:no-privilege":
					fmt.Fprintln(out, "Skipped: admin privilege was not granted.")
				default:
					fmt.Fprintf(out, "Failed: %v\n", r.Receipt.Err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the frozen plan and exit without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "bypass the confirmation gate")
	return cmd
}
