// Package profilefile provides the local file-backed implementation of
// profile.Store: a JSON file at ~/.athena/profile.json (path chosen by the
// caller, not this package — see internal/infrastructure/athenahome).
package profilefile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/santaniello/athena/internal/domain/profile"
)

// Store is a file-backed implementation of profile.Store.
type Store struct {
	path string
}

// NewStore creates a Store that reads and writes the profile at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Save writes the profile to disk with owner-only permissions, creating the
// parent directory if it does not exist.
func (s *Store) Save(p profile.UserProfile) error {
	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("profilefile: encoding profile: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("profilefile: creating profile directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("profilefile: writing profile file: %w", err)
	}
	return nil
}

// Load reads the profile from disk.
func (s *Store) Load() (profile.UserProfile, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return profile.UserProfile{}, fmt.Errorf("profilefile: reading profile file: %w", err)
	}
	var p profile.UserProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return profile.UserProfile{}, fmt.Errorf("profilefile: decoding profile file: %w", err)
	}
	return p, nil
}
