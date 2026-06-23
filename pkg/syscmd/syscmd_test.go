package syscmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRun_RejectsNonAllowlisted(t *testing.T) {
	r := New()
	_, err := r.Run(context.Background(), "rm", "-rf", "/")
	if !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("Run(rm) err = %v, want ErrNotAllowed", err)
	}
}

func TestRun_NoShellInjection(t *testing.T) {
	// /bin/echo prints its arguments literally. If the args were ever passed
	// through a shell, the metacharacters would be expanded or would spawn a
	// subshell. We assert the argument comes back verbatim — proving no shell.
	r := NewWithAllowlist(map[string]string{"echo": "/bin/echo"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	payload := "$(touch /tmp/pwned); `id`; a&&b; *"
	res, err := r.Run(ctx, "echo", payload)
	if err != nil {
		t.Fatalf("Run(echo): %v", err)
	}
	got := strings.TrimRight(string(res.Stdout), "\n")
	if got != payload {
		t.Fatalf("argument was not passed verbatim:\n got %q\nwant %q", got, payload)
	}
}

func TestRun_BoundsOutput(t *testing.T) {
	r := NewWithAllowlist(map[string]string{"yes": "/usr/bin/yes"})
	r.maxOutput = 1024
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// `yes` streams forever; the context deadline stops it and the bounded
	// buffer caps memory. We only assert the capture stayed bounded.
	res, _ := r.Run(ctx, "yes")
	if len(res.Stdout) > r.maxOutput {
		t.Fatalf("captured %d bytes, want <= %d", len(res.Stdout), r.maxOutput)
	}
	if !res.Truncated {
		t.Fatalf("expected Truncated = true for an unbounded producer")
	}
}

func TestRun_SetsDeterministicLocale(t *testing.T) {
	r := NewWithAllowlist(map[string]string{"env": "/usr/bin/env"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := r.Run(ctx, "env")
	if err != nil {
		t.Fatalf("Run(env): %v", err)
	}
	if !strings.Contains(string(res.Stdout), "LC_ALL=C") {
		t.Fatalf("environment not scrubbed to LC_ALL=C; got:\n%s", res.Stdout)
	}
}
