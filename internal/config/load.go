package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// FilePath returns the optional config-file location.
func FilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "Application Support", "Suns", "config.yaml"), nil
}

// Overrides carry per-invocation flag values. Only non-nil fields override the
// config file and defaults.
type Overrides struct {
	ConfirmMode  *bool
	DeletionMode *string
}

// Load builds the SessionState with the documented precedence: inline flags >
// saved config file > hardcoded safe defaults (§4.9). Viper reads the file
// exactly once here; engines never touch Viper at runtime — they read the
// returned SessionState through its locked accessors.
func Load(ov Overrides) (*SessionState, error) {
	s := NewDefault()

	if path, err := FilePath(); err == nil {
		if _, statErr := os.Stat(path); statErr == nil {
			v := viper.New()
			v.SetConfigType("yaml")
			v.SetConfigFile(path)
			if err := v.ReadInConfig(); err != nil {
				return nil, fmt.Errorf("config: reading %s: %w", path, err)
			}
			if v.IsSet("confirm_mode") {
				s.confirmMode = v.GetBool("confirm_mode")
			}
			if v.IsSet("deletion_mode") {
				if err := validateDeletionMode(v.GetString("deletion_mode")); err != nil {
					return nil, fmt.Errorf("config: %w", err)
				}
				s.deletionMode = v.GetString("deletion_mode")
			}
		}
	}

	if ov.ConfirmMode != nil {
		s.confirmMode = *ov.ConfirmMode
	}
	if ov.DeletionMode != nil {
		if err := validateDeletionMode(*ov.DeletionMode); err != nil {
			return nil, err
		}
		s.deletionMode = *ov.DeletionMode
	}
	return s, nil
}

func validateDeletionMode(m string) error {
	switch m {
	case DeletionTrash, DeletionObliterate:
		return nil
	default:
		return fmt.Errorf("invalid deletion_mode %q (want %q or %q)", m, DeletionTrash, DeletionObliterate)
	}
}

// ErrInvalidDeletionMode is returned for an unrecognized deletion mode.
var ErrInvalidDeletionMode = errors.New("invalid deletion mode")
