package audit

import "testing"

func TestParseSIP(t *testing.T) {
	cases := []struct {
		in    string
		state string
		sev   Severity
	}{
		{"System Integrity Protection status: enabled.", "enabled", SevOK},
		{"System Integrity Protection status: disabled.", "disabled", SevRisk},
		{"System Integrity Protection status: unknown (Custom Configuration).", "custom", SevWarn},
		{"", "unknown", SevUnknown},
		{"garbage output", "unknown", SevUnknown},
	}
	for _, c := range cases {
		got := parseSIP([]byte(c.in))
		if got.State != c.state || got.Severity != c.sev {
			t.Errorf("parseSIP(%q) = {%s,%s}, want {%s,%s}", c.in, got.State, got.Severity, c.state, c.sev)
		}
	}
}

func TestParseGatekeeper(t *testing.T) {
	cases := []struct {
		in    string
		state string
		sev   Severity
	}{
		{"assessments enabled", "enabled", SevOK},
		{"assessments disabled", "disabled", SevRisk},
		{"", "unknown", SevUnknown},
		{"something else", "unknown", SevUnknown},
	}
	for _, c := range cases {
		got := parseGatekeeper([]byte(c.in))
		if got.State != c.state || got.Severity != c.sev {
			t.Errorf("parseGatekeeper(%q) = {%s,%s}, want {%s,%s}", c.in, got.State, got.Severity, c.state, c.sev)
		}
	}
}

func TestParseFileVault(t *testing.T) {
	cases := []struct {
		in    string
		state string
		sev   Severity
	}{
		{"FileVault is On.", "on", SevOK},
		{"FileVault is Off.", "off", SevWarn},
		{"FileVault is On but a deferred enablement appears to be active.", "deferred", SevWarn},
		{"Encryption in progress", "encrypting", SevWarn},
		{"", "unknown", SevUnknown},
		{"mystery", "unknown", SevUnknown},
	}
	for _, c := range cases {
		got := parseFileVault([]byte(c.in))
		if got.State != c.state || got.Severity != c.sev {
			t.Errorf("parseFileVault(%q) = {%s,%s}, want {%s,%s}", c.in, got.State, got.Severity, c.state, c.sev)
		}
	}
}
