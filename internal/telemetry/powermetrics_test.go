package telemetry

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

const pmFixture = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>gpu</key><dict><key>idle_ratio</key><real>0.25</real></dict>
<key>processor</key><dict><key>combined_power</key><real>18000</real><key>package_watts</key><real>12.5</real></dict>
<key>thermal_pressure</key><string>Nominal</string>
</dict></plist>`

const pmThrottled = `<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>gpu</key><dict><key>idle_ratio</key><real>0.0</real></dict>
<key>processor</key><dict><key>combined_power</key><real>30000</real></dict>
<key>thermal_pressure</key><string>Heavy</string>
</dict></plist>`

func TestDecodePowerMetrics(t *testing.T) {
	p, err := DecodePowerMetrics([]byte(pmFixture))
	if err != nil {
		t.Fatal(err)
	}
	if p.GPUPercent != 75 {
		t.Errorf("GPUPercent = %v, want 75", p.GPUPercent)
	}
	if p.SystemWatts != 18 {
		t.Errorf("SystemWatts = %v, want 18", p.SystemWatts)
	}
	if p.PackageWatts != 12.5 {
		t.Errorf("PackageWatts = %v, want 12.5", p.PackageWatts)
	}
	if p.Throttled {
		t.Error("Nominal pressure should not be throttled")
	}

	hot, err := DecodePowerMetrics([]byte(pmThrottled))
	if err != nil {
		t.Fatal(err)
	}
	if !hot.Throttled {
		t.Error("Heavy thermal pressure should be throttled")
	}
	if hot.GPUPercent != 100 {
		t.Errorf("GPUPercent = %v, want 100", hot.GPUPercent)
	}
}

// The supervised source decodes a concatenated stream and goes live, without
// any privileged subprocess (the fixture stands in for powermetrics output).
func TestPowerSource_RunGoesLive(t *testing.T) {
	ps := NewPowerSource()
	r := strings.NewReader(pmFixture + pmFixture) // two docs, then EOF
	if err := ps.run(context.Background(), r, time.Second); err != nil {
		t.Fatalf("run: %v", err)
	}
	p, st := ps.Latest()
	if st.Health != HealthLive {
		t.Fatalf("health = %v, want live", st.Health)
	}
	if p.GPUPercent != 75 {
		t.Fatalf("GPUPercent = %v, want 75", p.GPUPercent)
	}
}

// Supervise relaunches via the injected launcher and stops cleanly on context
// cancel.
func TestPowerSource_SuperviseStartsAndStops(t *testing.T) {
	ps := NewPowerSource()
	ctx, cancel := context.WithCancel(context.Background())

	// A launcher that hands back a fresh fixture stream each time it is called,
	// so the supervisor keeps relaunching.
	launch := func(context.Context) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(pmFixture)), nil
	}

	done := make(chan struct{})
	go func() {
		ps.Supervise(ctx, launch, 200*time.Millisecond)
		close(done)
	}()

	// Wait until at least one sample has been decoded.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if p, _ := ps.Latest(); p.GPUPercent == 75 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("source never went live")
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Supervise did not stop on cancel")
	}
}
