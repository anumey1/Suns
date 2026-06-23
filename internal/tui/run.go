package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anumey1/Suns/internal/config"
)

// Run starts the interactive TUI with the given session state, in the alternate
// screen buffer. The program lifetime is tied to ctx, so a SIGINT from the
// shell tears it down cleanly (§3.5).
func Run(ctx context.Context, state *config.SessionState) error {
	p := tea.NewProgram(New(ctx, state), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}

// RunDashboard starts the TUI directly on the get-coffee live dashboard (the
// `suns get-coffee` entry point).
func RunDashboard(ctx context.Context, state *config.SessionState) error {
	p := tea.NewProgram(NewDashboard(ctx, state), tea.WithAltScreen(), tea.WithContext(ctx))
	_, err := p.Run()
	return err
}
