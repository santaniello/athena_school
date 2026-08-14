// Package session provides the local file-backed implementation of
// auth.SessionStore: a JSON marker at ~/.athena/session.json (path chosen
// by the caller, not this package — see internal/infrastructure/athenahome).
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/santaniello/athena/internal/domain/auth"
)

// Store is a file-backed implementation of auth.SessionStore.
type Store struct {
	path string
}

// NewStore creates a Store that reads and writes the session marker at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

type sessionFile struct {
	AccountID string    `json:"account_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Save writes the session marker to disk with owner-only permissions,
// creating the parent directory if it does not exist.
func (s *Store) Save(session auth.Session) error {
	data, err := json.Marshal(sessionFile{AccountID: session.AccountID, CreatedAt: session.CreatedAt})
	if err != nil {
		return fmt.Errorf("session: encoding session: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("session: creating session directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("session: writing session file: %w", err)
	}
	return nil
}

// Load reads the session marker from disk.
func (s *Store) Load() (auth.Session, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return auth.Session{}, fmt.Errorf("session: reading session file: %w", err)
	}
	var file sessionFile
	if err := json.Unmarshal(data, &file); err != nil {
		return auth.Session{}, fmt.Errorf("session: decoding session file: %w", err)
	}
	return auth.Session{AccountID: file.AccountID, CreatedAt: file.CreatedAt}, nil
}
