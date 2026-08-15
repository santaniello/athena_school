// Package configfile provides the local file-backed implementation of
// config.Store: a YAML file at ~/.athena/config.yaml (path chosen by the
// caller, not this package — see internal/infrastructure/athenahome).
package configfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/santaniello/athena/internal/domain/config"
)

// Store is a file-backed implementation of config.Store.
type Store struct {
	path string
}

// NewStore creates a Store that reads and writes the config at path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

type configFile struct {
	OpenRouterKey string `yaml:"openrouter_key"`
}

// Save writes the config to disk with owner-only permissions, creating the
// parent directory if it does not exist.
func (s *Store) Save(cfg config.Config) error {
	data, err := yaml.Marshal(configFile{OpenRouterKey: cfg.OpenRouterKey})
	if err != nil {
		return fmt.Errorf("configfile: encoding config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("configfile: creating config directory: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("configfile: writing config file: %w", err)
	}
	return nil
}

// Load reads the config from disk.
func (s *Store) Load() (config.Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return config.Config{}, fmt.Errorf("configfile: reading config file: %w", err)
	}
	var file configFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return config.Config{}, fmt.Errorf("configfile: decoding config file: %w", err)
	}
	return config.Config{OpenRouterKey: file.OpenRouterKey}, nil
}
