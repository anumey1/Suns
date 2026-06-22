// Package theme holds the lipgloss palettes and mode-based accent recoloring
// (§10.2). Two coordinated palettes exist; the active confirm_mode recolors the
// entire chrome — cool plasma blue (#4FC3F7) when safe, aggressive ember red
// (#FF3B30) when armed — so the operator's safety posture is impossible to miss.
//
// It uses lipgloss.AdaptiveColor for light terminals, respects NO_COLOR
// (degrading to monochrome-plus-symbols), detects truecolor/256/16-color, and
// provides the Apple_Terminal sparkline (block-element) fallback. The Jarjar
// deletion-mode chip (🗑 TRASH / 🔥 OBLITERATE) is independent of the accent.
package theme
