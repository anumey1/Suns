package audit

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Auth-log analyzer (§12.14). It queries the unified logging system — NOT the
// deprecated /var/log/system.log — for sudo activity and surfaces failed
// authentications and privilege-escalation attempts, highlighting rapid-failure
// bursts. The query needs root and is run through the §6 chokepoint by the
// caller (the `log` action is on the privileged allowlist).

// logTimeLayout is the timestamp format `log show --style json` emits.
const logTimeLayout = "2006-01-02 15:04:05.000000-0700"

// burstThreshold/burstWindow define a rapid-failure burst: this many failures
// from one user within this window.
const (
	burstThreshold = 3
	burstWindow    = 60 * time.Second
)

// LogOptions controls the auth-log query.
type LogOptions struct {
	Since string // log "--last" value (e.g. "1d", "6h"); default "1d"
}

func (o LogOptions) since() string {
	if o.Since == "" {
		return "1d"
	}
	return o.Since
}

// AuthEvent is one classified sudo/auth log entry.
type AuthEvent struct {
	Time    time.Time `json:"time"`
	Process string    `json:"process"`
	User    string    `json:"user,omitempty"`
	Outcome string    `json:"outcome"` // success | failure | denied | info
	Message string    `json:"message"`
}

// Burst is a run of rapid authentication failures by one user.
type Burst struct {
	User  string    `json:"user"`
	Count int       `json:"count"`
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// AuthReport is the read-only auth-log analysis.
type AuthReport struct {
	Window   string      `json:"window"`
	Events   []AuthEvent `json:"events"`
	Failures int         `json:"failures"`
	Bursts   []Burst     `json:"bursts,omitempty"`
}

// rawEvent mirrors the subset of `log show --style json` fields we read.
type rawEvent struct {
	Timestamp    string `json:"timestamp"`
	EventMessage string `json:"eventMessage"`
	Process      string `json:"process"`
}

// AuthLog runs the unified-log query through the (privileged) runner and returns
// a classified timeline. Unexpected or non-JSON output degrades to an empty
// report rather than an error, so a partial environment still succeeds.
func AuthLog(ctx context.Context, r Runner, opts LogOptions) (AuthReport, error) {
	rep := AuthReport{Window: opts.since()}
	res, err := r.Run(ctx, "log", "show",
		"--style", "json",
		"--predicate", `process == "sudo"`,
		"--last", opts.since(),
		"--info",
	)
	if err != nil {
		return rep, err
	}

	var raw []rawEvent
	if json.Unmarshal(res.Stdout, &raw) != nil {
		return rep, nil // not JSON / empty → no events
	}

	for _, e := range raw {
		ev := AuthEvent{
			Process: e.Process,
			Message: strings.TrimSpace(e.EventMessage),
			Outcome: classifyAuth(e.EventMessage),
			User:    sudoUser(e.EventMessage),
		}
		if t, perr := time.Parse(logTimeLayout, e.Timestamp); perr == nil {
			ev.Time = t
		}
		if ev.Outcome == "failure" || ev.Outcome == "denied" {
			rep.Failures++
		}
		rep.Events = append(rep.Events, ev)
	}

	sort.SliceStable(rep.Events, func(i, j int) bool { return rep.Events[i].Time.Before(rep.Events[j].Time) })
	rep.Bursts = detectBursts(rep.Events)
	return rep, nil
}

// classifyAuth labels a sudo log message by outcome.
func classifyAuth(msg string) string {
	s := strings.ToLower(msg)
	switch {
	case strings.Contains(s, "incorrect password") || strings.Contains(s, "authentication failure") || strings.Contains(s, "auth failure"):
		return "failure"
	case strings.Contains(s, "not in the sudoers") || strings.Contains(s, "not allowed") || strings.Contains(s, "command not allowed"):
		return "denied"
	case strings.Contains(s, "command=") || strings.Contains(s, "session opened") || strings.Contains(s, "tty="):
		return "success"
	default:
		return "info"
	}
}

// sudoUser extracts the actor from a sudo message of the form
// "<user> : <details>".
func sudoUser(msg string) string {
	if i := strings.Index(msg, " : "); i > 0 {
		return strings.TrimSpace(msg[:i])
	}
	return ""
}

// detectBursts finds, per user, the first window of >= burstThreshold failures
// within burstWindow.
func detectBursts(events []AuthEvent) []Burst {
	byUser := map[string][]time.Time{}
	for _, e := range events {
		if (e.Outcome == "failure" || e.Outcome == "denied") && !e.Time.IsZero() {
			byUser[e.User] = append(byUser[e.User], e.Time)
		}
	}
	var bursts []Burst
	for user, times := range byUser {
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		lo := 0
		for hi := range times {
			for times[hi].Sub(times[lo]) > burstWindow {
				lo++
			}
			if hi-lo+1 >= burstThreshold {
				bursts = append(bursts, Burst{User: user, Count: hi - lo + 1, Start: times[lo], End: times[hi]})
				break
			}
		}
	}
	sort.Slice(bursts, func(i, j int) bool { return bursts[i].User < bursts[j].User })
	return bursts
}
