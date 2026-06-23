package tui

import (
	"context"
	"strings"
	"syscall"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/scanner"
	"github.com/anumey1/Suns/internal/telemetry"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/safety"
)

func newTestModel(t *testing.T) model {
	t.Helper()
	t.Setenv("NO_COLOR", "1") // strip ANSI so substring assertions are reliable
	m := New(context.Background(), config.NewDefault())
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return nm.(model)
}

func send(m model, msg tea.Msg) (model, tea.Cmd) {
	nm, cmd := m.Update(msg)
	return nm.(model), cmd
}

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestView_MenuShowsTitleAndModeChips(t *testing.T) {
	m := newTestModel(t)
	v := m.View()
	for _, want := range []string{"Suns", "Clean", "Restore", "Doctor", "deathstar:OFF", "jarjar:TRASH"} {
		if !strings.Contains(v, want) {
			t.Errorf("menu view missing %q\n---\n%s", want, v)
		}
	}
}

func TestToggle_JarjarFlipsDeletionChip(t *testing.T) {
	m := newTestModel(t)
	if strings.Contains(m.View(), "OBLITERATE") {
		t.Fatal("started in obliterate unexpectedly")
	}
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlJ})
	if !strings.Contains(m.View(), "OBLITERATE") {
		t.Fatalf("ctrl+j did not flip Jarjar to OBLITERATE\n%s", m.View())
	}
}

func TestToggle_DeathstarArmsTheme(t *testing.T) {
	m := newTestModel(t)
	m, _ = send(m, tea.KeyMsg{Type: tea.KeyCtrlD})
	if !m.state.ConfirmMode() {
		t.Fatal("ctrl+d did not arm confirm_mode")
	}
	if !strings.Contains(m.View(), "deathstar:ARMED") {
		t.Fatalf("armed chip not shown\n%s", m.View())
	}
}

func TestMenu_CursorMovesAndQuitQuits(t *testing.T) {
	m := newTestModel(t)
	if m.cursor != 0 {
		t.Fatal("cursor should start at 0")
	}
	m, _ = send(m, runes("j")) // Down
	if m.cursor != 1 {
		t.Fatalf("cursor = %d after down, want 1", m.cursor)
	}
	_, cmd := send(m, runes("q"))
	if cmd == nil || cmd() != tea.QuitMsg(struct{}{}) {
		t.Fatal("q did not produce a Quit command")
	}
}

func TestHelp_ToggleExpands(t *testing.T) {
	m := newTestModel(t)
	if m.help.ShowAll {
		t.Fatal("help should start collapsed")
	}
	m, _ = send(m, runes("?"))
	if !m.help.ShowAll {
		t.Fatal("? did not expand help")
	}
}

// Selecting Clean kicks off discovery (busy screen) without executing anything.
func TestSelectClean_EntersBusy(t *testing.T) {
	m := newTestModel(t)
	// Clean is the first menu item; select it.
	m, cmd := send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.screen != screenBusy {
		t.Fatalf("screen = %v after selecting Clean, want busy", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected a discovery command")
	}
	// We deliberately do NOT invoke cmd() — that would touch the real filesystem.
}

// onDiscovered with a non-empty result in safe mode shows the confirmation
// modal; the modal renders the sealed plan with a reversibility badge.
func TestDiscovered_ShowsModalWithPlan(t *testing.T) {
	m := newTestModel(t)
	res := scanner.Result{Ops: []operation.Operation{
		operation.FileDeleteOp{Path: "/Users/x/Library/Caches/demo", Size: 4096},
	}}
	m, _ = send(m, discoveredMsg{res: res})
	if m.screen != screenModal {
		t.Fatalf("screen = %v, want modal", m.screen)
	}
	v := m.View()
	for _, want := range []string{"Confirm plan", "/Users/x/Library/Caches/demo", "Reversible", "enter", "esc"} {
		if !strings.Contains(v, want) {
			t.Errorf("modal missing %q\n---\n%s", want, v)
		}
	}
}

func TestDiscovered_EmptyGoesToResult(t *testing.T) {
	m := newTestModel(t)
	m, _ = send(m, discoveredMsg{res: scanner.Result{}})
	if m.screen != screenResult {
		t.Fatalf("screen = %v, want result", m.screen)
	}
	if !strings.Contains(m.View(), "Nothing to clean") {
		t.Fatalf("expected nothing-to-clean message\n%s", m.View())
	}
}

// Entering the dashboard starts the poller and renders live vitals; the heavy/
// root sources show an honest unavailable badge rather than fake zeros.
func TestDashboard_RendersLiveVitals(t *testing.T) {
	m := newTestModel(t)
	nm, cmd := m.enterDashboard(false)
	m = nm.(model)
	if m.screen != screenDashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
	}
	if cmd == nil {
		t.Fatal("expected a dashboard tick command")
	}
	defer func() {
		if m.dashCancel != nil {
			m.dashCancel()
		}
	}()

	// Wait for the first published snapshot (gopsutil sources are read-only).
	deadline := time.Now().Add(3 * time.Second)
	for m.poller.Snapshot() == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if m.poller.Snapshot() == nil {
		t.Fatal("no snapshot published")
	}

	v := m.View()
	for _, want := range []string{"get-coffee", "CPU", "MEMORY", "PID", "uptime"} {
		if !strings.Contains(v, want) {
			t.Errorf("dashboard missing %q\n---\n%s", want, v)
		}
	}
	// Heavy/root source rendered as unavailable, not a fake value.
	if !strings.Contains(v, "N/A") {
		t.Errorf("expected an N/A badge for an unwired root source\n%s", v)
	}
}

