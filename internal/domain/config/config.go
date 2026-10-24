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

// ModelSettings holds the user's model choice per AI feature, as OpenRouter
// model IDs. An empty field means "use the default" (the task's tier model,
// or the built-in embedding model), so a config without a models section
// behaves exactly as before configurable models existed. See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md.
type ModelSettings struct {
	Study               string
	ConversationSummary string
	QueryEnrichment     string
	NoteFormatting      string
	Embedding           string
}

// Config is the local app configuration persisted to ~/.athena/config.yaml.
type Config struct {
	OpenRouterKey string
	Models        ModelSettings
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
