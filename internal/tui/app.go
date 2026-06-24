// Package tui is the presentation layer: a Bubble Tea Model-Update-View root
// that routes between views and renders the layout grammar, theme/mode
// recoloring, help overlay, and the frozen-plan confirmation modal (§3.1, §10).
//
// Dependencies flow downward only: the TUI drives the engines (scanner, safety,
// trash, history) and never the reverse. The same gate logic (safety.BuildGroups)
// backs both the CLI preview and this modal.
package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anumey1/Suns/internal/config"
	"github.com/anumey1/Suns/internal/doctor"
	"github.com/anumey1/Suns/internal/restore"
	"github.com/anumey1/Suns/internal/scanner"
	"github.com/anumey1/Suns/internal/telemetry"
	"github.com/anumey1/Suns/internal/tui/theme"
	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/plan"
	"github.com/anumey1/Suns/pkg/procctl"
	"github.com/anumey1/Suns/pkg/safety"
	"github.com/anumey1/Suns/pkg/trash"
)

type screen int

const (
	screenMenu screen = iota
	screenBusy
	screenModal
	screenResult
	screenDashboard
)

type action int

const (
	actClean action = iota
	actRestore
	actDashboard
	actAudit
	actDoctor
	actQuit
)

type menuItem struct {
	title  string
	desc   string
	action action
}

type model struct {
	ctx     context.Context
	state   *config.SessionState
	keys    keyMap
	help    help.Model
	spinner spinner.Model

	width, height int
	screen        screen
	menu          []menuItem
	cursor        int

	// clean flow
	plan    *plan.Plan
	groups  []safety.Group
	busyMsg string
	result  string

	// dashboard (get-coffee)
	startInDashboard    bool
	dashHome            bool // launched via `suns get-coffee` → back quits
	poller              *telemetry.Poller
	dashCtx             context.Context
	dashCancel          context.CancelFunc
	powerStarted        bool
	powerLauncher       telemetry.Launcher // injectable so tests never spawn sudo
	netLauncher         telemetry.Launcher // injectable; experimental nettop stream
	paused              bool
	procCursor          int    // selected row in the top-processes table
	dashStatus          string // transient status line (kill result / elevate)
	killReturnDashboard bool   // the active modal is a dashboard kill → return to dashboard
}

// New builds the root model.
func New(ctx context.Context, state *config.SessionState) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	return model{
		ctx:           ctx,
		state:         state,
		keys:          defaultKeys(),
		help:          help.New(),
		spinner:       sp,
		powerLauncher: powermetricsLauncher,
		netLauncher:   nettopLauncher,
		screen:        screenMenu,
		menu: []menuItem{
			{"Clean", "Trash dev caches from the safe-cache allowlist", actClean},
			{"Restore", "List trashed files that can be undone", actRestore},
			{"Get Coffee", "Live telemetry dashboard (later phase)", actDashboard},
			{"Audit", "Security posture: SIP / Gatekeeper / FileVault (later phase)", actAudit},
			{"Doctor", "Environment & capability self-check", actDoctor},
			{"Quit", "Exit Suns", actQuit},
		},
	}
}

// NewDashboard builds a model that opens directly on the get-coffee dashboard
// (the `suns get-coffee` entry point); leaving it quits the program.
func NewDashboard(ctx context.Context, state *config.SessionState) model {
	m := New(ctx, state)
	m.startInDashboard = true
	return m
}

func (m model) Init() tea.Cmd {
	if m.startInDashboard {
		return tea.Batch(m.spinner.Tick, func() tea.Msg { return enterDashboardMsg{home: true} })
	}
	return m.spinner.Tick
}

// --- messages from background commands ---

type discoveredMsg struct{ res scanner.Result }
type executedMsg struct{ summary string }
type doctorMsg struct{ rep doctor.Report }
type errMsg struct{ err error }
type enterDashboardMsg struct{ home bool }
type dashTickMsg struct{}
type elevatedMsg struct{ err error }

func (m model) th() theme.Theme { return theme.New(m.state.ConfirmMode()) }

func (m model) deletionObliterate() bool {
	return m.state.DeletionMode() == config.DeletionObliterate
}

