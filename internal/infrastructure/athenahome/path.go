// Package athenahome resolves paths inside the user's ~/.athena directory.
// It never reads os.UserHomeDir() as a side effect of other packages —
// callers that need a testable path (SQLite DB, session file) receive one
// explicitly and only the app bootstrap calls Dir/File directly.
package athenahome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const dirName = ".athena"

// Dir returns the path to ~/.athena, creating it with owner-only
// permissions if it does not already exist.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, dirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// File returns the path to a file named name inside ~/.athena. It rejects
// any name that would resolve outside that directory.
func File(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if path != dir && !strings.HasPrefix(path, dir+string(filepath.Separator)) {
		return "", errors.New("athenahome: resulting path escapes ~/.athena")
	}
	return path, nil
}
