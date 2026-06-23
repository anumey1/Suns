// Package components holds reusable TUI rendering primitives (§10.4).
package components

import "strings"

// blocks are the eight vertical block elements. Block-element charts render
// correctly everywhere, including Apple's Terminal.app (which has a documented
// Braille line-height bug), so they are the safe default; richer Braille charts
// via ntcharts are a later enhancement gated on TERM_PROGRAM (§2.2, §10.4).
var blocks = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders the most recent values as a block-element rolling chart of
// at most width columns, auto-scaled to the maximum sample.
func Sparkline(values []float64, width int) string {
	if width <= 0 || len(values) == 0 {
		return ""
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	max := 0.0
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}
	var b strings.Builder
	for _, v := range values {
		idx := int(v/max*float64(len(blocks)-1) + 0.5)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(blocks) {
			idx = len(blocks) - 1
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}

// Gauge renders a horizontal fill bar for percent (0–100) of the given width
// using full/light block shading.
func Gauge(percent float64, width int) string {
	if width < 1 {
		width = 1
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent/100*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
