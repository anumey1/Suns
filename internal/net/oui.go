package net

import (
	"bufio"
	"bytes"
	"strings"
	"sync"

	"github.com/anumey1/Suns/assets"
)

// OUI vendor lookup against the embedded curated subset (assets/oui.csv). The
// table is a deliberate common-vendor subset, not the full IEEE registry, so an
// absent prefix yields "" (rendered as "unknown vendor") rather than a guess.

var (
	ouiOnce sync.Once
	ouiMap  map[string]string
)

func ouiTable() map[string]string {
	ouiOnce.Do(func() {
		ouiMap = map[string]string{}
		sc := bufio.NewScanner(bytes.NewReader(assets.OUIDatabase))
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, ",", 2)
			if len(parts) != 2 {
				continue
			}
			prefix := normalizeMAC(parts[0])
			vendor := strings.TrimSpace(parts[1])
			if prefix != "" && vendor != "" && vendor != "vendor" { // skip a stray header row
				ouiMap[prefix] = vendor
			}
		}
	})
	return ouiMap
}

// lookupVendor returns the vendor for a MAC's OUI prefix, or "" if not in the
// curated table.
func lookupVendor(mac string) string {
	p := macPrefix(mac)
	if p == "" {
		return ""
	}
	return ouiTable()[p]
}

// macPrefix returns the normalized first three octets of a MAC ("AA:BB:CC").
func macPrefix(mac string) string {
	octets := strings.Split(mac, ":")
	if len(octets) < 3 {
		return ""
	}
	return normalizeOctets(octets[:3])
}

// normalizeMAC normalizes a full or partial MAC to uppercase, zero-padded
// colon-separated octets.
func normalizeMAC(mac string) string {
	return normalizeOctets(strings.Split(strings.TrimSpace(mac), ":"))
}

func normalizeOctets(octets []string) string {
	out := make([]string, 0, len(octets))
	for _, o := range octets {
		o = strings.TrimSpace(o)
		if o == "" {
			return ""
		}
		if len(o) == 1 {
			o = "0" + o
		}
		out = append(out, strings.ToUpper(o))
	}
	return strings.Join(out, ":")
}
