package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	sunsnet "github.com/anumey1/Suns/internal/net"
	"github.com/anumey1/Suns/pkg/syscmd"
	"github.com/spf13/cobra"
)

// newNetCmd builds `suns net` — the read-only socket map + listening-port audit.
func newNetCmd() *cobra.Command {
	var (
		asJSON    bool
		noDNS     bool
		listening bool
	)
	cmd := &cobra.Command{
		Use:   "net",
		Short: "Map which app owns which socket; audit externally-reachable ports (read-only)",
		Long: `Net lists live network sockets and which process owns each, and audits listening
ports by reachability — flagging those bound to all interfaces (0.0.0.0 / ::),
which are reachable from the network, versus loopback-only. It is read-only and
never prompts. Remote addresses are reverse-resolved (cached, bounded) unless
--no-dns is given.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			rep, err := sunsnet.Sockets(ctx, syscmd.New(), sunsnet.Options{ResolveDNS: !noDNS})
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			renderListening(out, rep.Listening())
			if !listening {
				renderActive(out, rep.Active())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the full report as JSON")
	cmd.Flags().BoolVar(&noDNS, "no-dns", false, "skip reverse-DNS resolution of remote addresses")
	cmd.Flags().BoolVar(&listening, "listening", false, "show only the listening-port audit")
	cmd.AddCommand(newNetLanCmd())
	cmd.AddCommand(newNetBWCmd())
	return cmd
}

// newNetBWCmd builds `suns net bw` — the bandwidth detector: reliable per-
// interface throughput plus experimental per-process attribution via nettop.
func newNetBWCmd() *cobra.Command {
	var (
		asJSON   bool
		interval int
		topN     int
	)
	cmd := &cobra.Command{
		Use:     "bw",
		Aliases: []string{"bandwidth"},
		Short:   "Network throughput: per-interface rates + experimental per-process talkers",
		Long: `Bw samples network throughput over a short window. Per-interface rates (the
reliable core) come from the kernel interface counters. Per-process attribution
is EXPERIMENTAL — it differences a short nettop capture and degrades to
"unavailable" rather than show wrong numbers if nettop can't be read.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			rep, err := sunsnet.Bandwidth(ctx, syscmd.New(), sunsnet.BWOptions{
				Interval: time.Duration(interval) * time.Second,
				TopN:     topN,
			})
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			fmt.Fprintf(out, "Interface throughput (over %.1fs):\n", rep.IntervalSeconds)
			if len(rep.Interfaces) == 0 {
				fmt.Fprintln(out, "  (no active interfaces in the window)")
			}
			for _, i := range rep.Interfaces {
				fmt.Fprintf(out, "  %-8s ↓ %s/s   ↑ %s/s\n", i.Name,
					humanBytes(int64(i.RxBytesPerSec)), humanBytes(int64(i.TxBytesPerSec)))
			}

			fmt.Fprintln(out)
			if !rep.ProcessesAvailable {
				fmt.Fprintln(out, "Per-process bandwidth: unavailable (nettop could not be read).")
				return nil
			}
			fmt.Fprintln(out, "Top processes (experimental):")
			for _, p := range rep.Processes {
				fmt.Fprintf(out, "  %-7d %-22s ↓ %s/s   ↑ %s/s\n", p.PID, p.Name,
					humanBytes(int64(p.RxBytesPerSec)), humanBytes(int64(p.TxBytesPerSec)))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the report as JSON")
	cmd.Flags().IntVar(&interval, "interval", 1, "sampling window in seconds")
	cmd.Flags().IntVar(&topN, "top", 10, "max per-process rows")
	return cmd
}

// newNetLanCmd builds `suns net lan` — the passive LAN device scanner with an
// opt-in, warning-gated active port probe.
func newNetLanCmd() *cobra.Command {
	var (
		asJSON bool
		noDNS  bool
		probe  bool
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "lan",
		Short: "Discover LAN devices from the ARP cache (IP/MAC/vendor/host); optional port probe",
		Long: `Lan lists devices in this Mac's ARP cache — those it has recently communicated
with — enriched with a MAC-vendor guess (curated subset) and a reverse-DNS
hostname. It is passive and does not promise every device on the network.

--probe additionally attempts TCP connections to common ports on each device.
That is ACTIVE scanning: only do it on networks you own or are authorized to
test. You will be asked to confirm unless --yes is given.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			if probe && !yes {
				fmt.Fprintln(out, "⚠  Active port probing connects to devices you may not own, which can be")
				fmt.Fprintln(out, "   unwelcome or unlawful on networks you are not authorized to test.")
				if !confirm(cmd.InOrStdin(), out, "Proceed with active probing?") {
					fmt.Fprintln(out, "Probing skipped; showing passive results only.")
					probe = false
				}
			}

			rep, err := sunsnet.LANScan(ctx, syscmd.New(), sunsnet.LANOptions{ResolveDNS: !noDNS, Probe: probe})
			if err != nil {
				return err
			}

			if asJSON {
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(rep)
			}

			fmt.Fprintf(out, "LAN devices in ARP cache (%d):\n", len(rep.Devices))
			for _, d := range rep.Devices {
				vendor := d.Vendor
				if vendor == "" {
					vendor = "unknown vendor"
				}
				fmt.Fprintf(out, "  %-15s %-17s %-22s %s\n", d.IP, d.MAC, vendor, d.Hostname)
				if len(d.OpenPorts) > 0 {
					fmt.Fprintf(out, "      open ports: %v\n", d.OpenPorts)
				}
			}
			if !rep.Probed {
				fmt.Fprintln(out, "(passive — pass --probe to actively check open ports)")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit the device list as JSON")
	cmd.Flags().BoolVar(&noDNS, "no-dns", false, "skip reverse-DNS hostname resolution")
	cmd.Flags().BoolVar(&probe, "probe", false, "actively probe common TCP ports (asks to confirm)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the active-probe confirmation")
	return cmd
}

func renderListening(out io.Writer, conns []sunsnet.Conn) {
	fmt.Fprintf(out, "Listening ports (%d):\n", len(conns))
	for _, c := range conns {
		fmt.Fprintf(out, "  %-5s %-4s %-5s %-22s %-7d %s\n",
			scopeMarker(c.Scope), c.Proto, c.Family, c.LocalAddr+":"+c.LocalPort, c.PID, c.Command)
	}
	fmt.Fprintln(out)
}

func renderActive(out io.Writer, conns []sunsnet.Conn) {
	fmt.Fprintf(out, "Active connections (%d):\n", len(conns))
	for _, c := range conns {
		remote := c.RemoteAddr + ":" + c.RemotePort
		if c.RemoteHost != "" {
			remote = fmt.Sprintf("%s (%s)", remote, c.RemoteHost)
		}
		fmt.Fprintf(out, "  %-6s %-7d %-20s → %-28s %s\n",
			c.Proto, c.PID, c.LocalAddr+":"+c.LocalPort, remote, c.Command)
	}
}

// scopeMarker colors a listening socket by reachability.
func scopeMarker(scope string) string {
	switch scope {
	case sunsnet.ScopeWildcard:
		return "[!]" // externally reachable
	case sunsnet.ScopeLoopback:
		return "[ok]"
	default:
		return "[~]" // bound to a specific interface
	}
}
