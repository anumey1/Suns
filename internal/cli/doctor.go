package cli

import (
	"errors"
	"fmt"

	"github.com/anumey1/Suns/internal/doctor"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Environment, permission, tool-version, and capability self-check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			rep := doctor.Run(cmd.Context())
			fmt.Fprintln(out, "suns doctor")
			for _, c := range rep.Checks {
				fmt.Fprintf(out, "  %-4s %-18s %s\n", marker(c.Status), c.Name, c.Detail)
			}
			if !rep.OK() {
				return errors.New("one or more checks failed")
			}
			return nil
		},
	}
}

func marker(s doctor.Status) string {
	switch s {
	case doctor.OK:
		return "[ok]"
	case doctor.Warn:
		return "[!]"
	default:
		return "[x]"
	}
}
