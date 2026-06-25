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

func TestParseNettopHeaderCSV(t *testing.T) {
	cols, ok := parseNettopHeader("time,,interface,state,bytes_in,bytes_out,rx_dupe")
	if !ok || cols.format != nettopFmtCSV || cols.inIdx != 4 || cols.outIdx != 5 {
		t.Errorf("cols = %+v, want CSV (4,5)", cols)
	}
	_, ok = parseNettopHeader("time,,interface,state")
	if ok {
		t.Error("header missing bytes_in/bytes_out should fail")
	}
}

func TestParseNettopHeaderFixed(t *testing.T) {
	// Real fixed-width header from macOS `nettop -P -x -l 0 -s 1`.
	h := "time                                                                                                              interface         state        bytes_in       bytes_out   rx_dupe    rx_ooo     re-tx   rtt_avg   rcvsize    tx_win  tc_class    tc_mgt   cc_algo P C R W arch"
	cols, ok := parseNettopHeader(h)
	if !ok {
		t.Fatal("fixed-width header should parse")
	}
	if cols.format != nettopFmtFixed {
		t.Errorf("format = %d, want nettopFmtFixed", cols.format)
	}
	if cols.inStart <= 0 || cols.inEnd <= cols.inStart {
		t.Errorf("bytes_in range = [%d:%d], want positive span", cols.inStart, cols.inEnd)
	}
	if cols.outStart < cols.inEnd || cols.outEnd <= cols.outStart {
		t.Errorf("bytes_out range = [%d:%d], want positive span after bytes_in (%d)", cols.outStart, cols.outEnd, cols.inEnd)
	}
}

func TestParseNetRowCSVDegrades(t *testing.T) {
	if _, _, ok := parseNetRowCSV("not,a,real,row,here,now", 4, 5); ok {
		t.Error("non-numeric timestamp should not parse")
	}
	if _, _, ok := parseNetRowCSV("02:00:01.000000,foo.5,,,7,9", 4, 5); !ok {
		t.Error("well-formed row should parse")
	}
}

func TestParseNetRowFixed(t *testing.T) {
	// Data rows from real macOS nettop fixed-width output.
	header := "time                                                                                                              interface         state        bytes_in       bytes_out   rx_dupe    rx_ooo     re-tx   rtt_avg   rcvsize    tx_win  tc_class    tc_mgt   cc_algo P C R W arch"
	inStart := strings.Index(header, "bytes_in")
	inEnd := strings.Index(header, "bytes_out")
	outStart := inEnd
	outEnd := strings.Index(header, "rx_dupe")

	// Single-word process name.
	label, row, ok := parseNetRowFixed("23:54:06.466526 syslogd.557                                                                                                                             0             793         0         0         0", inStart, inEnd, outStart, outEnd)
	if !ok {
		t.Fatal("single-word process row should parse")
	}
	if label != "syslogd.557" || row.name != "syslogd" || row.pid != 557 {
		t.Errorf("label=%q name=%q pid=%d, want syslogd.557 / syslogd / 557", label, row.name, row.pid)
	}
	if row.in != 0 || row.out != 793 {
		t.Errorf("in=%d out=%d, want 0 / 793", row.in, row.out)
	}

	// Multi-word process name (Google Chrome H.1323).
	label2, row2, ok2 := parseNetRowFixed("23:54:06.466534 Google Chrome H.1323                                                                                                             39026157          832339         0      2276         0", inStart, inEnd, outStart, outEnd)
	if !ok2 {
		t.Fatal("multi-word process row should parse")
	}
	if label2 != "Google Chrome H.1323" || row2.name != "Google Chrome H" || row2.pid != 1323 {
		t.Errorf("label=%q name=%q pid=%d, want Google Chrome H.1323 / Google Chrome H / 1323", label2, row2.name, row2.pid)
	}
	if row2.in != 39026157 || row2.out != 832339 {
		t.Errorf("in=%d out=%d, want 39026157 / 832339", row2.in, row2.out)
	}

	// Degrade: bad timestamp.
	if _, _, ok := parseNetRowFixed("not a timestamp syslogd.557                                                                                                                             0             793", inStart, inEnd, outStart, outEnd); ok {
		t.Error("bad timestamp should not parse")
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
