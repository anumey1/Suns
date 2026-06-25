package docker

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/syscmd"
)

// runner is the minimal executor the engine needs; production wraps the
// discovered binary in a single-entry hardened allowlist, tests inject a fake.
type runner interface {
	Run(ctx context.Context, name string, args ...string) (syscmd.Result, error)
}

// Status is the detected container-engine picture.
type Status struct {
	Installed   bool   `json:"installed"`          // a docker CLI binary was located
	Running     bool   `json:"running"`            // the daemon answered
	Endpoint    string `json:"endpoint,omitempty"` // the socket or context found
	Reclaimable int64  `json:"reclaimable_bytes"`  // estimate from `docker system df`
	Detail      string `json:"detail,omitempty"`   // human explanation when not usable
	Binary      string `json:"-"`                  // discovered CLI path (for the pruner)
}

// candidateBinaries are the absolute locations the supported runtimes install the
// docker CLI. Probed in order; the production allowlist never contains a variable
// path, so the chosen path is wrapped in a one-entry allowlist at use time.
func candidateBinaries() []string {
	home, _ := os.UserHomeDir()
	return []string{
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
		filepath.Join(home, ".docker/bin/docker"),
		filepath.Join(home, ".rd/bin/docker"), // Rancher Desktop
		"/Applications/Docker.app/Contents/Resources/bin/docker",
	}
}

// candidateSockets are the well-known daemon socket locations across Docker
// Desktop, Colima, and OrbStack; the first that exists labels the endpoint.
func candidateSockets() []string {
	home, _ := os.UserHomeDir()
	return []string{
		filepath.Join(home, ".docker/run/docker.sock"),
		filepath.Join(home, ".colima/default/docker.sock"),
		filepath.Join(home, ".orbstack/run/docker.sock"),
		"/var/run/docker.sock",
	}
}

// Detect locates the docker CLI, confirms the daemon, and estimates reclaimable
// space. It never errors: an absent or stopped engine yields a Status with a
// human Detail and Installed/Running false, so the CLI can print a clean no-op.
func Detect(ctx context.Context) Status {
	bin := locate(candidateBinaries())
	if bin == "" {
		return Status{Detail: "no docker CLI found (Docker Desktop, Colima, or OrbStack not installed)"}
	}
	r := syscmd.NewWithAllowlist(map[string]string{"docker": bin})
	return detectWith(ctx, bin, r)
}

// detectWith is the testable core: given a located binary and a runner, it probes
// the daemon and reclaimable space.
func detectWith(ctx context.Context, bin string, r runner) Status {
	st := Status{Installed: true, Binary: bin, Endpoint: firstExisting(candidateSockets())}

	// `docker version --format {{.Server.Version}}` succeeds only when the daemon
	// is reachable; an error or empty server version means it is not running.
	res, err := r.Run(ctx, "docker", "version", "--format", "{{.Server.Version}}")
	if err != nil || res.ExitCode != 0 || len(strings.TrimSpace(string(res.Stdout))) == 0 {
		st.Detail = "docker CLI present but the daemon is not running"
		return st
	}
	st.Running = true
	st.Reclaimable = reclaimable(ctx, r)
	return st
}

// reclaimable sums the RECLAIMABLE column of `docker system df`, degrading to 0 on
// any parse trouble (the estimate is advisory; the gate shows it, the prune
// reports the real figure).
func reclaimable(ctx context.Context, r runner) int64 {
	res, err := r.Run(ctx, "docker", "system", "df")
	if err != nil || res.ExitCode != 0 {
		return 0
	}
	return parseReclaimable(res.Stdout)
}

// parseReclaimable reads the RECLAIMABLE column from `docker system df` table
// output and sums it. The column header fixes the byte offset; each cell looks
// like "1.2GB" or "800MB (66%)".
func parseReclaimable(out []byte) int64 {
	lines := strings.Split(string(out), "\n")
	if len(lines) == 0 {
		return 0
	}
	col := strings.Index(lines[0], "RECLAIMABLE")
	if col < 0 {
		return 0
	}
	var total int64
	for _, ln := range lines[1:] {
		if len(ln) <= col {
			continue
		}
		field := strings.Fields(ln[col:])
		if len(field) == 0 {
			continue
		}
		total += parseHumanSize(field[0])
	}
	return total
}

// parseReclaimedLine extracts the byte count from a prune's
// "Total reclaimed space: 1.2GB" trailer.
func parseReclaimedLine(out []byte) int64 {
	for _, ln := range strings.Split(string(out), "\n") {
		if i := strings.Index(ln, "Total reclaimed space:"); i >= 0 {
			return parseHumanSize(strings.TrimSpace(ln[i+len("Total reclaimed space:"):]))
		}
	}
	return 0
}

// parseHumanSize parses docker's human size strings (decimal units: kB, MB, GB,
// TB) into bytes, tolerating a trailing "(NN%)" and surrounding spaces.
func parseHumanSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Drop a trailing percentage in parentheses.
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	num := strings.TrimRight(s, "BbKkMmGgTtPi")
	unit := strings.ToUpper(s[len(num):])
	f, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
	if err != nil {
		return 0
	}
	mult := float64(1)
	switch {
	case strings.HasPrefix(unit, "T"):
		mult = 1e12
	case strings.HasPrefix(unit, "G"):
		mult = 1e9
	case strings.HasPrefix(unit, "M"):
		mult = 1e6
	case strings.HasPrefix(unit, "K"):
		mult = 1e3
	}
	return int64(f * mult)
}

// Pruner is the injected ContainerPruner backed by the discovered docker CLI.
type Pruner struct {
	r runner
}

// NewPruner wraps the discovered binary in a one-entry hardened allowlist.
func NewPruner(bin string) *Pruner {
	return &Pruner{r: syscmd.NewWithAllowlist(map[string]string{"docker": bin})}
}

// Prune runs `docker system prune -a -f [--volumes]` and returns the reclaimed
// bytes parsed from its summary. -f suppresses docker's own prompt; the Suns gate
// already obtained confirmation.
func (p *Pruner) Prune(ctx context.Context, includeVolumes bool) (operation.PruneStats, error) {
	args := []string{"system", "prune", "-a", "-f"}
	if includeVolumes {
		args = append(args, "--volumes")
	}
	res, err := p.r.Run(ctx, "docker", args...)
	if err != nil {
		return operation.PruneStats{}, err
	}
	if res.ExitCode != 0 {
		return operation.PruneStats{}, &PruneError{Stderr: strings.TrimSpace(string(res.Stderr))}
	}
	return operation.PruneStats{ReclaimedBytes: parseReclaimedLine(res.Stdout)}, nil
}

// PruneError carries a non-zero docker exit's stderr.
type PruneError struct{ Stderr string }

func (e *PruneError) Error() string {
	if e.Stderr == "" {
		return "docker system prune failed"
	}
	return "docker system prune failed: " + e.Stderr
}

// locate returns the first existing executable path, or "".
func locate(paths []string) string {
	for _, p := range paths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0 {
			return p
		}
	}
	return ""
}

// firstExisting returns the first path that exists, or "".
func firstExisting(paths []string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
