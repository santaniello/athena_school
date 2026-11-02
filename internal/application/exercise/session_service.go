package exercise

import (
	"context"
	"fmt"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// SessionService lists the studied sessions a challenge can be generated
// from. It is read-only and returns no message content. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
type SessionService struct {
	sessions domainexercise.SessionOptionRepository
}

// NewSessionService creates a SessionService backed by sessions.
func NewSessionService(sessions domainexercise.SessionOptionRepository) *SessionService {
	return &SessionService{sessions: sessions}
}

// ListSessionsForGeneration returns every study session across all folders,
// most recently started first. The list is never nil.
func (s *SessionService) ListSessionsForGeneration(ctx context.Context) ([]domainexercise.SessionOption, error) {
	options, err := s.sessions.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing sessions for generation: %w", err)
	}
	if options == nil {
		return []domainexercise.SessionOption{}, nil
	}
	return options, nil
}
