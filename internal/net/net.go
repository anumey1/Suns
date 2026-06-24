package net

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// Address-scope classes for a listening socket (§12.12).
const (
	ScopeWildcard = "wildcard" // 0.0.0.0 / :: / *  — externally reachable
	ScopeLoopback = "loopback" // 127.x / ::1        — local only
	ScopeSpecific = "specific" // a particular interface address
)

// Runner is the unprivileged executor for lsof. Production uses syscmd.New();
// tests inject a fake.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Conn is one network socket owned by a process.
type Conn struct {
	PID        int    `json:"pid"`
	Command    string `json:"command"`
	Proto      string `json:"proto"`            // TCP / UDP
	Family     string `json:"family"`           // IPv4 / IPv6
	LocalAddr  string `json:"local_addr"`
	LocalPort  string `json:"local_port"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	RemotePort string `json:"remote_port,omitempty"`
	State      string `json:"state,omitempty"`       // LISTEN, ESTABLISHED, …
	Scope      string `json:"scope,omitempty"`       // set for LISTEN sockets
	Exposed    bool   `json:"exposed,omitempty"`     // LISTEN on a wildcard address
	RemoteHost string `json:"remote_host,omitempty"` // cached reverse-DNS of RemoteAddr
}

// Report is the read-only outcome of Sockets.
type Report struct {
	Conns []Conn `json:"connections"`
}

// Listening returns the LISTEN sockets (the port-audit view), sorted by port.
func (r Report) Listening() []Conn {
	var out []Conn
	for _, c := range r.Conns {
		if c.State == "LISTEN" {
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return portNum(out[i].LocalPort) < portNum(out[j].LocalPort) })
	return out
}

// Active returns sockets with an established remote endpoint (the socket map).
func (r Report) Active() []Conn {
	var out []Conn
	for _, c := range r.Conns {
		if c.RemoteAddr != "" && c.RemoteAddr != "*" {
			out = append(out, c)
		}
	}
	return out
}

// Options controls a scan.
type Options struct {
	ResolveDNS bool          // resolve reverse-DNS for remote addresses
	DNSTimeout time.Duration // per-lookup timeout; 0 → 800ms
}

func (o Options) dnsTimeout() time.Duration {
	if o.DNSTimeout <= 0 {
		return 800 * time.Millisecond
	}
	return o.DNSTimeout
}

// Sockets maps which process owns which live socket by parsing lsof, classifies
// listening ports by reachability, and (optionally) resolves remote addresses to
// hostnames. It is read-only.
func Sockets(ctx context.Context, r Runner, opts Options) (Report, error) {
	// lsof exits non-zero when some handles are inaccessible but still prints the
	// rest; tolerate that as long as we got output.
	res, err := r.Run(ctx, "lsof", "-i", "-n", "-P", "-F", "pcnPtT")
	if err != nil && len(res.Stdout) == 0 {
		return Report{}, fmt.Errorf("net: lsof: %w", err)
	}
	conns := parseLsof(res.Stdout)
	if opts.ResolveDNS {
		resolveHosts(ctx, conns, opts.dnsTimeout())
	}
	sort.SliceStable(conns, func(i, j int) bool {
		if conns[i].PID != conns[j].PID {
			return conns[i].PID < conns[j].PID
		}
		return portNum(conns[i].LocalPort) < portNum(conns[j].LocalPort)
	})
	return Report{Conns: conns}, nil
}

// resolveHosts fills RemoteHost via reverse DNS. Lookups run concurrently with a
// bounded worker pool, are deduplicated by address (the cache), and each is
// capped by a per-lookup timeout so a slow resolver never hangs the command.
func resolveHosts(ctx context.Context, conns []Conn, timeout time.Duration) {
	cache := map[string]string{}
	for _, c := range conns {
		if c.RemoteAddr != "" && c.RemoteAddr != "*" {
			cache[c.RemoteAddr] = ""
		}
	}
	if len(cache) == 0 {
		return
	}

	addrs := make([]string, 0, len(cache))
	for a := range cache {
		addrs = append(addrs, a)
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 16)
	)
	for _, a := range addrs {
		wg.Add(1)
		sem <- struct{}{}
		go func(a string) {
			defer wg.Done()
			defer func() { <-sem }()
			lctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			names, err := net.DefaultResolver.LookupAddr(lctx, a)
			if err == nil && len(names) > 0 {
				mu.Lock()
				cache[a] = trimDot(names[0])
				mu.Unlock()
			}
		}(a)
	}
	wg.Wait()

	for i := range conns {
		if h := cache[conns[i].RemoteAddr]; h != "" {
			conns[i].RemoteHost = h
		}
	}
}

func trimDot(s string) string {
	if len(s) > 0 && s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}

// portNum parses a port string to an int for sorting; "*" and non-numerics sort
// first.
func portNum(p string) int {
	n, err := strconv.Atoi(p)
	if err != nil {
		return -1
	}
	return n
}
