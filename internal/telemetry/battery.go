package telemetry

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/anumey1/Suns/pkg/syscmd"
)

var (
	batteryPctRe  = regexp.MustCompile(`(\d+)%`)
	batteryTimeRe = regexp.MustCompile(`(\d+:\d{2})\s+remaining`)
)

// ParseBattery parses `pmset -g batt` output. It is read-only and needs no
// elevation (§11: the dashboard works out of the box). On a desktop with no
// battery, Present is false.
func ParseBattery(out string) Battery {
	b := Battery{}
	b.OnAC = strings.Contains(out, "'AC Power'")

	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "InternalBattery") {
			continue
		}
		b.Present = true
		if m := batteryPctRe.FindStringSubmatch(line); m != nil {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				b.Percent = v
			}
		}
		// State is the word after the first "; " following the percentage.
		if parts := strings.Split(line, ";"); len(parts) >= 2 {
			b.State = strings.TrimSpace(parts[1])
		}
		if m := batteryTimeRe.FindStringSubmatch(line); m != nil && m[1] != "0:00" {
			b.TimeRemaining = m[1]
		}
		break
	}
	return b
}

// fetchBattery runs `pmset -g batt` through the hardened executor.
func fetchBattery(ctx context.Context) (Battery, bool) {
	r := syscmd.New()
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res, err := r.Run(cctx, "pmset", "-g", "batt")
	if err != nil {
		return Battery{}, false
	}
	b := ParseBattery(string(res.Stdout))
	return b, b.Present
}
