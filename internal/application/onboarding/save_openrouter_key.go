package onboarding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainconfig "github.com/santaniello/athena/internal/domain/config"
)

// SaveOpenRouterKey validates key against OpenRouter and, on success,
// persists it to the local config. A blank key never reaches the validator.
func (s *Service) SaveOpenRouterKey(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return ErrOpenRouterKeyRequired
	}

	if err := s.validator.ValidateKey(ctx, key); err != nil {
		if errors.Is(err, domainconfig.ErrKeyInvalid) {
			return err
		}
		return fmt.Errorf("onboarding: validating openrouter key: %w", err)
	}

	if err := s.config.Save(domainconfig.Config{OpenRouterKey: key}); err != nil {
		return fmt.Errorf("onboarding: saving openrouter key: %w", err)
	}

	return nil
}
