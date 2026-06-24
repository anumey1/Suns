package net

import (
	"context"
	"testing"
)

// A representative `arp -a -n` capture: a router, two hosts (one with a
// leading-zero-trimmed MAC), an incomplete entry, and the broadcast row.
const arpFixture = `? (192.168.1.1) at 0:11:22:33:44:55 on en0 ifscope [ethernet]
? (192.168.1.20) at b8:27:eb:aa:bb:cc on en0 ifscope [ethernet]
? (192.168.1.9) at (incomplete) on en0 ifscope [ethernet]
? (192.168.1.255) at ff:ff:ff:ff:ff:ff on en0 ifscope [ethernet]
? (224.0.0.251) at 1:0:5e:0:0:fb on en0 ifscope permanent [ethernet]
`

func TestLANScan_ParsesAndFiltersARP(t *testing.T) {
	rep, err := LANScan(context.Background(), fakeRunner{out: arpFixture}, LANOptions{})
	if err != nil {
		t.Fatalf("LANScan: %v", err)
	}
	// Only the router and the Pi are real unicast hosts; incomplete, broadcast,
	// and multicast rows are dropped.
	if len(rep.Devices) != 2 {
		t.Fatalf("devices = %d, want 2: %+v", len(rep.Devices), rep.Devices)
	}
	// Sorted by IP: .1 then .20.
	if rep.Devices[0].IP != "192.168.1.1" || rep.Devices[1].IP != "192.168.1.20" {
		t.Errorf("device order wrong: %+v", rep.Devices)
	}
	// Leading-zero MAC normalized.
	if rep.Devices[0].MAC != "00:11:22:33:44:55" {
		t.Errorf("MAC = %q, want zero-padded", rep.Devices[0].MAC)
	}
	// Raspberry Pi OUI from the curated table.
	if rep.Devices[1].Vendor == "" {
		t.Errorf("B8:27:EB should resolve to a vendor in the curated table")
	}
}

func TestLookupVendor(t *testing.T) {
	cases := map[string]string{
		"B8:27:EB:00:00:01": "Raspberry Pi Foundation",
		"b8:27:eb:00:00:01": "Raspberry Pi Foundation", // case-insensitive
		"00:1B:63:00:00:01": "Apple",
		"AA:BB:CC:DD:EE:FF": "", // not in the curated subset
	}
	for mac, want := range cases {
		if got := lookupVendor(mac); got != want {
			t.Errorf("lookupVendor(%q) = %q, want %q", mac, got, want)
		}
	}
}

func TestNormalizeMAC(t *testing.T) {
	cases := map[string]string{
		"0:11:22:33:44:55":  "00:11:22:33:44:55",
		"a4:b1:c2:d3:e4:f5": "A4:B1:C2:D3:E4:F5",
		"00:1B:63":          "00:1B:63",
	}
	for in, want := range cases {
		if got := normalizeMAC(in); got != want {
			t.Errorf("normalizeMAC(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsMulticastOrBroadcast(t *testing.T) {
	cases := map[string]bool{
		"FF:FF:FF:FF:FF:FF": true,  // broadcast
		"01:00:5E:00:00:FB": true,  // IPv4 multicast
		"33:33:00:00:00:01": true,  // IPv6 multicast
		"00:11:22:33:44:55": false, // unicast
		"B8:27:EB:00:00:01": false, // unicast
	}
	for mac, want := range cases {
		if got := isMulticastOrBroadcast(mac); got != want {
			t.Errorf("isMulticastOrBroadcast(%q) = %v, want %v", mac, got, want)
		}
	}
}
