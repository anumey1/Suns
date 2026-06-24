package net

import (
	"strconv"
	"strings"
)

// Parsing contract for `lsof -i -n -P -F pcnPtT` (§13.1). The -F (field) mode is
// used instead of the columnar default because it is unambiguous: each line is a
// single field, prefixed by a one-character tag, so a command name containing
// spaces (or an unusual address form) cannot misalign the parse. Records stream
// as a process block (`p`<pid>, `c`<command>) followed by one file block per
// socket (`f`<fd>, `t`<family>, `P`<proto>, `n`<name>, `T`ST=<state>).
//
// `-n -P` disables host/port name resolution, so addresses and ports are numeric
// and locale-independent. Unexpected lines are ignored rather than guessed.

// parseLsof turns lsof field output into connection records.
func parseLsof(out []byte) []Conn {
	var conns []Conn
	var pid int
	var command string
	var cur *Conn

	flush := func() {
		if cur != nil && (cur.LocalAddr != "" || cur.LocalPort != "") {
			conns = append(conns, *cur)
		}
		cur = nil
	}

	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		tag, val := line[0], line[1:]
		switch tag {
		case 'p':
			flush()
			pid, _ = strconv.Atoi(val)
			command = ""
		case 'c':
			command = val
		case 'f': // a new file (socket) block begins
			flush()
			cur = &Conn{PID: pid, Command: command}
		case 't':
			if cur != nil {
				cur.Family = val
			}
		case 'P':
			if cur != nil {
				cur.Proto = val
			}
		case 'n':
			if cur != nil {
				setName(cur, val)
			}
		case 'T':
			if cur != nil && strings.HasPrefix(val, "ST=") {
				cur.State = val[len("ST="):]
			}
		}
	}
	flush()

	for i := range conns {
		if conns[i].State == "LISTEN" {
			conns[i].Scope = addrScope(conns[i].LocalAddr)
			conns[i].Exposed = conns[i].Scope == ScopeWildcard
		}
	}
	return conns
}

// setName splits an lsof NAME ("local" or "local->remote") into address/port
// pairs, tolerating IPv6 bracket form and the "*" wildcard.
func setName(c *Conn, name string) {
	if i := strings.Index(name, "->"); i >= 0 {
		c.LocalAddr, c.LocalPort = splitAddrPort(name[:i])
		c.RemoteAddr, c.RemotePort = splitAddrPort(name[i+2:])
		return
	}
	c.LocalAddr, c.LocalPort = splitAddrPort(name)
}

// splitAddrPort parses "addr:port", "[ipv6]:port", "*:port", or "*:*".
func splitAddrPort(s string) (addr, port string) {
	if s == "" {
		return "", ""
	}
	if strings.HasPrefix(s, "[") {
		if i := strings.LastIndex(s, "]"); i >= 0 {
			return s[1:i], strings.TrimPrefix(s[i+1:], ":")
		}
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// addrScope classifies a local listening address for the port auditor (§12.12).
func addrScope(addr string) string {
	switch addr {
	case "*", "0.0.0.0", "::":
		return ScopeWildcard
	case "127.0.0.1", "::1":
		return ScopeLoopback
	}
	if strings.HasPrefix(addr, "127.") {
		return ScopeLoopback
	}
	return ScopeSpecific
}
