package privilege

import (
	"context"
	"errors"
	"testing"
)

type fakePrompter struct {
	calls int
	err   error
}

func (f *fakePrompter) Authenticate(context.Context) error {
	f.calls++
	return f.err
}

func TestAcquire_SkipsPromptWhenTicketValid(t *testing.T) {
	fp := &fakePrompter{}
	c := NewWithOptions(Options{
		Prompter:  fp,
		HasTicket: func(context.Context) bool { return true },
	})
	if err := c.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if fp.calls != 0 {
		t.Fatalf("prompter called %d times despite valid ticket", fp.calls)
	}
}

func TestAcquire_PromptsWhenNoTicket(t *testing.T) {
	fp := &fakePrompter{}
	c := NewWithOptions(Options{
		Prompter:  fp,
		HasTicket: func(context.Context) bool { return false },
	})
	if err := c.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if fp.calls != 1 {
		t.Fatalf("prompter called %d times, want 1", fp.calls)
	}
}

func TestAcquire_CanceledPromptPropagates(t *testing.T) {
	fp := &fakePrompter{err: ErrCanceled}
	c := NewWithOptions(Options{
		Prompter:  fp,
		HasTicket: func(context.Context) bool { return false },
	})
	if err := c.Acquire(context.Background()); !errors.Is(err, ErrCanceled) {
		t.Fatalf("Acquire err = %v, want ErrCanceled", err)
	}
}

// A non-allowlisted privileged action is refused without ever touching sudo
// (§6.4).
func TestRun_RejectsNonAllowlistedAction(t *testing.T) {
	c := NewWithOptions(Options{
		Allow:     map[string]bool{"dscacheutil": true},
		HasTicket: func(context.Context) bool { return true },
	})
	_, err := c.Run(context.Background(), "rm", "-rf", "/")
	if !errors.Is(err, ErrActionNotAllowed) {
		t.Fatalf("Run(rm) err = %v, want ErrActionNotAllowed", err)
	}
}
