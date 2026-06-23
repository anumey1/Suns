package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSafeCacheManifest_Embedded(t *testing.T) {
	m, err := LoadSafeCacheManifest()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Targets) == 0 {
		t.Fatal("embedded manifest has no targets")
	}
	// At least one expensive opt-in target (iOS DeviceSupport) should exist.
	var sawOptIn bool
	for _, tg := range m.Targets {
		if tg.OptIn {
			sawOptIn = true
		}
	}
	if !sawOptIn {
		t.Error("expected at least one opt-in target in the manifest")
	}
}

func TestDiscover_FindsExistingTargetsAndSkipsMissing(t *testing.T) {
	root := t.TempDir()

	present := filepath.Join(root, "cache")
	if err := os.MkdirAll(present, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(present, "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(root, "does-not-exist")

	targets := []Target{
		{ID: "present", Path: present, Category: "dev-cache", RebuildCost: CostCheap},
		{ID: "missing", Path: missing, Category: "dev-cache", RebuildCost: CostCheap},
	}
	res, err := Discover(context.Background(), targets, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Ops) != 1 {
		t.Fatalf("got %d ops, want 1", len(res.Ops))
	}
	if len(res.Skipped) != 1 {
		t.Fatalf("got %d skipped, want 1 (the missing target)", len(res.Skipped))
	}
	if got := res.Found[0].Op.Size; got < 2048 {
		t.Errorf("discovered size = %d, want >= 2048", got)
	}
}

func TestDiscover_OptInExcludedByDefault(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "expensive")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	targets := []Target{{ID: "x", Path: dir, OptIn: true, RebuildCost: CostExpensive}}

	if res, _ := Discover(context.Background(), targets, Options{IncludeOptIn: false}); len(res.Ops) != 0 {
		t.Fatalf("opt-in target included without IncludeOptIn: %d ops", len(res.Ops))
	}
	if res, _ := Discover(context.Background(), targets, Options{IncludeOptIn: true}); len(res.Ops) != 1 {
		t.Fatalf("opt-in target not included with IncludeOptIn: %d ops", len(res.Ops))
	}
}
