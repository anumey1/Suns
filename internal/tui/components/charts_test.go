package components

import (
	"strings"
	"testing"
)

func TestSparkline_OneRunePerSampleScaled(t *testing.T) {
	got := Sparkline([]float64{0, 50, 100}, 10)
	r := []rune(got)
	if len(r) != 3 {
		t.Fatalf("got %d runes, want 3 (%q)", len(r), got)
	}
	if r[2] != '█' {
		t.Errorf("max sample should render as full block, got %q", string(r[2]))
	}
	if r[0] != '▁' {
		t.Errorf("min sample should render as lowest block, got %q", string(r[0]))
	}
}

func TestSparkline_CapsToWidth(t *testing.T) {
	vals := make([]float64, 100)
	for i := range vals {
		vals[i] = float64(i)
	}
	if got := Sparkline(vals, 10); len([]rune(got)) != 10 {
		t.Fatalf("expected 10 runes, got %d", len([]rune(got)))
	}
}

func TestGauge_FillProportional(t *testing.T) {
	got := Gauge(50, 10)
	if n := strings.Count(got, "█"); n != 5 {
		t.Fatalf("Gauge(50,10) filled = %d, want 5 (%q)", n, got)
	}
	if got := Gauge(0, 10); strings.Count(got, "█") != 0 {
		t.Fatalf("Gauge(0) should be empty: %q", got)
	}
	if got := Gauge(100, 10); strings.Count(got, "█") != 10 {
		t.Fatalf("Gauge(100) should be full: %q", got)
	}
}
