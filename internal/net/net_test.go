package net

import (
	"context"
	"testing"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// A representative `lsof -i -n -P -F pcnPtT` capture: a wildcard TCP listener, a
// loopback listener, an IPv6 listener, an established outbound connection, and a
// UDP socket.
const lsofFixture = `p1
claunchd
f6
tIPv4
PTCP
n*:22
TST=LISTEN
p501
cPostgres
f7
tIPv4
PTCP
n127.0.0.1:5432
TST=LISTEN
f8
tIPv6
PTCP
n[::1]:631
TST=LISTEN
p888
cssh
f3
tIPv4
PTCP
n192.168.1.10:54321->93.184.216.34:443
TST=ESTABLISHED
p999
cmDNSResponder
f5
tIPv4
PUDP
n*:5353
`

func parse(t *testing.T) Report {
	t.Helper()
	fr := fakeRunner{out: lsofFixture}
	rep, err := Sockets(context.Background(), fr, Options{ResolveDNS: false})
	if err != nil {
		t.Fatalf("Sockets: %v", err)
	}
	return rep
}

type fakeRunner struct {
	out string
	err error
}

func (f fakeRunner) Run(_ context.Context, _ string, _ ...string) (syscmd.Result, error) {
	return syscmd.Result{Stdout: []byte(f.out)}, f.err
}

func byPort(conns []Conn, port string) *Conn {
	for i := range conns {
		if conns[i].LocalPort == port {
			return &conns[i]
		}
	}
	return nil
}

func TestSockets_ParsesAllRecords(t *testing.T) {
	rep := parse(t)
	if len(rep.Conns) != 5 {
		t.Fatalf("parsed %d conns, want 5: %+v", len(rep.Conns), rep.Conns)
	}
}

func TestSockets_ListeningScopes(t *testing.T) {
	lis := parse(t).Listening()
	if len(lis) != 3 {
		t.Fatalf("listening = %d, want 3", len(lis))
	}
	// Sorted by port: 22, 631, 5432.
	if c := byPort(lis, "22"); c == nil || c.Scope != ScopeWildcard || !c.Exposed {
		t.Errorf("port 22 should be wildcard+exposed: %+v", c)
	}
	if c := byPort(lis, "5432"); c == nil || c.Scope != ScopeLoopback || c.Exposed {
		t.Errorf("port 5432 should be loopback, not exposed: %+v", c)
	}
	if c := byPort(lis, "631"); c == nil || c.Scope != ScopeLoopback {
		t.Errorf("port 631 ([::1]) should be loopback: %+v", c)
	}
}

func TestSockets_EstablishedRemote(t *testing.T) {
	act := parse(t).Active()
	if len(act) != 1 {
		t.Fatalf("active = %d, want 1", len(act))
	}
	c := act[0]
	if c.Command != "ssh" || c.LocalAddr != "192.168.1.10" || c.LocalPort != "54321" {
		t.Errorf("local end wrong: %+v", c)
	}
	if c.RemoteAddr != "93.184.216.34" || c.RemotePort != "443" || c.State != "ESTABLISHED" {
		t.Errorf("remote end wrong: %+v", c)
	}
}

func TestSockets_UDPHasNoState(t *testing.T) {
	udp := byPort(parse(t).Conns, "5353")
	if udp == nil || udp.Proto != "UDP" || udp.State != "" {
		t.Errorf("UDP socket parsed wrong: %+v", udp)
	}
}

func TestSplitAddrPort(t *testing.T) {
	cases := []struct{ in, addr, port string }{
		{"*:22", "*", "22"},
		{"127.0.0.1:5432", "127.0.0.1", "5432"},
		{"[::1]:631", "::1", "631"},
		{"[fe80::1]:0", "fe80::1", "0"},
		{"*:*", "*", "*"},
	}
	for _, c := range cases {
		a, p := splitAddrPort(c.in)
		if a != c.addr || p != c.port {
			t.Errorf("splitAddrPort(%q) = (%q,%q), want (%q,%q)", c.in, a, p, c.addr, c.port)
		}
	}
}

func TestSockets_ToleratesLsofPartialExit(t *testing.T) {
	// lsof returns a non-nil error (exit 1) but still prints usable output.
	fr := fakeRunner{out: lsofFixture, err: context.DeadlineExceeded}
	rep, err := Sockets(context.Background(), fr, Options{ResolveDNS: false})
	if err != nil {
		t.Fatalf("should tolerate partial lsof exit when output is present: %v", err)
	}
	if len(rep.Conns) != 5 {
		t.Fatalf("parsed %d, want 5", len(rep.Conns))
	}
}
