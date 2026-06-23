package tui

import "github.com/charmbracelet/bubbles/key"

// keyMap defines all key bindings; it satisfies help.KeyMap so the help overlay
// (?) lists every binding (§10.4, §10.6).
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Select    key.Binding
	Back      key.Binding
	Deathstar key.Binding
	Jarjar    key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "move up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "move down")),
		Select:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select/confirm")),
		Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/cancel")),
		Deathstar: key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "toggle deathstar")),
		Jarjar:    key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "toggle jarjar")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}
}

// ShortHelp is the footer hint line.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Select, k.Deathstar, k.Jarjar, k.Help, k.Quit}
}

// FullHelp is the expanded ? overlay.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Select, k.Back},
		{k.Deathstar, k.Jarjar},
		{k.Help, k.Quit},
	}
}