func modeFor(state *config.SessionState) operation.Mode {
	if state.DeletionMode() == config.DeletionObliterate {
		return operation.ModeObliterate
	}
	return operation.ModeTrash
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case discoveredMsg:
		return m.onDiscovered(msg.res)

	case executedMsg:
		if m.killReturnDashboard {
			m.killReturnDashboard = false
			m.dashStatus = msg.summary
			m.screen = screenDashboard
			return m, dashTick()
		}
		m.screen = screenResult
		m.result = msg.summary
		return m, nil

	case elevatedMsg:
		if msg.err != nil {
			m.dashStatus = "elevation canceled"
		} else {
			m.dashStatus = "elevated — starting powermetrics"
			// Begin the supervised powermetrics stream (GPU/thermal/power).
			if m.screen == screenDashboard && m.poller != nil && m.dashCtx != nil &&
				m.powerLauncher != nil && !m.powerStarted {
				ps := telemetry.NewPowerSource()
				m.poller.AttachPowerSource(ps)
				go ps.Supervise(m.dashCtx, m.powerLauncher, 3*time.Second)
				m.powerStarted = true
			}
		}
		if m.screen == screenDashboard {
			return m, dashTick()
		}
		return m, nil

	case doctorMsg:
		m.screen = screenResult
		m.result = renderDoctor(msg.rep)
		return m, nil

	case errMsg:
		m.screen = screenResult
		m.result = "Error: " + msg.err.Error()
		return m, nil

	case enterDashboardMsg:
		return m.enterDashboard(msg.home)

	case dashTickMsg:
		if m.screen != screenDashboard {
			return m, nil // dashboard left; stop ticking
		}
		return m, dashTick()

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global keys available everywhere.
	switch {
	case key.Matches(msg, m.keys.Help):
		m.help.ShowAll = !m.help.ShowAll
		return m, nil
	case key.Matches(msg, m.keys.Deathstar):
		m.state.SetConfirmMode(!m.state.ConfirmMode())
		return m, nil
	case key.Matches(msg, m.keys.Jarjar):
		if m.deletionObliterate() {
			m.state.SetDeletionMode(config.DeletionTrash)
		} else {
			m.state.SetDeletionMode(config.DeletionObliterate)
		}
		return m, nil
	}

	switch m.screen {
	case screenMenu:
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, m.keys.Down):
			if m.cursor < len(m.menu)-1 {
				m.cursor++
			}
		case key.Matches(msg, m.keys.Select):
			return m.runAction(m.menu[m.cursor].action)
		}
	case screenModal:
		switch {
		case key.Matches(msg, m.keys.Select):
			m.screen = screenBusy
			m.busyMsg = "Executing plan…"
			return m, executeCmd(m.ctx, m.plan, modeFor(m.state))
		case key.Matches(msg, m.keys.Back):
			if m.killReturnDashboard {
				m.killReturnDashboard = false
				m.screen = screenDashboard
				return m, dashTick()
			}
			m.screen = screenMenu
			return m, nil
		}
	case screenResult:
		// Any of select/back returns to the menu.
		if key.Matches(msg, m.keys.Select) || key.Matches(msg, m.keys.Back) || key.Matches(msg, m.keys.Quit) {
			m.screen = screenMenu
			m.result = ""
			return m, nil
		}
	case screenDashboard:
		switch {
		case key.Matches(msg, m.keys.Back):
			return m.leaveDashboard()
		case key.Matches(msg, m.keys.Quit):
			if m.dashCancel != nil {
				m.dashCancel()
			}
			return m, tea.Quit
		case key.Matches(msg, m.keys.Up):
			if m.procCursor > 0 {
				m.procCursor--
			}
		case key.Matches(msg, m.keys.Down):
			m.procCursor++ // clamped against the live list at render time
		case msg.String() == "p":
			m.paused = !m.paused
		case msg.String() == "e":
			return m, elevateCmd()
		case msg.String() == "k":
			return m.startKill()
		}
	case screenBusy:
		// Busy work is uninterruptible in this slice except for a hard quit.
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) runAction(a action) (tea.Model, tea.Cmd) {
	switch a {
	case actQuit:
		return m, tea.Quit
	case actClean:
		m.screen = screenBusy
		m.busyMsg = "Scanning the safe-cache allowlist…"
		return m, discoverCmd(m.ctx)
	case actDoctor:
		m.screen = screenBusy
		m.busyMsg = "Running checks…"
		return m, doctorCmd(m.ctx)
	case actRestore:
		m.screen = screenResult
		m.result = renderRestoreList()
		return m, nil
	case actDashboard:
		return m.enterDashboard(false)
	case actAudit:
		m.screen = screenResult
		m.result = "This feature lands in a later phase. The safety spine,\n" +
			"the clean/restore core, and the get-coffee dashboard are ready today."
		return m, nil
	}
	return m, nil
}

