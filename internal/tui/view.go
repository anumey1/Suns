package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety"
)

// View renders the three-zone frame: header / body / footer (§10.3).
func (m model) View() string {
	if m.width == 0 {
		return "starting…"
	}
	th := m.th()

	header := m.renderHeader()
	footer := m.help.View(m.keys)

	// Body fills the space between header and footer. -2 accounts for the
	// blank separator lines around the body.
	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer) - 2
	if bodyHeight < 1 {
		bodyHeight = 1
	}
	body := m.renderBody(bodyHeight)

	_ = th
	return strings.Join([]string{header, "", body, "", footer}, "\n")
}

func (m model) renderHeader() string {
	th := m.th()
	left := th.Title().Render("☀ Suns") + th.MutedText().Render("  ·  Burn It All")
	right := th.ConfirmChip() + "   " + th.DeletionChip(m.deletionObliterate())

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) renderBody(h int) string {
	th := m.th()
	switch m.screen {
	case screenBusy:
		return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" "+th.NormalText().Render(m.busyMsg))

	case screenModal:
		return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, m.renderModal())

	case screenResult:
		box := th.NormalText().Render(m.result) + "\n\n" + th.MutedText().Render("press enter to return")
		return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center, box)

	case screenDashboard:
		return m.renderDashboard(h)

	default: // screenMenu
		return m.renderMenu(h)
	}
}

func (m model) renderMenu(h int) string {
	th := m.th()
	var b strings.Builder
	b.WriteString(th.MutedText().Render("Select an action:") + "\n\n")
	for i, item := range m.menu {
		if i == m.cursor {
			b.WriteString(th.Selected().Render("› "+item.title) + th.MutedText().Render("  — "+item.desc) + "\n")
		} else {
			b.WriteString(th.NormalText().Render("  "+item.title) + th.MutedText().Render("  — "+item.desc) + "\n")
		}
	}
	return lipgloss.NewStyle().Padding(0, 2).Render(b.String())
}

// renderModal is the frozen-plan confirmation modal — the visual embodiment of
// the safety gate (§10.4). It renders the sealed plan grouped by kind with a
// reversibility badge and complete totals, then the confirm/cancel hints.
func (m model) renderModal() string {
	th := m.th()
	var b strings.Builder
	b.WriteString(th.Title().Render("Confirm plan") + "\n\n")

	for _, g := range m.groups {
		danger := g.Reversibility == operation.Irreversible
		b.WriteString(th.Badge(safety.Badge(g.Reversibility), danger))
		fmt.Fprintf(&b, "  %s  (%d items", g.Kind, g.Count)
		if g.Bytes > 0 {
			fmt.Fprintf(&b, ", %s", humanBytes(g.Bytes))
		}
		b.WriteString(")\n")
		shown := g.Lines
		const maxLines = 8
		if len(shown) > maxLines {
			shown = shown[:maxLines]
		}
		for _, line := range shown {
			b.WriteString(th.MutedText().Render("    "+line) + "\n")
		}
		if extra := len(g.Lines) - len(shown); extra > 0 {
			b.WriteString(th.MutedText().Render(fmt.Sprintf("    … and %d more", extra)) + "\n")
		}
	}

	sum := safety.Summarize(m.groups)
	fmt.Fprintf(&b, "\nTotal: %d operations", sum.TotalOps)
	if sum.TotalBytes > 0 {
		fmt.Fprintf(&b, " · %s reclaimable", humanBytes(sum.TotalBytes))
	}
	b.WriteString("\n")
	b.WriteString(th.MutedText().Render(fmt.Sprintf("Deletion mode: %s", m.state.DeletionMode())) + "\n\n")
	b.WriteString(th.Selected().Render("enter") + th.MutedText().Render(" confirm   ") +
		th.Selected().Render("esc") + th.MutedText().Render(" cancel"))

	width := m.width * 3 / 4
	if width > 90 {
		width = 90
	}
	if width < 20 {
		width = 20
	}
	return th.Border(true).Padding(1, 2).Width(width).Render(b.String())
}
