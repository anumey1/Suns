// Command suns is the entry point for the Suns macOS system utility.
//
// It builds the Cobra root, reads configuration once into a RWMutex-guarded
// SessionState (§4.9), and dispatches to either a one-shot CLI command or the
// interactive TUI (the no-subcommand case). See Docs/SunsMasterTD.md §8.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "suns:", err)
		os.Exit(1)
	}
}

// run wires the application. Phase 0 will:
//   - construct the Cobra root and command tree (internal/cli),
//   - load config into config.SessionState,
//   - launch the TUI when invoked with no subcommand, else run the CLI command.
func run() error {
	// TODO(phase0): bootstrap session and dispatch.
	return nil
}
