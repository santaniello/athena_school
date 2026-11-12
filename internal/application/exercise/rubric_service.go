package exercise

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// maxVersionAttempts bounds how many times SetChallengeRubric re-reads the
// latest version and retries after another call took the version number it
// had chosen.
const maxVersionAttempts = 3

// RubricService implements the rubric use cases. A stored rubric version is
// immutable, so changing a challenge's rubric creates the next version and
// there is deliberately no update or delete. See
// specs/phases/phase-03-architecture-lab/02-rubrics.md.
type RubricService struct {
	challenges domainexercise.ChallengeRepository
	rubrics    domainexercise.RubricRepository
	now        func() time.Time
	newID      func() string
}

// NewRubricService creates a RubricService backed by the given ports.
func NewRubricService(challenges domainexercise.ChallengeRepository, rubrics domainexercise.RubricRepository) *RubricService {
	return &RubricService{
		challenges: challenges,
		rubrics:    rubrics,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      uuid.NewString,
	}
}

// ChallengeCriterionInput is one criterion written for a specific challenge.
// Weight is relative: the final weights are scaled together with the base
// criteria so the rubric sums to 100.
type ChallengeCriterionInput struct {
	Key         string
	Name        string
	Description string
	Weight      int
	Mandatory   bool
	Levels      [domainexercise.LevelCount]string
}

// SetChallengeRubric merges the base criteria (none for a practice challenge,
// whose rubric is lean) with the given challenge criteria, validates the result and stores it as the challenge's next
// version (1 if it has none). It returns domainexercise.ErrChallengeNotFound
// for an unknown challenge, and stores nothing when the criteria are invalid.
func (s *RubricService) SetChallengeRubric(ctx context.Context, challengeID string, criteria []ChallengeCriterionInput) (domainexercise.RubricVersion, error) {
	challenge, err := s.challenges.Get(ctx, challengeID)
	if err != nil {
		return domainexercise.RubricVersion{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}

	merged, err := domainexercise.MergeRubric(domainexercise.BaseCriteriaFor(challenge.Kind), s.toCriteria(criteria))
	if err != nil {
		return domainexercise.RubricVersion{}, fmt.Errorf("exercise: merging rubric: %w", err)
	}
	for i := range merged {
		merged[i].ID = s.newID()
	}

	version := domainexercise.RubricVersion{
		ID:          s.newID(),
		ChallengeID: challengeID,
		BaseVersion: domainexercise.BaseVersionFor(challenge.Kind),
		Criteria:    merged,
		CreatedAt:   s.now(),
	}
	if err := version.Validate(); err != nil {
		return domainexercise.RubricVersion{}, fmt.Errorf("exercise: invalid rubric: %w", err)
	}
	if err := s.createNextVersion(ctx, &version); err != nil {
		return domainexercise.RubricVersion{}, err
	}
	return version, nil
}

// createNextVersion numbers the version one above the challenge's latest and
// stores it. Reading the latest and storing are separate calls, so another
// call may take that number in between; the repository then reports
// ErrRubricVersionConflict and the number is chosen again, up to
// maxVersionAttempts times.
func (s *RubricService) createNextVersion(ctx context.Context, version *domainexercise.RubricVersion) error {
	var err error
	for range maxVersionAttempts {
		if version.Version, err = s.nextVersionNumber(ctx, version.ChallengeID); err != nil {
			return err
		}
		err = s.rubrics.Create(ctx, *version)
		if !errors.Is(err, domainexercise.ErrRubricVersionConflict) {
			break
		}
	}
	if err != nil {
		return fmt.Errorf("exercise: creating rubric version: %w", err)
	}
	return nil
}

// Get returns the rubric version with the given id, or
// domainexercise.ErrRubricNotFound.
func (s *RubricService) Get(ctx context.Context, versionID string) (domainexercise.RubricVersion, error) {
	version, err := s.rubrics.Get(ctx, versionID)
	if err != nil {
		return domainexercise.RubricVersion{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	return version, nil
}

// Latest returns the challenge's newest rubric version, or
// domainexercise.ErrRubricNotFound if it has none.
func (s *RubricService) Latest(ctx context.Context, challengeID string) (domainexercise.RubricVersion, error) {
	version, err := s.rubrics.Latest(ctx, challengeID)
	if err != nil {
		return domainexercise.RubricVersion{}, fmt.Errorf("exercise: getting latest rubric: %w", err)
	}
	return version, nil
}

// ListVersions returns every rubric version of the challenge, oldest first.
func (s *RubricService) ListVersions(ctx context.Context, challengeID string) ([]domainexercise.RubricVersion, error) {
	versions, err := s.rubrics.ListVersions(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing rubric versions: %w", err)
	}
	return versions, nil
}

func (s *RubricService) nextVersionNumber(ctx context.Context, challengeID string) (int, error) {
	latest, err := s.rubrics.Latest(ctx, challengeID)
	if errors.Is(err, domainexercise.ErrRubricNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("exercise: getting latest rubric: %w", err)
	}
	return latest.Version + 1, nil
}

// toCriteria converts the input to criteria with trimmed text.
func (s *RubricService) toCriteria(inputs []ChallengeCriterionInput) []domainexercise.RubricCriterion {
	criteria := make([]domainexercise.RubricCriterion, len(inputs))
	for i, input := range inputs {
		criteria[i] = domainexercise.RubricCriterion{
			Key:         strings.TrimSpace(input.Key),
			Name:        strings.TrimSpace(input.Name),
			Description: strings.TrimSpace(input.Description),
			Weight:      input.Weight,
			Mandatory:   input.Mandatory,
		}
		for level, descriptor := range input.Levels {
			criteria[i].Levels[level] = strings.TrimSpace(descriptor)
		}
	}
	return criteria
}
