package scheduler

import (
	"strings"
	"testing"
)

func TestGeneratePlist_ContainsConstrainedInvocation(t *testing.T) {
	data, err := GeneratePlist(Config{BinaryPath: "/usr/local/bin/suns", Hour: 3, Minute: 30})
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)

	for _, want := range []string{
		Label,
		"/usr/local/bin/suns",
		"<string>clean</string>",
		"<string>--scheduled</string>",
		"StartCalendarInterval",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q\n%s", want, s)
		}
	}
	// RunAtLoad must be false — no burn at install/login. It is the only boolean
	// in the document, so a <true/> anywhere would be wrong.
	if !strings.Contains(s, "<key>RunAtLoad</key>") || strings.Contains(s, "<true/>") {
		t.Errorf("RunAtLoad must be present and false:\n%s", s)
	}
	// The schedule hour/minute must be present.
	if !strings.Contains(s, "<integer>3</integer>") || !strings.Contains(s, "<integer>30</integer>") {
		t.Errorf("schedule 3:30 not encoded:\n%s", s)
	}
}

func TestGeneratePlist_EmptyBinaryFails(t *testing.T) {
	if _, err := GeneratePlist(Config{}); err == nil {
		t.Error("empty binary path must fail")
	}
}

func TestGeneratePlist_ClampsSchedule(t *testing.T) {
	data, err := GeneratePlist(Config{BinaryPath: "/x/suns", Hour: 99, Minute: -5})
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "<integer>23</integer>") || !strings.Contains(s, "<integer>0</integer>") {
		t.Errorf("out-of-range schedule not clamped to 23:00:\n%s", s)
	}
}

func TestPlistPath(t *testing.T) {
	p, err := PlistPath()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "/Library/LaunchAgents/"+Label+".plist") {
		t.Errorf("plist path = %q", p)
	}
}
