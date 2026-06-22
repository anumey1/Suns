// Package views holds the individual TUI screens routed by the root App:
// menu, clean, nuke, dashboard, audit, net, scan, restore, onboarding (§8, §10).
//
// Every screen uses the consistent three-zone frame (header/mode bar, flex
// body, footer keybind hints) composed with lipgloss.JoinVertical/JoinHorizontal
// and is responsive to tea.WindowSizeMsg (§10.3).
package views
