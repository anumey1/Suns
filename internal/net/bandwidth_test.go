package net

import (
	"math"
	"testing"
)

// A two-sample `nettop -P -x -L 2` capture (trimmed to the columns we use). The
// header names the byte columns; each process appears once per sample with the
// row's own timestamp leading. mDNSResponder shows ~1s apart with growing
// counters; the second process appears only once and must be dropped.
const nettopFixture = `time,,interface,state,bytes_in,bytes_out,rx_dupe,rx_ooo,re-tx,rtt_avg
23:19:02.000000,mDNSResponder.647,,,62752801,45852241,0,0,0,0
23:19:02.000000,launchd.1,,,1000,2000,0,0,0,0
23:19:03.000000,mDNSResponder.647,,,62755397,45853173,0,0,0,0
`

func TestParseNettopRates(t *testing.T) {
	procs := parseNettop([]byte(nettopFixture))
	if len(procs) != 1 {
		t.Fatalf("want 1 process with two samples, got %d: %+v", len(procs), procs)
	}
	p := procs[0]
	if p.Name != "mDNSResponder" || p.PID != 647 {
		t.Fatalf("label split wrong: name=%q pid=%d", p.Name, p.PID)
	}
	// Δin = 62755397-62752801 = 2596 over 1.0s; Δout = 45853173-45852241 = 932.
	if math.Abs(p.RxBytesPerSec-2596) > 0.5 {
		t.Errorf("rx rate = %v, want ~2596", p.RxBytesPerSec)
	}
	if math.Abs(p.TxBytesPerSec-932) > 0.5 {
		t.Errorf("tx rate = %v, want ~932", p.TxBytesPerSec)
	}
}

func TestParseNettopMissingColumns(t *testing.T) {
	// Header without byte columns → degrade to nil, never panic.
	if got := parseNettop([]byte("time,,interface,state\n23:19:02.000000,foo.1,,\n")); got != nil {
		t.Errorf("want nil on missing byte columns, got %+v", got)
	}
}

func TestParseNettopEmpty(t *testing.T) {
	if got := parseNettop(nil); got != nil {
		t.Errorf("want nil on empty input, got %+v", got)
	}
}

func TestSplitNamePID(t *testing.T) {
	cases := []struct {
		label string
		name  string
		pid   int
	}{
		{"mDNSResponder.647", "mDNSResponder", 647},
		{"com.apple.WebKit.Networking.1234", "com.apple.WebKit.Networking", 1234},
		{"nopid", "nopid", 0},
	}
	for _, c := range cases {
		name, pid := splitNamePID(c.label)
		if name != c.name || pid != c.pid {
			t.Errorf("splitNamePID(%q) = (%q,%d), want (%q,%d)", c.label, name, pid, c.name, c.pid)
		}
	}
}
