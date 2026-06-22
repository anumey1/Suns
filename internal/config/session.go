// Package config loads configuration once at startup (via Viper) into a
// RWMutex-guarded SessionState (§4.9).
//
// Viper is NOT safe for concurrent read/write, so it is read exactly once into
// this struct; engines never call viper.Get* at runtime — they read via
// accessors that take the read lock. TUI toggles take the write lock so no
// worker observes a half-written value. Viper is written back to disk only on
// an explicit "Save to config" action.
//
// State precedence for one-shot CLI invocations: inline flags > saved config
// file > hardcoded safe defaults (confirm_mode=off, deletion_mode=trash).
package config

import "sync"

// Deletion modes (the Jarjar axis machine values).
const (
	DeletionTrash      = "trash"
	DeletionObliterate = "obliterate"
)

// SessionState is the in-memory, mutex-guarded configuration shared with the
// engines. The memorable UI names (Deathstar, Jarjar) map to these neutral
// machine keys (§13.3).
type SessionState struct {
	mu sync.RWMutex

	confirmMode  bool   // Deathstar (confirm_mode): false → preview + gate shown
	deletionMode string // Jarjar (deletion_mode): "trash" | "obliterate"
}

// NewDefault returns the hardcoded safe defaults: gate shown, move to Trash.
func NewDefault() *SessionState {
	return &SessionState{confirmMode: false, deletionMode: DeletionTrash}
}

// ConfirmMode reports the Deathstar axis. False means the confirmation gate is
// shown before acting (the safe default).
func (s *SessionState) ConfirmMode() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.confirmMode
}

// DeletionMode reports the Jarjar axis (FileDelete only).
func (s *SessionState) DeletionMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deletionMode
}

// SetConfirmMode toggles Deathstar under the write lock (ctrl+d in the TUI).
func (s *SessionState) SetConfirmMode(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.confirmMode = on
}

// SetDeletionMode toggles Jarjar under the write lock (ctrl+j in the TUI).
func (s *SessionState) SetDeletionMode(mode string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletionMode = mode
}
