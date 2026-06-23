// Package history implements the operation-history log (§4.10, §13.3).
//
// The canonical record lives at ~/Library/Application Support/Suns/history.jsonl
// (mode 0600, append-only, crash-safe JSON Lines, one self-contained object per
// line). Each operation writes a typed, per-kind record carrying the Plan.ID,
// which makes the reversibility claim honest. It is an operational record, NOT a
// tamper-evident forensic trail — there is no cryptographic chaining or signing
// in v1, and it is described as such.
//
// The canonical log is FULL FIDELITY (real absolute paths and identities)
// because restore and debugging depend on it; redaction is an EXPORT-ONLY
// transform (ExportRedacted), never applied in place.
package history

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/anumey1/Suns/pkg/operation"
)

// DefaultPath returns the canonical history-log path.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Suns", "history.jsonl"), nil
}

// Log is an append-only handle to the canonical history file.
type Log struct {
	mu   sync.Mutex
	path string
}

// Open returns a Log for the given path, creating the parent directory (0700)
// and the file (0600) if needed.
func Open(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	return &Log{path: path}, nil
}

// Append writes one entry as a single JSON line. Each call opens, appends, and
// closes so a crash cannot leave the file locked; O_APPEND keeps concurrent
// writers from interleaving partial lines.
func (l *Log) Append(e operation.HistoryEntry) error {
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// ReadAll parses every record in the log file at path.
func ReadAll(path string) ([]operation.HistoryEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []operation.HistoryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e operation.HistoryEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("history: malformed record: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

// Redact returns an export-safe copy of e: paths are home-relativized and
// hashed, and the process name is elided. The canonical record is never
// modified (§13.3). Identities (size/mtime/inode/hash) are retained since they
// do not reveal user or project context.
func Redact(e operation.HistoryEntry) operation.HistoryEntry {
	e.Path = redactPath(e.Path)
	e.OrigPath = redactPath(e.OrigPath)
	e.TrashPath = redactPath(e.TrashPath)
	e.Exec = redactPath(e.Exec)
	if e.Name != "" {
		e.Name = "[redacted]"
	}
	return e
}

// ExportRedacted reads the canonical log at src and writes a redacted copy to
// dst (0600), leaving src untouched.
func ExportRedacted(src, dst string) error {
	entries, err := ReadAll(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, e := range entries {
		line, err := json.Marshal(Redact(e))
		if err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return w.Flush()
}

// redactPath maps a path to a privacy-preserving form: paths under the home
// directory become "~/<hash>", others become "<hash>". The hash is a stable
// 12-hex-char SHA-256 prefix, so equal paths redact equally (useful for
// debugging patterns) without revealing usernames or project names.
func redactPath(p string) string {
	if p == "" {
		return ""
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			return "~"
		}
		if rest, ok := strings.CutPrefix(p, home+string(os.PathSeparator)); ok {
			return "~/" + shortHash(rest)
		}
	}
	return shortHash(p)
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:12]
}
