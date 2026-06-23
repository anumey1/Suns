package cli

import (
	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/tui"
	"github.com/spf13/cobra"
)

func newGetCoffeeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-coffee",
		Short: "Live telemetry dashboard (read-only; no Full Disk Access required)",
		Long: `get-coffee starts a continuous, real-time monitoring session — fire it up,
walk away to get a coffee, and the machine's vitals are laid out as live
widgets. It is entirely read-only and needs no Full Disk Access (§11).`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			state, err := config.Load(config.Overrides{})
			if err != nil {
				return err
			}
			return tui.RunDashboard(cmd.Context(), state)
		},
	}
}