// Leaving a non-home dashboard returns to the menu and cancels the poller.
func TestDashboard_LeaveReturnsToMenu(t *testing.T) {
	m := newTestModel(t)
	nm, _ := m.enterDashboard(false)
	m = nm.(model)
	nm, _ = m.leaveDashboard()
	m = nm.(model)
	if m.screen != screenMenu {
		t.Fatalf("after leave, screen = %v, want menu", m.screen)
	}
	if m.poller != nil {
		t.Error("poller should be cleared after leaving the dashboard")
	}
}

// The confirmation modal renders a ProcessKill plan with an irreversible badge
// (the deletion axis is inert for non-file ops, §4.3).
func TestModal_RendersProcessKill(t *testing.T) {
	m := newTestModel(t)
	op := operation.ProcessKillOp{PID: 1234, Name: "demo", Signal: int(syscall.SIGTERM)}
	m.plan = plan.New([]operation.Operation{op}).Seal()
	m.groups = safety.BuildGroups(m.plan, operation.ModeTrash)
	m.screen = screenModal

	v := m.View()
	for _, want := range []string{"process_kill", "Irreversible", "PID 1234"} {
		if !strings.Contains(v, want) {
			t.Errorf("kill modal missing %q\n---\n%s", want, v)
		}
	}
}

// After a dashboard kill executes, the model returns to the dashboard with a
// status line rather than the generic result screen.
func TestExecuted_KillReturnsToDashboard(t *testing.T) {
	m := newTestModel(t)
	m.poller = telemetry.New(time.Second) // non-nil so a dashboard render is safe
	m.killReturnDashboard = true
	m.screen = screenBusy

	m, _ = send(m, executedMsg{summary: "killed 1"})
	if m.screen != screenDashboard {
		t.Fatalf("screen = %v, want dashboard", m.screen)
	}
	if m.dashStatus != "killed 1" {
		t.Fatalf("dashStatus = %q, want %q", m.dashStatus, "killed 1")
	}
	if m.killReturnDashboard {
		t.Error("killReturnDashboard should be cleared")
	}
}

func TestElevatedMsg_SetsStatus(t *testing.T) {
	m := newTestModel(t)
	m.poller = telemetry.New(time.Second)
	m.screen = screenDashboard

	ok, _ := send(m, elevatedMsg{err: nil})
	if !strings.Contains(ok.dashStatus, "elevated") {
		t.Fatalf("dashStatus = %q, want elevated", ok.dashStatus)
	}
	cancelled, _ := send(m, elevatedMsg{err: context.Canceled})
	if !strings.Contains(cancelled.dashStatus, "canceled") {
		t.Fatalf("dashStatus = %q, want canceled", cancelled.dashStatus)
	}
}

// Sanity: BuildGroups (the same gate logic the modal renders) classifies an
// obliterate-mode file delete as irreversible.
func TestModalReflectsObliterateReversibility(t *testing.T) {
	p := plan.New([]operation.Operation{operation.FileDeleteOp{Path: "/x", Size: 1}}).Seal()
	g := safety.BuildGroups(p, operation.ModeObliterate)
	if g[0].Reversibility != operation.Irreversible {
		t.Fatalf("obliterate file delete = %v, want Irreversible", g[0].Reversibility)
	}
}
