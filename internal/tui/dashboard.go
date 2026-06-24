package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/anumey1/Suns/internal/telemetry"
	"github.com/anumey1/Suns/internal/tui/components"
	"github.com/anumey1/Suns/internal/tui/theme"
)

const tileWidth = 22

// renderDashboard renders the get-coffee live dashboard from the latest
// published snapshot (§11). Cheap sources show live values + sparklines; heavy/
// root sources honestly show a staleness/unavailable badge rather than frozen
// or fake data (§7.4).
func (m model) renderDashboard(h int) string {
	th := m.th()
	snap := m.poller.Snapshot()
	if snap == nil {
		return lipgloss.Place(m.width, h, lipgloss.Center, lipgloss.Center,
			m.spinner.View()+" "+th.MutedText().Render("warming up telemetry…"))
	}

	header := m.dashHeader(snap)
	tiles := m.dashTiles(th, snap)
	procs := m.dashProcs(th, snap)
	hints := th.MutedText().Render("↑/↓ select · k kill (gated) · e elevate · p pause · esc back")

	return strings.Join([]string{header, "", tiles, "", procs, "", hints}, "\n")
}

func (m model) dashHeader(snap *telemetry.SystemSnapshot) string {
	th := m.th()
	left := th.Title().Render("☕ get-coffee")
	info := fmt.Sprintf("uptime %s · load %.2f %.2f %.2f · %s",
		humanUptime(snap.Host.Uptime), snap.Host.Load1, snap.Host.Load5, snap.Host.Load15,
		snap.Time.Format("15:04:05"))
	right := th.MutedText().Render(info)
	if m.paused {
		right = th.Badge("⏸ PAUSED", true) + "  " + right
	}
	if m.dashStatus != "" {
		right = th.NormalText().Render(m.dashStatus) + "  " + right
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m model) dashTiles(th theme.Theme, snap *telemetry.SystemSnapshot) string {
	cpu := tile(th, "CPU", health(snap, telemetry.SrcCPU),
		fmt.Sprintf("%5.1f%%", snap.CPU.Percent),
		components.Sparkline(snap.CPU.History, tileWidth-4))

	mem := tile(th, "MEMORY", health(snap, telemetry.SrcMemory),
		fmt.Sprintf("%5.1f%%  swap %s", snap.Memory.UsedPercent, humanBytes(int64(snap.Memory.SwapUsed))),
		components.Sparkline(snap.Memory.History, tileWidth-4))

	io := tile(th, "DISK I/O", health(snap, telemetry.SrcDisk),
		fmt.Sprintf("R %s/s", humanBytes(int64(snap.DiskIO.ReadBytesPerSec))),
		fmt.Sprintf("W %s/s", humanBytes(int64(snap.DiskIO.WriteBytesPerSec))))

	// Show the fullest volume across all mounted physical volumes (§3.3), so the
	// most space-constrained disk is what surfaces, not just root.
	var diskBody, diskSpark, diskTitle string
	diskTitle = "DISK"
	if len(snap.Disks) > 0 {
		d := fullestVolume(snap.Disks)
		diskBody = fmt.Sprintf("%4.1f%% used", d.UsedPercent)
		diskSpark = components.Gauge(d.UsedPercent, tileWidth-4)
		diskTitle = "DISK " + shortVolume(d.Path)
	}
	space := tile(th, diskTitle, health(snap, telemetry.SrcDisk), diskBody, diskSpark)

	// GPU / thermal / power come from the supervised powermetrics stream once
	// elevated; until then they honestly read N/A (no fake zeros).
	powerLive := snap.Sources[telemetry.SrcPower].Health == telemetry.HealthLive
	gpuBody, gpuHint := "N/A", "press e to elevate"
	thermalBody := "N/A"
	powerBody, powerHint := "N/A", "press e to elevate"
	if powerLive {
		gpuBody, gpuHint = fmt.Sprintf("%4.1f%%", snap.Power.GPUPercent), ""
		if snap.Power.Throttled {
			thermalBody = "throttle: HEAVY"
		} else {
			thermalBody = "throttle: none"
		}
		powerBody = fmt.Sprintf("pkg %.1f W", snap.Power.PackageWatts)
		powerHint = fmt.Sprintf("sys %.1f W", snap.Power.SystemWatts)
	}
	gpu := tile(th, "GPU", health(snap, telemetry.SrcGPU), gpuBody, th.MutedText().Render(gpuHint))
	thermal := tile(th, "THERMAL", health(snap, telemetry.SrcThermal), thermalBody, "")
	power := tile(th, "POWER", health(snap, telemetry.SrcPower), powerBody, th.MutedText().Render(powerHint))

	// Battery via pmset (no elevation needed); N/A on desktops.
	batteryBody, batteryHint := "N/A", ""
	if snap.Battery.Present {
		batteryBody = fmt.Sprintf("%3.0f%% %s", snap.Battery.Percent, snap.Battery.State)
		if snap.Battery.TimeRemaining != "" {
			batteryHint = snap.Battery.TimeRemaining + " left"
		}
	}
	battery := tile(th, "BATTERY", health(snap, telemetry.SrcBattery), batteryBody, th.MutedText().Render(batteryHint))

	row1 := lipgloss.JoinHorizontal(lipgloss.Top, cpu, mem, gpu, thermal)
	row2 := lipgloss.JoinHorizontal(lipgloss.Top, io, space, battery, power)
	return lipgloss.JoinVertical(lipgloss.Left, row1, row2)
}

func (m model) dashProcs(th theme.Theme, snap *telemetry.SystemSnapshot) string {
	var b strings.Builder
	b.WriteString(th.MutedText().Render(fmt.Sprintf("  %-6s %-22s %7s %10s", "PID", "NAME", "CPU%", "MEM")) + "\n")
	for i, p := range snap.TopProcs {
		name := p.Name
		if len(name) > 22 {
			name = name[:21] + "…"
		}
		row := fmt.Sprintf("%-6d %-22s %6.1f%% %10s", p.PID, name, p.CPU, humanBytes(int64(p.Mem)))
		if i == m.procCursor {
			b.WriteString(th.Selected().Render("› "+row) + "\n")
		} else {
			b.WriteString(th.NormalText().Render("  "+row) + "\n")
		}
	}
	return b.String()
}

// tile renders one bordered widget with a title, a health badge, and up to two
// body lines.
func tile(th theme.Theme, title, badge, line1, line2 string) string {
	head := th.Title().Render(title) + " " + badge
	body := th.NormalText().Render(line1)
	if line2 != "" {
		body += "\n" + th.MutedText().Render(line2)
	}
	return th.Border(false).Padding(0, 1).Width(tileWidth).Render(head + "\n" + body)
}

// health maps a source's supervised state to a compact badge (§7.4).
func health(snap *telemetry.SystemSnapshot, src string) string {
	st := snap.Sources[src]
	switch st.Health {
	case telemetry.HealthLive:
		// Stale if the last sample is older than ~3s.
		if !st.LastSample.IsZero() && time.Since(st.LastSample) > 3*time.Second {
			return "⚠ stale"
		}
		return "🟢"
	case telemetry.HealthStale, telemetry.HealthStalled:
		return "⚠ stale"
	case telemetry.HealthRestarting:
		return "↻"
	default:
		return "⚠ N/A"
	}
}

// fullestVolume picks the volume with the highest used-percentage, preferring
// root on a tie so the tile stays stable when volumes are equally empty.
func fullestVolume(disks []telemetry.DiskUsage) telemetry.DiskUsage {
	best := disks[0]
	for _, d := range disks[1:] {
		if d.UsedPercent > best.UsedPercent {
			best = d
		}
	}
	return best
}

// shortVolume labels a mountpoint compactly for the tile title: "/" stays "/",
// "/Volumes/Data" becomes "Data".
func shortVolume(path string) string {
	if path == "/" {
		return "/"
	}
	if i := strings.LastIndex(path, "/"); i >= 0 && i+1 < len(path) {
		return path[i+1:]
	}
	return path
}

func humanUptime(sec uint64) string {
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if h >= 24 {
		return fmt.Sprintf("%dd%dh", h/24, h%24)
	}
	return fmt.Sprintf("%dh%dm", h, mins)
}