// enterDashboard starts a telemetry poller under a child context and begins
// the render tick. home=true means the dashboard is the app's home screen
// (launched via `suns get-coffee`), so leaving it quits.
func (m model) enterDashboard(home bool) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(m.ctx)
	p := telemetry.New(time.Second)
	go p.Run(ctx)
	// Start the experimental per-process network stream (unprivileged nettop), so
	// the NET widget warms without needing elevation (§3.3). Supervised: a stall
	// relaunches; the source stays honestly Unavailable until it has two frames.
	if m.netLauncher != nil {
		ns := telemetry.NewNetSource(5)
		p.AttachNetSource(ns)
		go ns.Supervise(ctx, m.netLauncher, 5*time.Second)
	}
	m.poller = p
	m.dashCtx = ctx
	m.dashCancel = cancel
	m.dashHome = home
	m.paused = false
	m.powerStarted = false
	m.screen = screenDashboard
	return m, dashTick()
}

// leaveDashboard cancels the poller and returns to the menu, or quits if the
// dashboard was the home screen.
func (m model) leaveDashboard() (tea.Model, tea.Cmd) {
	if m.dashCancel != nil {
		m.dashCancel()
		m.dashCancel = nil
	}
	m.poller = nil
	if m.dashHome {
		return m, tea.Quit
	}
	m.screen = screenMenu
	return m, nil
}

// dashTick drives the dashboard's ~2 Hz re-render (the data updates at ~1 Hz,
// §3.3); the view reads whatever the poller has published.
func dashTick() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return dashTickMsg{} })
}

// elevateCmd runs `sudo -v` via tea.Exec, which releases the terminal so the
// password prompt renders directly, then restores the TUI (§6.1). A cached
// ticket then lets the privileged process killer run non-interactively.
func elevateCmd() tea.Cmd {
	return tea.ExecProcess(exec.Command("/usr/bin/sudo", "-v"), func(err error) tea.Msg {
		return elevatedMsg{err: err}
	})
}

// startKill builds an identity-checked ProcessKill for the selected process and
// shows the confirmation modal (the gate). Zombies are reported, never killed
// (§12.8). On confirm, executeCmd runs it through the same safety path as any
// destructive operation.
func (m model) startKill() (tea.Model, tea.Cmd) {
	snap := m.poller.Snapshot()
	if snap == nil || len(snap.TopProcs) == 0 {
		return m, nil
	}
	idx := m.procCursor
	if idx >= len(snap.TopProcs) {
		idx = len(snap.TopProcs) - 1
	}
	pr := snap.TopProcs[idx]

	expect, ownerUID, status, err := procctl.Current(int(pr.PID))
	if err != nil {
		m.dashStatus = fmt.Sprintf("PID %d is gone", pr.PID)
		return m, nil
	}
	if strings.Contains(strings.ToLower(status), "zombie") {
		m.dashStatus = fmt.Sprintf("PID %d is a zombie — already dead; reap via its parent", pr.PID)
		return m, nil
	}
	op := operation.ProcessKillOp{
		PID: int(pr.PID), Name: pr.Name, Expect: expect,
		Signal: int(syscall.SIGTERM), Privileged: !procctl.IsOwnUser(ownerUID),
	}
	m.plan = plan.New([]operation.Operation{op}).Seal()
	m.groups = safety.BuildGroups(m.plan, operation.ModeTrash)
	m.killReturnDashboard = true
	m.screen = screenModal
	return m, nil
}

