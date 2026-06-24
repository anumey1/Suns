package net

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// LAN scanner (§12.6). It is PASSIVE by default: it reads the host's ARP cache
// (devices this Mac has recently talked to) and enriches each entry with a
// curated-subset OUI vendor and a reverse-DNS hostname. It does not promise every
// device on the network. Active TCP port probing is opt-in and gated behind an
// explicit warning, because scanning hosts you do not own may be unwelcome or
// unlawful.

// arpLineRE matches a macOS `arp -a -n` row: "? (192.168.1.1) at a4:b1:.. on en0 …".
var arpLineRE = regexp.MustCompile(`\(([0-9.]+)\) at ([0-9a-fA-F:]+) on (\w+)`)

// defaultProbePorts are the common TCP ports tried when active probing is enabled.
var defaultProbePorts = []int{21, 22, 53, 80, 139, 443, 445, 3389, 5900, 8080}

// Device is one host discovered on the LAN.
type Device struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Vendor    string `json:"vendor,omitempty"`
	Hostname  string `json:"hostname,omitempty"`
	Interface string `json:"interface,omitempty"`
	OpenPorts []int  `json:"open_ports,omitempty"`
}

// LANReport is the read-only outcome of LANScan.
type LANReport struct {
	Devices []Device `json:"devices"`
	Probed  bool     `json:"probed"`
}

// LANOptions controls a LAN scan.
type LANOptions struct {
	ResolveDNS   bool          // reverse-resolve device hostnames
	Probe        bool          // active TCP port probe (caller must warn first)
	Ports        []int         // probe ports; nil → defaultProbePorts
	DNSTimeout   time.Duration // per reverse-lookup; 0 → 800ms
	ProbeTimeout time.Duration // per TCP dial; 0 → 400ms
}

func (o LANOptions) ports() []int {
	if len(o.Ports) == 0 {
		return defaultProbePorts
	}
	return o.Ports
}

func (o LANOptions) dnsTimeout() time.Duration {
	if o.DNSTimeout <= 0 {
		return 800 * time.Millisecond
	}
	return o.DNSTimeout
}

func (o LANOptions) probeTimeout() time.Duration {
	if o.ProbeTimeout <= 0 {
		return 400 * time.Millisecond
	}
	return o.ProbeTimeout
}

// LANScan reads the ARP cache and assembles the device list. Hostname resolution
// and port probing are optional.
func LANScan(ctx context.Context, r Runner, opts LANOptions) (LANReport, error) {
	res, err := r.Run(ctx, "arp", "-a", "-n")
	if err != nil && len(res.Stdout) == 0 {
		return LANReport{}, fmt.Errorf("net: arp: %w", err)
	}
	devs := parseARP(res.Stdout)
	if opts.ResolveDNS {
		resolveDeviceHosts(ctx, devs, opts.dnsTimeout())
	}
	if opts.Probe {
		probeDevices(ctx, devs, opts.ports(), opts.probeTimeout())
	}
	sortDevices(devs)
	return LANReport{Devices: devs, Probed: opts.Probe}, nil
}

// parseARP turns `arp -a -n` output into devices, skipping incomplete entries
// and broadcast/multicast MACs (which are not real hosts).
func parseARP(out []byte) []Device {
	var devs []Device
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "incomplete") {
			continue
		}
		m := arpLineRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		mac := normalizeMAC(m[2])
		if mac == "" || isMulticastOrBroadcast(mac) {
			continue
		}
		devs = append(devs, Device{
			IP:        m[1],
			MAC:       mac,
			Interface: m[3],
			Vendor:    lookupVendor(mac),
		})
	}
	return devs
}

// isMulticastOrBroadcast reports whether a MAC's group bit (LSB of the first
// octet) is set — true for broadcast (ff:…) and IPv4/IPv6 multicast (01:00:5e,
// 33:33), none of which are individual hosts.
func isMulticastOrBroadcast(mac string) bool {
	octets := strings.SplitN(mac, ":", 2)
	first, err := strconv.ParseUint(octets[0], 16, 8)
	if err != nil {
		return true
	}
	return first&0x01 == 1
}

// resolveDeviceHosts fills Hostname via bounded, cached reverse DNS (macOS routes
// .local reverse lookups through mDNSResponder).
func resolveDeviceHosts(ctx context.Context, devs []Device, timeout time.Duration) {
	cache := map[string]string{}
	for _, d := range devs {
		cache[d.IP] = ""
	}
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 16)
	)
	for ip := range cache {
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			lctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if names, err := net.DefaultResolver.LookupAddr(lctx, ip); err == nil && len(names) > 0 {
				mu.Lock()
				cache[ip] = trimDot(names[0])
				mu.Unlock()
			}
		}(ip)
	}
	wg.Wait()
	for i := range devs {
		devs[i].Hostname = cache[devs[i].IP]
	}
}

// probeDevices attempts a TCP connect to each port of each device, recording the
// open ones. Concurrency is bounded and each dial is capped by timeout.
func probeDevices(ctx context.Context, devs []Device, ports []int, timeout time.Duration) {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		sem = make(chan struct{}, 64)
	)
	for di := range devs {
		for _, p := range ports {
			wg.Add(1)
			sem <- struct{}{}
			go func(di, port int) {
				defer wg.Done()
				defer func() { <-sem }()
				d := net.Dialer{Timeout: timeout}
				conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(devs[di].IP, strconv.Itoa(port)))
				if err != nil {
					return
				}
				_ = conn.Close()
				mu.Lock()
				devs[di].OpenPorts = append(devs[di].OpenPorts, port)
				mu.Unlock()
			}(di, p)
		}
	}
	wg.Wait()
	for i := range devs {
		sort.Ints(devs[i].OpenPorts)
	}
}

// sortDevices orders devices by IP address numerically.
func sortDevices(devs []Device) {
	sort.SliceStable(devs, func(i, j int) bool {
		a, b := net.ParseIP(devs[i].IP), net.ParseIP(devs[j].IP)
		if a != nil && b != nil {
			return bytes.Compare(a.To16(), b.To16()) < 0
		}
		return devs[i].IP < devs[j].IP
	})
}
