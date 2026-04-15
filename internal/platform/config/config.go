package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultProvider  = "ollama"
	DefaultModel     = "llama3"
	DefaultOllamaHost = "http://localhost:11434"
)

// Config holds the persisted user preferences.
type Config struct {
	Provider string       `yaml:"provider"`
	Model    string       `yaml:"model"`
	Ollama   OllamaConfig `yaml:"ollama"`
}

// OllamaConfig holds Ollama-specific settings.
type OllamaConfig struct {
	Host string `yaml:"host"`
}

func defaults() *Config {
	return &Config{
		Provider: DefaultProvider,
		Model:    DefaultModel,
		Ollama:   OllamaConfig{Host: DefaultOllamaHost},
	}
}

// Load reads the config from path. Returns defaults when the file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults(), nil
		}
		return nil, err
	}

	cfg := defaults()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes cfg to path as YAML, creating parent directories as needed.
// The directory is created with perm 0700; the file is written with perm 0600.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// DefaultPath returns the XDG-compliant config path: ~/.config/athena/config.yaml.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "athena", "config.yaml"), nil
}
