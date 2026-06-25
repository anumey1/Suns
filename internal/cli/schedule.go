package cli

import (
	"fmt"
	"os"

	"github.com/anumey1/Suns/internal/scheduler"
	"github.com/anumey1/Suns/pkg/syscmd"
	"github.com/spf13/cobra"
)

// newScheduleCmd builds `suns schedule` — the Scheduled Burn Daemon authoring
// command (§12.20). Subcommands install / uninstall / status manage a launchd
// user LaunchAgent that runs the constrained `suns clean --scheduled`.
func newScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage the unattended scheduled cleanup (launchd user agent)",
		Long: `Schedule installs a launchd user LaunchAgent that periodically runs the most-
constrained cleanup ("suns clean --scheduled"): locked to the safe-cache
allowlist, deletion forced to trash, never obliterate. Use the subcommands to
install, remove, or check it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newScheduleInstallCmd())
	cmd.AddCommand(newScheduleUninstallCmd())
	cmd.AddCommand(newScheduleStatusCmd())
	return cmd
}

func newScheduleInstallCmd() *cobra.Command {
	var (
		hour   int
		minute int
		yes    bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install/refresh the daily scheduled cleanup agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			bin, err := os.Executable()
			if err != nil {
				return fmt.Errorf("cannot resolve the suns binary path: %w", err)
			}

			fmt.Fprintf(out, "This installs a launchd agent that runs unattended at %02d:%02d daily:\n", hour, minute)
			fmt.Fprintln(out, "  • locked to the safe-cache allowlist (nothing else is touched)")
			fmt.Fprintln(out, "  • deletion forced to Trash — never permanent")
			fmt.Fprintln(out, "  • needs Full Disk Access, or it silently skips protected paths")
			if !yes {
				if !confirm(cmd.InOrStdin(), out, "Install the scheduled cleanup agent?") {
					fmt.Fprintln(out, "Aborted. Nothing was installed.")
					return nil
				}
			}

			cfg := scheduler.Config{BinaryPath: bin, Hour: hour, Minute: minute}
			if err := scheduler.Install(ctx, syscmd.New(), cfg); err != nil {
				return err
			}
			path, _ := scheduler.PlistPath()
			fmt.Fprintf(out, "Installed and loaded: %s\n", path)
			return nil
		},
	}
	cmd.Flags().IntVar(&hour, "hour", 3, "local hour (0–23) for the daily run")
	cmd.Flags().IntVar(&minute, "minute", 0, "local minute (0–59) for the daily run")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation")
	return cmd
}

func newScheduleUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the scheduled cleanup agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if err := scheduler.Uninstall(cmd.Context(), syscmd.New()); err != nil {
				return err
			}
			fmt.Fprintln(out, "Scheduled cleanup agent removed.")
			return nil
		},
	}
	return cmd
}

func newScheduleStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show whether the scheduled cleanup agent is installed and loaded",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			st, err := scheduler.CheckStatus(cmd.Context(), syscmd.New())
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Installed: %v\n", st.Installed)
			fmt.Fprintf(out, "Loaded:    %v\n", st.Loaded)
			fmt.Fprintf(out, "Plist:     %s\n", st.PlistPath)
			return nil
		},
	}
	return cmd
}
