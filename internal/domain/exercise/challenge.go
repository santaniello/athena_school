// Package exercise holds the exercise lifecycle model (challenges and, in
// later specs, rubrics, attempts and evaluations) and the ports
// infrastructure adapters implement. It carries no diagram-specific field
// and never imports internal/domain/architecture. See
// specs/decisions/ADR-005-exercise-engine.md and
// specs/phases/phase-03-architecture-lab/01-challenge-domain-and-persistence.md.
package exercise

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Size limits that keep a generated challenge from storing an unbounded blob.
// Lengths are counted in characters, not bytes.
const (
	MaxTitleLength    = 200
	MaxScenarioLength = 5000
	MaxListItemLength = 500
	MaxListItems      = 30
)

var (
	// ErrChallengeTitleRequired is returned when the title is blank.
	ErrChallengeTitleRequired = errors.New("challenge title is required")
	// ErrChallengeScenarioRequired is returned when the scenario is blank.
	ErrChallengeScenarioRequired = errors.New("challenge scenario is required")
	// ErrChallengeTopicRequired is returned when the topic is blank.
	ErrChallengeTopicRequired = errors.New("challenge topic is required")
	// ErrChallengeTypeInvalid is returned for a type other than ARCHITECTURE.
	ErrChallengeTypeInvalid = errors.New("challenge type is invalid")
	// ErrDifficultyInvalid is returned for an unknown difficulty.
	ErrDifficultyInvalid = errors.New("challenge difficulty is invalid")
	// ErrOriginInvalid is returned for an unknown origin.
	ErrOriginInvalid = errors.New("challenge origin is invalid")
	// ErrFunctionalRequirementsRequired is returned when there is no functional requirement.
	ErrFunctionalRequirementsRequired = errors.New("challenge needs at least one functional requirement")
	// ErrObjectivesRequired is returned when there is no objective.
	ErrObjectivesRequired = errors.New("challenge needs at least one objective")
	// ErrListItemEmpty is returned when a list item is blank.
	ErrListItemEmpty = errors.New("challenge list item is empty")
	// ErrListItemDuplicate is returned when a list repeats an item.
	ErrListItemDuplicate = errors.New("challenge list item is duplicated")
	// ErrChallengeTitleTooLong is returned when the title exceeds MaxTitleLength.
	ErrChallengeTitleTooLong = errors.New("challenge title is too long")
	// ErrChallengeScenarioTooLong is returned when the scenario exceeds MaxScenarioLength.
	ErrChallengeScenarioTooLong = errors.New("challenge scenario is too long")
	// ErrListItemTooLong is returned when a list item exceeds MaxListItemLength.
	ErrListItemTooLong = errors.New("challenge list item is too long")
	// ErrTooManyListItems is returned when a list exceeds MaxListItems.
	ErrTooManyListItems = errors.New("challenge list has too many items")
)

// ChallengeType identifies which lab a challenge belongs to.
type ChallengeType string

// ChallengeTypeArchitecture is the only type accepted today.
const ChallengeTypeArchitecture ChallengeType = "ARCHITECTURE"

// ChallengeOrigin records how a challenge came to exist.
type ChallengeOrigin string

// The two challenge origins.
const (
	OriginManual      ChallengeOrigin = "MANUAL"
	OriginAIGenerated ChallengeOrigin = "AI_GENERATED"
)

// Difficulty is how demanding a challenge is.
type Difficulty string

// The three difficulty levels.
const (
	DifficultyBeginner     Difficulty = "BEGINNER"
	DifficultyIntermediate Difficulty = "INTERMEDIATE"
	DifficultyAdvanced     Difficulty = "ADVANCED"
)

// Challenge is a system-design problem that exists independently of any
// attempt, session or evaluation. Its content is immutable after creation.
type Challenge struct {
	ID                        string
	Type                      ChallengeType
	Title                     string
	Scenario                  string
	FunctionalRequirements    []string
	NonFunctionalRequirements []string
	Constraints               []string
	Objectives                []string
	Difficulty                Difficulty
	Topic                     string // snapshot of the source session's topic
	SourceSessionID           string // "" once the session is deleted
	Origin                    ChallengeOrigin
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

// Validate reports the first rule the challenge breaks, each with its own error.
func (c Challenge) Validate() error {
	if err := validateText(c.Title, ErrChallengeTitleRequired, MaxTitleLength, ErrChallengeTitleTooLong); err != nil {
		return err
	}
	if err := validateText(c.Scenario, ErrChallengeScenarioRequired, MaxScenarioLength, ErrChallengeScenarioTooLong); err != nil {
		return err
	}
	if strings.TrimSpace(c.Topic) == "" {
		return ErrChallengeTopicRequired
	}
	if err := c.validateEnums(); err != nil {
		return err
	}
	if len(c.FunctionalRequirements) == 0 {
		return ErrFunctionalRequirementsRequired
	}
	if len(c.Objectives) == 0 {
		return ErrObjectivesRequired
	}
	for _, items := range [][]string{c.FunctionalRequirements, c.NonFunctionalRequirements, c.Constraints, c.Objectives} {
		if err := validateList(items); err != nil {
			return err
		}
	}
	return nil
}

func (c Challenge) validateEnums() error {
	if c.Type != ChallengeTypeArchitecture {
		return ErrChallengeTypeInvalid
	}
	switch c.Difficulty {
	case DifficultyBeginner, DifficultyIntermediate, DifficultyAdvanced:
	default:
		return ErrDifficultyInvalid
	}
	switch c.Origin {
	case OriginManual, OriginAIGenerated:
	default:
		return ErrOriginInvalid
	}
	return nil
}

func validateText(value string, errRequired error, maxLength int, errTooLong error) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errRequired
	}
	if utf8.RuneCountInString(trimmed) > maxLength {
		return errTooLong
	}
	return nil
}

func validateList(items []string) error {
	if len(items) > MaxListItems {
		return ErrTooManyListItems
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return ErrListItemEmpty
		}
		if utf8.RuneCountInString(trimmed) > MaxListItemLength {
			return ErrListItemTooLong
		}
		if _, duplicate := seen[trimmed]; duplicate {
			return ErrListItemDuplicate
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}
