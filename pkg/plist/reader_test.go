package plist

import (
	"os"
	"path/filepath"
	"testing"

	hplist "howett.net/plist"
)

// writePlist marshals m in the given format and writes it to a temp file.
func writePlist(t *testing.T, m map[string]any, format int) string {
	t.Helper()
	data, err := hplist.Marshal(m, format)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	p := filepath.Join(t.TempDir(), "Info.plist")
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestBundleIdentifier_XMLAndBinary(t *testing.T) {
	m := map[string]any{
		"CFBundleIdentifier": "com.example.Widget",
		"CFBundleName":       "Widget",
		"CFBundleExecutable": "Widget",
	}
	for _, tc := range []struct {
		name   string
		format int
	}{
		{"xml", hplist.XMLFormat},
		{"binary", hplist.BinaryFormat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := writePlist(t, m, tc.format)
			got, err := BundleIdentifier(path)
			if err != nil {
				t.Fatalf("BundleIdentifier: %v", err)
			}
			if got != "com.example.Widget" {
				t.Errorf("got %q, want com.example.Widget", got)
			}
			info, err := ReadInfo(path)
			if err != nil || info.CFBundleExecutable != "Widget" {
				t.Errorf("ReadInfo = %+v, %v", info, err)
			}
		})
	}
}

func TestBundleIdentifier_MissingKey(t *testing.T) {
	path := writePlist(t, map[string]any{"CFBundleName": "NoID"}, hplist.BinaryFormat)
	if _, err := BundleIdentifier(path); err == nil {
		t.Fatal("want error for plist with no CFBundleIdentifier")
	}
}

func TestDecode_Malformed(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bad.plist")
	if err := os.WriteFile(p, []byte("this is not a plist \x00\x01\x02 at all"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var info Info
	if err := Decode(p, &info); err == nil {
		t.Fatal("want error decoding malformed plist")
	}
}

func TestDecode_FileMissing(t *testing.T) {
	var info Info
	if err := Decode(filepath.Join(t.TempDir(), "nope.plist"), &info); err == nil {
		t.Fatal("want error for missing file")
	}
}
