package docker

import (
	"context"
	"fmt"
	"testing"

	"github.com/anumey1/Suns/pkg/syscmd"
)

// fakeDocker dispatches by the docker subcommand.
type fakeDocker struct {
	version  string // empty → daemon down
	df       string
	prune    string
	pruneErr int // exit code for prune
	lastArgs []string
}

func (f *fakeDocker) Run(_ context.Context, _ string, args ...string) (syscmd.Result, error) {
	f.lastArgs = args
	switch {
	case has(args, "version"):
		return syscmd.Result{Stdout: []byte(f.version), ExitCode: 0}, nil
	case has(args, "df"):
		return syscmd.Result{Stdout: []byte(f.df), ExitCode: 0}, nil
	case has(args, "prune"):
		return syscmd.Result{Stdout: []byte(f.prune), ExitCode: f.pruneErr}, nil
	}
	return syscmd.Result{}, nil
}

func has(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// dfTable builds an aligned `docker system df` table so the RECLAIMABLE column
// starts at a fixed byte offset in every row.
func dfTable() string {
	row := func(a, b, c, d, e string) string {
		return fmt.Sprintf("%-14s%-7s%-8s%-8s%s\n", a, b, c, d, e)
	}
	return row("TYPE", "TOTAL", "ACTIVE", "SIZE", "RECLAIMABLE") +
		row("Images", "5", "2", "1.2GB", "800MB (66%)") +
		row("Containers", "3", "1", "50MB", "30MB") +
		row("Local Volumes", "2", "1", "500MB", "200MB (40%)") +
		row("Build Cache", "10", "0", "300MB", "300MB")
}

func TestParseHumanSize(t *testing.T) {
	cases := map[string]int64{
		"1.2GB":       1_200_000_000,
		"800MB (66%)": 800_000_000,
		"512kB":       512_000,
		"0B":          0,
		"":            0,
		"2TB":         2_000_000_000_000,
	}
	for in, want := range cases {
		if got := parseHumanSize(in); got != want {
			t.Errorf("parseHumanSize(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseReclaimable(t *testing.T) {
	// 800 + 30 + 200 + 300 MB = 1.33 GB.
	got := parseReclaimable([]byte(dfTable()))
	want := int64(1_330_000_000)
	if got != want {
		t.Errorf("parseReclaimable = %d, want %d", got, want)
	}
}

func TestParseReclaimedLine(t *testing.T) {
	out := "deleted: sha256:abc\nTotal reclaimed space: 1.5GB\n"
	if got := parseReclaimedLine([]byte(out)); got != 1_500_000_000 {
		t.Errorf("parseReclaimedLine = %d, want 1.5e9", got)
	}
}

func TestDetectWith_DaemonDown(t *testing.T) {
	st := detectWith(context.Background(), "/x/docker", &fakeDocker{version: ""})
	if !st.Installed || st.Running {
		t.Errorf("daemon down: Installed=%v Running=%v, want true/false", st.Installed, st.Running)
	}
	if st.Detail == "" {
		t.Error("expected a human Detail when the daemon is down")
	}
}

func TestDetectWith_Running(t *testing.T) {
	st := detectWith(context.Background(), "/x/docker", &fakeDocker{version: "24.0.7\n", df: dfTable()})
	if !st.Running {
		t.Fatal("expected Running when version resolves")
	}
	if st.Reclaimable != 1_330_000_000 {
		t.Errorf("reclaimable = %d, want 1.33e9", st.Reclaimable)
	}
}

func TestPruner_SuccessAndVolumesFlag(t *testing.T) {
	fd := &fakeDocker{prune: "Total reclaimed space: 2GB\n"}
	p := &Pruner{r: fd}

	stats, err := p.Prune(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if stats.ReclaimedBytes != 2_000_000_000 {
		t.Errorf("reclaimed = %d, want 2e9", stats.ReclaimedBytes)
	}
	if !has(fd.lastArgs, "--volumes") {
		t.Errorf("prune argv must include --volumes when requested: %v", fd.lastArgs)
	}
}

func TestPruner_FailureIsError(t *testing.T) {
	p := &Pruner{r: &fakeDocker{pruneErr: 1}}
	if _, err := p.Prune(context.Background(), false); err == nil {
		t.Error("non-zero prune exit must be an error")
	}
}
