package telemetry

import "testing"

func TestParseBattery(t *testing.T) {
	cases := []struct {
		name    string
		out     string
		present bool
		pct     float64
		state   string
		remain  string
		onAC    bool
	}{
		{
			name:    "discharging on battery",
			out:     "Now drawing from 'Battery Power'\n -InternalBattery-0 (id=4456543)\t87%; discharging; 3:42 remaining present: true",
			present: true, pct: 87, state: "discharging", remain: "3:42", onAC: false,
		},
		{
			name:    "charging on AC",
			out:     "Now drawing from 'AC Power'\n -InternalBattery-0 (id=4456543)\t64%; charging; 1:10 remaining present: true",
			present: true, pct: 64, state: "charging", remain: "1:10", onAC: true,
		},
		{
			name:    "charged hides 0:00",
			out:     "Now drawing from 'AC Power'\n -InternalBattery-0 (id=1)\t100%; charged; 0:00 remaining present: true",
			present: true, pct: 100, state: "charged", remain: "", onAC: true,
		},
		{
			name:    "desktop no battery",
			out:     "Now drawing from 'AC Power'",
			present: false, onAC: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := ParseBattery(c.out)
			if b.Present != c.present {
				t.Fatalf("Present = %v, want %v", b.Present, c.present)
			}
			if !c.present {
				return
			}
			if b.Percent != c.pct {
				t.Errorf("Percent = %v, want %v", b.Percent, c.pct)
			}
			if b.State != c.state {
				t.Errorf("State = %q, want %q", b.State, c.state)
			}
			if b.TimeRemaining != c.remain {
				t.Errorf("TimeRemaining = %q, want %q", b.TimeRemaining, c.remain)
			}
			if b.OnAC != c.onAC {
				t.Errorf("OnAC = %v, want %v", b.OnAC, c.onAC)
			}
		})
	}
}
