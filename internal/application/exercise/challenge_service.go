package exercise

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// ChallengeService implements the challenge catalog use cases. A challenge's
// content is immutable after creation, so there is deliberately no Update.
type ChallengeService struct {
	challenges domainexercise.ChallengeRepository
	sessions   SessionTopicReader
	now        func() time.Time
	newID      func() string
}

// NewChallengeService creates a ChallengeService backed by the given ports.
func NewChallengeService(challenges domainexercise.ChallengeRepository, sessions SessionTopicReader) *ChallengeService {
	return &ChallengeService{
		challenges: challenges,
		sessions:   sessions,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      uuid.NewString,
	}
}

// CreateChallengeInput is the content of a new challenge plus the studied
// session it comes from. The topic is not an input: it is copied from that
// session.
type CreateChallengeInput struct {
	SourceSessionID           string
	Title                     string
	Scenario                  string
	FunctionalRequirements    []string
	NonFunctionalRequirements []string
	Constraints               []string
	Objectives                []string
	Difficulty                domainexercise.Difficulty
	Origin                    domainexercise.ChallengeOrigin
}

// Create validates and persists a challenge. Its topic is a snapshot of the
// source session's topic, normalized, so later renaming or deleting the
// session does not change it. It returns ErrSourceSessionNotFound when the
// session does not exist, and persists nothing when the input is invalid.
func (s *ChallengeService) Create(ctx context.Context, input CreateChallengeInput) (domainexercise.Challenge, error) {
	sessionID := strings.TrimSpace(input.SourceSessionID)
	if sessionID == "" {
		return domainexercise.Challenge{}, ErrSourceSessionNotFound
	}
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: reading source session: %w", err)
	}

	now := s.now()
	challenge := domainexercise.Challenge{
		ID:                        s.newID(),
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Title:                     strings.TrimSpace(input.Title),
		Scenario:                  strings.TrimSpace(input.Scenario),
		FunctionalRequirements:    trimItems(input.FunctionalRequirements),
		NonFunctionalRequirements: trimItems(input.NonFunctionalRequirements),
		Constraints:               trimItems(input.Constraints),
		Objectives:                trimItems(input.Objectives),
		Difficulty:                input.Difficulty,
		Topic:                     domainexercise.NormalizeTopic(session.Topic),
		SourceSessionID:           sessionID,
		Origin:                    input.Origin,
		TimeLimitMinutes:          domainexercise.DefaultTimeLimit(input.Difficulty),
		DefenseQuestions:          domainexercise.DefaultDefenseQuestions,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := challenge.Validate(); err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: invalid challenge: %w", err)
	}
	if err := s.challenges.Create(ctx, challenge); err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: creating challenge: %w", err)
	}
	return challenge, nil
}

// Get returns the challenge with the given id, or
// domainexercise.ErrChallengeNotFound.
func (s *ChallengeService) Get(ctx context.Context, id string) (domainexercise.Challenge, error) {
	challenge, err := s.challenges.Get(ctx, id)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	return challenge, nil
}

// List returns one page of the challenges matching filter.
func (s *ChallengeService) List(ctx context.Context, filter domainexercise.ChallengeFilter, page domainexercise.Page) (domainexercise.ChallengePage, error) {
	result, err := s.challenges.List(ctx, filter, page)
	if err != nil {
		return domainexercise.ChallengePage{}, fmt.Errorf("exercise: listing challenges: %w", err)
	}
	return result, nil
}

// trimItems returns a trimmed copy of items, never nil, so a challenge reads
// the same before and after a round trip through storage.
func trimItems(items []string) []string {
	trimmed := make([]string, len(items))
	for i, item := range items {
		trimmed[i] = strings.TrimSpace(item)
	}
	return trimmed
}
