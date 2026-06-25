package cli

import (
	"fmt"

	"github.com/anumey1/Suns/internal/docker"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/spf13/cobra"
)

// newDockerPruneCmd builds `suns docker-prune` — the Docker Environment Nuke
// (§12.15). It probes for a container engine (Docker Desktop / Colima / OrbStack),
// previews reclaimable space, and — only after the gate confirms — prunes all
// unused images, containers, and networks. Volumes (which hold persistent data)
// are excluded unless --volumes is given: a deliberate safe-by-default hardening
// of the spec's prune, consistent with the obliterate / --prune-now opt-ins.
func newDockerPruneCmd() *cobra.Command {
	var (
		dryRun  bool
		yes     bool
		volumes bool
	)
	cmd := &cobra.Command{
		Use:     "docker-prune",
		Aliases: []string{"docker"},
		Short:   "Reclaim space from unused Docker images/containers (and optionally volumes)",
		Long: `Docker-prune detects a running container engine (Docker Desktop, Colima, or
OrbStack), previews how much it can reclaim, and after you confirm runs
"docker system prune -a" to remove all unused images, stopped containers, and
networks. If no engine is installed or the daemon is not running, it prints a
clear no-op and changes nothing.

Volumes hold persistent data (databases, etc.), so they are NOT pruned unless you
pass --volumes. Pruned images must be re-pulled or rebuilt — this is irreversible.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()

			st := docker.Detect(ctx)
			if !st.Installed || !st.Running {
				detail := st.Detail
				if detail == "" {
					detail = "no usable container engine"
				}
				fmt.Fprintf(out, "Docker: %s — nothing to do.\n", detail)
				return nil
			}

			fmt.Fprintf(out, "Engine: running")
			if st.Endpoint != "" {
				fmt.Fprintf(out, " (%s)", st.Endpoint)
			}
			fmt.Fprintf(out, " · reclaimable ≈ %s\n", humanBytes(st.Reclaimable))

			op := operation.ContainerPruneOp{
				Endpoint:       st.Endpoint,
				EstimatedBytes: st.Reclaimable,
				IncludeVolumes: volumes,
			}
			p := plan.New([]operation.Operation{op}).Seal()
			groups := safety.BuildGroups(p, operation.ModeTrash)
			fmt.Fprint(out, safety.Render(groups, 30))
			if !volumes {
				fmt.Fprintln(out, "(volumes preserved — pass --volumes to also prune unused volumes)")
			}

			if dryRun {
				fmt.Fprintln(out, "(dry run — no changes made)")
				return nil
			}
			if !yes {
				if !confirm(cmd.InOrStdin(), out, "Prune now?") {
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

			// Wire the CLI-backed pruner with the discovered binary just before
			// executing, mirroring how clean/nuke wire the Trasher.
			operation.UseContainerPruner(docker.NewPruner(st.Binary))

			for _, r := range safety.Execute(ctx, p, operation.ModeTrash) {
				if appendErr := log.Append(r.Entry); appendErr != nil {
					fmt.Fprintf(out, "warning: could not record history: %v\n", appendErr)
				}
				switch r.Receipt.Status {
				case "ok":
					fmt.Fprintln(out, "Pruned.")
				default:
					fmt.Fprintf(out, "Not pruned: %s\n", r.Receipt.Status)
					if r.Receipt.Err != nil {
						fmt.Fprintf(out, "  %v\n", r.Receipt.Err)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "detect and preview without acting")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "bypass the confirmation gate")
	cmd.Flags().BoolVar(&volumes, "volumes", false, "also prune unused volumes (deletes persistent data)")
	return cmd
}
