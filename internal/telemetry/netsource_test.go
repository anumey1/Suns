package telemetry

import (
	"context"
	"io"
	"math"
	"strings"
	"testing"
	"time"
)

// netStreamFixture is three `nettop -P -x -l 0` frames. The byte columns are
// cumulative, so consecutive frames are differenced. mDNSResponder grows by
// 2596 in / 932 out over the 1.0s between frame 1 and 2; "idle" never changes so
// it must be dropped. The trailing frame has no closing header, so only one diff
// (frame1→frame2) is emitted.
const netStreamFixture = `time,,interface,state,bytes_in,bytes_out,rx_dupe
02:00:01.000000,mDNSResponder.647,,,1000,500,0
02:00:01.000000,idle.10,,,42,42,0
time,,interface,state,bytes_in,bytes_out,rx_dupe
02:00:02.000000,mDNSResponder.647,,,3596,1432,0
02:00:02.000000,idle.10,,,42,42,0
time,,interface,state,bytes_in,bytes_out,rx_dupe
02:00:03.000000,mDNSResponder.647,,,9000,2000,0
02:00:03.000000,idle.10,,,42,42,0
`

func TestNetSourceRunDiffsFrames(t *testing.T) {
	ns := NewNetSource(5)
	err := ns.run(context.Background(), strings.NewReader(netStreamFixture), time.Second)
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	procs, st := ns.Latest()
	if st.Health != HealthLive {
		t.Fatalf("health = %v, want live", st.Health)
	}
	if len(procs) != 1 {
		t.Fatalf("want 1 talker (idle dropped), got %d: %+v", len(procs), procs)
	}
	p := procs[0]
	if p.Name != "mDNSResponder" || p.PID != 647 {
		t.Fatalf("label parse wrong: %q / %d", p.Name, p.PID)
	}
	if math.Abs(p.RxBytesPerSec-2596) > 0.5 {
		t.Errorf("rx = %v, want ~2596", p.RxBytesPerSec)
	}
	if math.Abs(p.TxBytesPerSec-932) > 0.5 {
		t.Errorf("tx = %v, want ~932", p.TxBytesPerSec)
	}
}

func TestNetSourceTopNCap(t *testing.T) {
	prev := map[string]netRow{
		"a.1": {ts: t0(0), in: 0, out: 0, name: "a", pid: 1},
		"b.2": {ts: t0(0), in: 0, out: 0, name: "b", pid: 2},
	}
	cur := map[string]netRow{
		"a.1": {ts: t0(1), in: 100, out: 0, name: "a", pid: 1},
		"b.2": {ts: t0(1), in: 999, out: 0, name: "b", pid: 2},
	}
	got := diffNetFrames(prev, cur, 1)
	if len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("want only top talker b, got %+v", got)
	}
}

func TestNetSourceStall(t *testing.T) {
	// A reader that yields a header then blocks forever must trip the deadline and
	// mark the source stalled, not hang the run loop.
	pr, pw := io.Pipe()
	defer pw.Close()
	go func() { _, _ = pw.Write([]byte("time,,interface,state,bytes_in,bytes_out\n")) }()
	ns := NewNetSource(5)
	done := make(chan error, 1)
	go func() { done <- ns.run(context.Background(), pr, 100*time.Millisecond) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stall run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("run did not return on stall")
	}
	if _, st := ns.Latest(); st.Health != HealthStalled {
		t.Errorf("health = %v, want stalled", st.Health)
	}
}

func TestNettopByteCols(t *testing.T) {
	in, out := nettopByteCols("time,,interface,state,bytes_in,bytes_out,rx_dupe")
	if in != 4 || out != 5 {
		t.Errorf("cols = (%d,%d), want (4,5)", in, out)
	}
	if in, out := nettopByteCols("time,,interface,state"); in != -1 || out != -1 {
		t.Errorf("missing cols = (%d,%d), want (-1,-1)", in, out)
	}
}

func TestParseNetRowDegrades(t *testing.T) {
	if _, _, ok := parseNetRow("not,a,real,row,here,now", 4, 5); ok {
		t.Error("non-numeric timestamp should not parse")
	}
	if _, _, ok := parseNetRow("02:00:01.000000,foo.5,,,7,9", 4, 5); !ok {
		t.Error("well-formed row should parse")
	}
}

func TestSplitNettopLabel(t *testing.T) {
	if n, p := splitNettopLabel("Google Chrome H.1323"); n != "Google Chrome H" || p != 1323 {
		t.Errorf("got (%q,%d)", n, p)
	}
	if n, p := splitNettopLabel("nopid"); n != "nopid" || p != 0 {
		t.Errorf("got (%q,%d)", n, p)
	}
}

func t0(sec int) time.Time {
	return time.Date(2000, 1, 1, 2, 0, sec, 0, time.UTC)
}
