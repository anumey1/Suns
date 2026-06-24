package tui

import (
	"context"
	"io"
	"os/exec"
)

// powermetricsLauncher starts `sudo -n powermetrics -f plist …` and returns its
// stdout. It relies on a cached sudo ticket (acquired via `e` elevate / the
// chokepoint); with no ticket, powermetrics exits and the supervisor retries
// with backoff, leaving the source Unavailable. Closing the stream kills the
// subprocess so a stall can be recovered (§7.4).
//
// This is the on-device live path: it spawns a privileged subprocess and is not
// exercised by the headless tests, which drive the decoder and the supervised
// stream directly with fixtures.
func powermetricsLauncher(ctx context.Context) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/sudo", "-n",
		"/usr/bin/powermetrics", "-f", "plist", "-i", "1000",
		"--samplers", "cpu_power,gpu_power,thermal")
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &powerStream{r: stdout, cmd: cmd}, nil
}

// nettopLauncher starts an unprivileged long-lived `nettop -P -x -l 0` stream and
// returns its stdout. Each ~1s frame reprints the CSV header; NetSource diffs
// consecutive frames for per-process tx/rx. No elevation is needed, so this can
// start as soon as the dashboard opens. Closing the stream kills the subprocess
// so a stall can be recovered (§3.3, §7.4). This is an on-device live path, not
// exercised by the headless tests (which drive the parser/stream with fixtures).
func nettopLauncher(ctx context.Context) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/nettop", "-P", "-x", "-l", "0", "-s", "1")
	cmd.Env = []string{"LC_ALL=C", "PATH=/usr/bin:/bin:/usr/sbin:/sbin"}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &powerStream{r: stdout, cmd: cmd}, nil
}

// powerStream adapts a supervised subprocess to io.ReadCloser; Close kills it.
type powerStream struct {
	r   io.ReadCloser
	cmd *exec.Cmd
}

func (p *powerStream) Read(b []byte) (int, error) { return p.r.Read(b) }

func (p *powerStream) Close() error {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	_ = p.cmd.Wait()
	return nil
}
