// Package config holds the local app configuration (today just the
// OpenRouter key) and the ports infrastructure adapters implement to store
// and validate it. It is deliberately not onboarding-specific: this package
// is also the config layer for specs/phases/phase-01-desktop-mvp/08-settings.md,
// which reuses the same validation logic instead of duplicating it.
package config

import (
	"context"
	"errors"
)

// ErrKeyInvalid is returned by KeyValidator when the given key is rejected
// by OpenRouter (missing, malformed, disabled or unauthorized).
var ErrKeyInvalid = errors.New("openrouter key is invalid or unauthorized")

// ErrMaxKnowledgeExtractionItemsOutOfRange is returned when the configured
// extraction limit is outside the supported 1-20 range.
var ErrMaxKnowledgeExtractionItemsOutOfRange = errors.New("maximum knowledge extraction items must be an integer between 1 and 20")

// DefaultMaxKnowledgeExtractionItems is the extraction ceiling used when the setting is absent.
const DefaultMaxKnowledgeExtractionItems = 8

// Config is the local app configuration persisted to ~/.athena/config.yaml.
type Config struct {
	OpenRouterKey               string
	MaxKnowledgeExtractionItems int
}

// WithDefaults returns a copy with unset optional values defaulted.
func (c Config) WithDefaults() Config {
	if c.MaxKnowledgeExtractionItems == 0 {
		c.MaxKnowledgeExtractionItems = DefaultMaxKnowledgeExtractionItems
	}
	return c
}

// Validate checks that configured values are within their supported ranges.
func (c Config) Validate() error {
	if c.MaxKnowledgeExtractionItems < 1 || c.MaxKnowledgeExtractionItems > 20 {
		return ErrMaxKnowledgeExtractionItemsOutOfRange
	}
	return nil
}

// Store persists the local Config.
type Store interface {
	Save(cfg Config) error
	Load() (Config, error)
}

// KeyValidator confirms an OpenRouter API key is valid before it is saved.
type KeyValidator interface {
	ValidateKey(ctx context.Context, key string) error
}
