package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"syscall"

	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/privilege"
	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	var (
		killPID int
		signal  string
		yes     bool
		asJSON  bool
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "Inspect processes; flag runaways/zombies; kill (gated, identity-checked)",
		Long: `ps lists processes sorted by CPU, flagging high-CPU (runaway) candidates and
zombies (already dead — it reports the parent PID to act on rather than
uselessly signalling the zombie). With --kill it sends a confirmed,
identity-checked signal, defeating PID reuse; killing a root/other-user process
is delegated under sudo (§4.7, §12.8).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			rows, err := procctl.List(ctx)
			if err != nil {
				return err
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].CPU > rows[j].CPU })

			if killPID != 0 {
				return runKill(cmd, rows, killPID, signal, yes)
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			fmt.Fprintf(out, "%-7s %-24s %7s %10s  %s\n", "PID", "NAME", "CPU%", "MEM", "STATE")
			shown := rows
			if limit > 0 && len(shown) > limit {
				shown = shown[:limit]
			}
			for _, r := range shown {
				flag := " "
				if r.Zombie {
					flag = "Z"
				} else if r.HighCPU {
					flag = "!"
				}
				name := r.Name
				if len(name) > 24 {
					name = name[:23] + "…"
				}
				fmt.Fprintf(out, "%-7d %-24s %6.1f%% %10s  %s %s\n", r.PID, name, r.CPU, humanBytes(int64(r.Mem)), flag, r.Status)
			}
			// Zombies cannot be killed; point at the parent to reap them.
			for _, r := range rows {
				if r.Zombie {
					fmt.Fprintf(out, "note: PID %d (%s) is a zombie — already dead; reap via parent PID %d\n", r.PID, r.Name, r.PPID)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&killPID, "kill", 0, "send a confirmed, identity-checked signal to this PID")
	cmd.Flags().StringVar(&signal, "signal", "term", "signal for --kill: term|kill")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "bypass the confirmation gate")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the listing as JSON")
	cmd.Flags().IntVar(&limit, "limit", 40, "max rows to list (0 = all)")
	return cmd
}

func runKill(cmd *cobra.Command, rows []procctl.Row, pid int, signal string, yes bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	var row *procctl.Row
	for i := range rows {
		if rows[i].PID == pid {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		return fmt.Errorf("no such process: %d", pid)
	}
	if row.Zombie {
		fmt.Fprintf(out, "PID %d (%s) is a zombie — already dead and cannot be killed.\n"+
			"Reap it via its parent PID %d.\n", row.PID, row.Name, row.PPID)
		return nil
	}

	expect, ownerUID, _, err := procctl.Current(pid)
	if err != nil {
		return fmt.Errorf("reading process %d: %w", pid, err)
	}
	sig, err := signalValue(signal)
	if err != nil {
		return err
	}
	privileged := !procctl.IsOwnUser(ownerUID)

	op := operation.ProcessKillOp{PID: pid, Name: row.Name, Expect: expect, Signal: int(sig), Privileged: privileged}
	p := plan.New([]operation.Operation{op}).Seal()
	fmt.Fprint(out, safety.Render(safety.BuildGroups(p, operation.ModeTrash), 20))
	if privileged {
		fmt.Fprintln(out, "🔐 this process is owned by another user; root is required to kill it.")
	}

	if !yes {
		if !confirm(cmd.InOrStdin(), out, "Send signal?") {
			fmt.Fprintln(out, "Aborted.")
			return nil
		}
	}

	// Acquire a sudo ticket up front for the privileged delegation (the killer
	// then runs `sudo -n suns __killproc …` non-interactively).
	if privileged {
		choke := privilege.New(privilege.TerminalPrompter{})
		if err := choke.Acquire(ctx); err != nil {
			fmt.Fprintln(out, "Elevation canceled; not killed.")
			return nil
		}
	}

	logPath, err := history.DefaultPath()
	if err != nil {
		return err
	}
	log, _ := history.Open(logPath)
	for _, r := range safety.Execute(ctx, p, operation.ModeTrash) {
		if log != nil {
			_ = log.Append(r.Entry)
		}
		switch r.Receipt.Fate {
		case "killed":
			fmt.Fprintf(out, "Killed PID %d (%s) with %s.\n", pid, row.Name, signalName(sig))
		default:
			fmt.Fprintf(out, "Not killed: %s\n", r.Receipt.Status)
		}
	}
	return nil
}

func signalValue(name string) (syscall.Signal, error) {
	switch name {
	case "term", "TERM", "sigterm", "SIGTERM":
		return syscall.SIGTERM, nil
	case "kill", "KILL", "sigkill", "SIGKILL":
		return syscall.SIGKILL, nil
	default:
		return 0, fmt.Errorf("unknown signal %q (want term|kill)", name)
	}
}

func signalName(sig syscall.Signal) string {
	if sig == syscall.SIGKILL {
		return "SIGKILL"
	}
	return "SIGTERM"
}
