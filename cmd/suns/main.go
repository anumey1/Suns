// Command suns is the entry point for the Suns macOS system utility.
//
// It reads configuration once into a RWMutex-guarded SessionState (§4.9) and
// dispatches to a CLI command or the interactive TUI. Phase 0 wires the proven
// meta-commands (doctor, version); the full command tree and TUI land in later
// phases. See Docs/SunsMasterTD.md §8.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/anumey1/Suns/internal/cli"
)

func main() {
	// One shared context cancelled on SIGINT/SIGTERM so every worker can exit
	// promptly (§3.5).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	os.Exit(cli.Execute(ctx))
}