// onDiscovered builds the sealed plan and either shows the modal (safe mode) or
// executes immediately (armed / confirm_mode on), per §4.3.
func (m model) onDiscovered(res scanner.Result) (tea.Model, tea.Cmd) {
	if len(res.Ops) == 0 {
		m.screen = screenResult
		m.result = "Nothing to clean — no allowlisted caches present."
		return m, nil
	}
	p := plan.New(res.Ops).Seal()
	mode := modeFor(m.state)
	m.plan = p
	m.groups = safety.BuildGroups(p, mode)

	if m.state.ConfirmMode() {
		// Armed: skip the gate and execute immediately (post-hoc record).
		m.screen = screenBusy
		m.busyMsg = "Executing plan…"
		return m, executeCmd(m.ctx, p, mode)
	}
	m.screen = screenModal
	return m, nil
}

// --- background commands ---

func discoverCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		man, err := scanner.LoadSafeCacheManifest()
		if err != nil {
			return errMsg{err}
		}
		res, err := scanner.Discover(ctx, man.Targets, scanner.Options{})
		if err != nil {
			return errMsg{err}
		}
		return discoveredMsg{res}
	}
}

func executeCmd(ctx context.Context, p *plan.Plan, mode operation.Mode) tea.Cmd {
	return func() tea.Msg {
		if tr, err := trash.New(); err == nil {
			operation.UseTrasher(tr)
		}
		var log *history.Log
		if path, err := history.DefaultPath(); err == nil {
			log, _ = history.Open(path)
		}
		var trashed, obliterated, killed, skipped int
		var reclaimed int64
		for _, r := range safety.Execute(ctx, p, mode) {
			if log != nil {
				_ = log.Append(r.Entry)
			}
			switch r.Receipt.Fate {
			case "trashed":
				trashed++
				reclaimed += r.Op.Describe().Bytes
			case "obliterated":
				obliterated++
				reclaimed += r.Op.Describe().Bytes
			case "killed":
				killed++
			default:
				skipped++
			}
		}
		var parts []string
		if trashed > 0 {
			parts = append(parts, fmt.Sprintf("trashed %d", trashed))
		}
		if obliterated > 0 {
			parts = append(parts, fmt.Sprintf("obliterated %d", obliterated))
		}
		if killed > 0 {
			parts = append(parts, fmt.Sprintf("killed %d", killed))
		}
		if skipped > 0 {
			parts = append(parts, fmt.Sprintf("skipped %d", skipped))
		}
		s := strings.Join(parts, " · ")
		if s == "" {
			s = "nothing to do"
		}
		if reclaimed > 0 {
			s += fmt.Sprintf(" · %s reclaimed", humanBytes(reclaimed))
		}
		return executedMsg{summary: s}
	}
}

func doctorCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg { return doctorMsg{rep: doctor.Run(ctx)} }
}

func renderDoctor(rep doctor.Report) string {
	var b strings.Builder
	b.WriteString("Doctor\n\n")
	for _, c := range rep.Checks {
		mark := "[ok]"
		switch c.Status {
		case doctor.Warn:
			mark = "[!] "
		case doctor.Fail:
			mark = "[x] "
		}
		fmt.Fprintf(&b, "  %s %-18s %s\n", mark, c.Name, c.Detail)
	}
	return b.String()
}

func renderRestoreList() string {
	path, err := history.DefaultPath()
	if err != nil {
		return "Error: " + err.Error()
	}
	entries, err := history.ReadAll(path)
	if err != nil {
		return "No operation history yet — nothing to restore."
	}
	cands := restore.Candidates(entries)
	if len(cands) == 0 {
		return "Nothing to restore."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d restorable item(s):\n\n", len(cands))
	for i, e := range cands {
		fmt.Fprintf(&b, "  %2d. %s  (%s)\n", i+1, e.OrigPath, humanBytes(e.Size))
	}
	b.WriteString("\n(Restoring from the TUI lands in a later slice; use `suns restore --all`.)")
	return b.String()
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

// compile-time check that model satisfies tea.Model (View is in view.go).
var _ tea.Model = model{}
