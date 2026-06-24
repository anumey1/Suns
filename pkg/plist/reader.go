package plist

import (
	"fmt"
	"os"

	hplist "howett.net/plist"
)

// Decode reads a property-list file — binary OR XML — into v, which must be a
// howett.net/plist-compatible target (a tagged struct or a map). This is the
// single binary-safe entry point Suns uses instead of encoding/xml, because many
// modern Info.plist and launchd files are the binary format that encoding/xml
// cannot parse (§2.4). Callers define their own field structs.
func Decode(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return DecodeBytes(data, v)
}

// DecodeBytes decodes in-memory plist bytes (binary or XML) into v.
func DecodeBytes(data []byte, v any) error {
	if _, err := hplist.Unmarshal(data, v); err != nil {
		return fmt.Errorf("plist: decode: %w", err)
	}
	return nil
}

// Info holds the Info.plist fields Suns reads when tracing an application
// (§12.15-uninstaller). Unknown keys are ignored.
type Info struct {
	CFBundleIdentifier string `plist:"CFBundleIdentifier"`
	CFBundleName       string `plist:"CFBundleName"`
	CFBundleExecutable string `plist:"CFBundleExecutable"`
}

// ReadInfo decodes an app's Info.plist (binary or XML).
func ReadInfo(path string) (Info, error) {
	var info Info
	if err := Decode(path, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}

// BundleIdentifier returns the CFBundleIdentifier from a (binary or XML)
// Info.plist, failing loudly if the key is absent — the bundle ID is the anchor
// for support-file tracing, so an empty one must never silently match nothing.
func BundleIdentifier(path string) (string, error) {
	info, err := ReadInfo(path)
	if err != nil {
		return "", err
	}
	if info.CFBundleIdentifier == "" {
		return "", fmt.Errorf("plist: %s has no CFBundleIdentifier", path)
	}
	return info.CFBundleIdentifier, nil
}
