package theme

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Theme is the active palette. The confirmation mode (Deathstar) recolors the
// entire chrome so the operator's safety posture is unmistakable: cool plasma
// blue when safe, aggressive ember red when armed (§10.2).
type Theme struct {
	Armed   bool // confirm_mode on → armed (immediate execution)
	NoColor bool // NO_COLOR set → degrade to monochrome-plus-symbols

	Accent    lipgloss.TerminalColor
	AccentDim lipgloss.TerminalColor
	Success   lipgloss.TerminalColor
	Warning   lipgloss.TerminalColor
	Danger    lipgloss.TerminalColor
	Text      lipgloss.TerminalColor
	Muted     lipgloss.TerminalColor
	Surface   lipgloss.TerminalColor
}

// New builds the theme for the given armed state, respecting NO_COLOR.
func New(armed bool) Theme {
	t := Theme{
		Armed:   armed,
		NoColor: os.Getenv("NO_COLOR") != "",
		Success: lipgloss.Color("#34C759"),
		Danger:  lipgloss.Color("#FF453A"),
		Text:    lipgloss.AdaptiveColor{Light: "#1c1c1e", Dark: "#E6E6E6"},
		Muted:   lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#8A8A8A"},
	}
	if armed {
		t.Accent = lipgloss.Color("#FF3B30")
		t.AccentDim = lipgloss.Color("#A11111")
		t.Warning = lipgloss.Color("#FF9F0A")
		t.Surface = lipgloss.AdaptiveColor{Light: "#f7eaea", Dark: "#2a1112"}
	} else {
		t.Accent = lipgloss.Color("#4FC3F7")
		t.AccentDim = lipgloss.Color("#2B7A99")
		t.Warning = lipgloss.Color("#FFD60A")
		t.Surface = lipgloss.AdaptiveColor{Light: "#eef6fb", Dark: "#1c1c1e"}
	}
	return t
}

// fg returns a foreground style for c, or a plain style under NO_COLOR.
func (t Theme) fg(c lipgloss.TerminalColor) lipgloss.Style {
	if t.NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}

// Title styles the app title / header accent.
func (t Theme) Title() lipgloss.Style { return t.fg(t.Accent).Bold(true) }

// Border styles a panel border in the active accent. focused panels are bright,
// others dim (§10.3 focus model).
func (t Theme) Border(focused bool) lipgloss.Style {
	c := t.AccentDim
	if focused {
		c = t.Accent
	}
	s := lipgloss.NewStyle().Border(lipgloss.RoundedBorder())
	if t.NoColor {
		return s
	}
	return s.BorderForeground(c)
}

// MutedText styles secondary text (footer hints, descriptions).
func (t Theme) MutedText() lipgloss.Style { return t.fg(t.Muted) }

// NormalText styles primary text.
func (t Theme) NormalText() lipgloss.Style { return t.fg(t.Text) }

// Selected styles the highlighted menu row.
func (t Theme) Selected() lipgloss.Style { return t.fg(t.Accent).Bold(true) }

// ConfirmChip renders the Deathstar (confirm_mode) indicator.
func (t Theme) ConfirmChip() string {
	if t.Armed {
		return t.fg(t.Danger).Bold(true).Render("◉ deathstar:ARMED")
	}
	return t.fg(t.Accent).Render("◐ deathstar:OFF")
}

// DeletionChip renders the Jarjar (deletion_mode) indicator, independent of the
// accent (§10.2): 🗑 TRASH (green) vs 🔥 OBLITERATE (red).
func (t Theme) DeletionChip(obliterate bool) string {
	if obliterate {
		return t.fg(t.Danger).Bold(true).Render("🔥 jarjar:OBLITERATE")
	}
	return t.fg(t.Success).Render("🗑 jarjar:TRASH")
}

// Badge colors a reversibility marker. The text already carries an icon + word,
// so it remains legible under NO_COLOR (colorblind-safe, §10.6).
func (t Theme) Badge(text string, danger bool) string {
	if danger {
		return t.fg(t.Danger).Render(text)
	}
	return t.fg(t.Success).Render(text)
}

// IsAppleTerminal reports whether the host is Apple's Terminal.app, which has a
// documented Braille line-height bug; charts fall back to block elements there
// (§2.2, §10.4). Exposed here for the dashboard slice.
func IsAppleTerminal() bool {
	return os.Getenv("TERM_PROGRAM") == "Apple_Terminal"
}
