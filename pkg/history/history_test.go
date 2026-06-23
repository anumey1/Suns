package history_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anumey1/Suns/pkg/history"
	"github.com/anumey1/Suns/pkg/operation"
	"github.com/anumey1/Suns/pkg/safety/identity"
)

func TestAppendReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	log, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	want := []operation.HistoryEntry{
		{
			TS: time.Now().UTC().Truncate(time.Second), Plan: "01plan", Op: operation.KindFileDelete,
			Reversible: operation.Reversible, Path: "/Users/x/Library/Caches/foo", Size: 12345,
			Fate: "trashed", TrashPath: "/Users/x/.Trash/foo", OrigPath: "/Users/x/Library/Caches/foo",
			Identity: &identity.FileIdent{Tier: identity.TierLarge, Size: 12345, Inode: 98765},
			Status:   "ok", Cmd: "clean",
		},
		{
			TS: time.Now().UTC().Truncate(time.Second), Plan: "01plan", Op: operation.KindProcessKill,
			Reversible: operation.Irreversible, PID: 941, Name: "Xcode", Signal: "SIGKILL", Cmd: "ps",
		},
	}
	for _, e := range want {
		if err := log.Append(e); err != nil {
			t.Fatal(err)
		}
	}

	got, err := history.ReadAll(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d entries, want %d", len(got), len(want))
	}
	if got[0].Reversible != operation.Reversible || got[1].Reversible != operation.Irreversible {
		t.Fatalf("reversibility did not round-trip: %v / %v", got[0].Reversible, got[1].Reversible)
	}
	if got[0].Path != want[0].Path || got[1].Name != "Xcode" {
		t.Fatalf("fields did not round-trip: %+v / %+v", got[0], got[1])
	}
}

func TestFileModeIs0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	log, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := log.Append(operation.HistoryEntry{Op: operation.KindDNSFlush}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("history file mode = %o, want 0600", perm)
	}
}

// Export redaction must transform paths/names while leaving the canonical file
// full-fidelity and untouched (§13.3).
func TestExportRedacted_TransformsCopyNotCanonical(t *testing.T) {
	dir := t.TempDir()
	canonical := filepath.Join(dir, "history.jsonl")
	log, err := history.Open(canonical)
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	secret := filepath.Join(home, "dev", "secret-project", "cache")
	if err := log.Append(operation.HistoryEntry{
		Op: operation.KindFileDelete, Reversible: operation.Reversible,
		Path: secret, OrigPath: secret, Name: "MySecretApp",
	}); err != nil {
		t.Fatal(err)
	}

	exported := filepath.Join(dir, "redacted.jsonl")
	if err := history.ExportRedacted(canonical, exported); err != nil {
		t.Fatal(err)
	}

	// Canonical unchanged: still full fidelity.
	canon, _ := history.ReadAll(canonical)
	if canon[0].Path != secret {
		t.Fatalf("canonical was modified: %q", canon[0].Path)
	}
	// Export redacted: no secret project name, name elided, home relativized.
	red, _ := history.ReadAll(exported)
	if strings.Contains(red[0].Path, "secret-project") {
		t.Fatalf("redacted export leaked path: %q", red[0].Path)
	}
	if !strings.HasPrefix(red[0].Path, "~/") {
		t.Fatalf("redacted home path not relativized: %q", red[0].Path)
	}
	if red[0].Name == "MySecretApp" {
		t.Fatalf("redacted export leaked process name: %q", red[0].Name)
	}
}
