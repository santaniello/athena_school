package settings

import (
	"context"
	"fmt"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// GetPassScore returns the minimum passing score of a challenge attempt. The
// default is returned only when none is stored; a config that cannot be read,
// or holds an invalid value, is an error, so a threshold that was not verified
// is never used. See
// specs/phases/phase-03-architecture-lab/06-completion-rules.md.
func (s *Service) GetPassScore() (int, error) {
	cfg, err := s.loadConfig()
	if err != nil {
		return 0, err
	}
	if cfg.PassScore == 0 {
		return domainexercise.DefaultPassScore, nil
	}
	if err := domainexercise.ValidatePassScore(cfg.PassScore); err != nil {
		return 0, fmt.Errorf("settings: stored pass score: %w", err)
	}
	return cfg.PassScore, nil
}

// SetPassScore validates and stores the minimum passing score next to the
// rest of the config, which is preserved. Nothing is stored for an invalid
// score.
func (s *Service) SetPassScore(score int) error {
	if err := domainexercise.ValidatePassScore(score); err != nil {
		return err
	}
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	cfg.PassScore = score
	if err := s.config.Save(cfg); err != nil {
		return fmt.Errorf("settings: saving pass score: %w", err)
	}
	return nil
}

// Current implements the exercise service's PassScoreProvider port.
func (s *Service) Current(_ context.Context) (int, error) {
	return s.GetPassScore()
}
