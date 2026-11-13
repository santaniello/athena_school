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

	// MinTimeLimitMinutes and MaxTimeLimitMinutes bound a challenge's exam time.
	MinTimeLimitMinutes = 1
	MaxTimeLimitMinutes = 180

	// The number of questions the tutor asks in the defense, per challenge.
	MinDefenseQuestions     = 3
	MaxDefenseQuestions     = 5
	DefaultDefenseQuestions = 5
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
	// ErrChallengeKindInvalid is returned for a kind other than INTERVIEW or PRACTICE.
	ErrChallengeKindInvalid = errors.New("challenge kind is invalid")
	// ErrStarterDiagramNotAllowed is returned when an interview carries a starter diagram.
	ErrStarterDiagramNotAllowed = errors.New("an interview challenge cannot have a starter diagram")
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
	// ErrTimeLimitInvalid is returned when the time limit is outside MinTimeLimitMinutes..MaxTimeLimitMinutes.
	ErrTimeLimitInvalid = errors.New("challenge time limit is out of range")
	// ErrDefenseQuestionsInvalid is returned when DefenseQuestions is outside MinDefenseQuestions..MaxDefenseQuestions.
	ErrDefenseQuestionsInvalid = errors.New("challenge defense questions are out of range")
)

// ChallengeType identifies which lab a challenge belongs to.
type ChallengeType string

// ChallengeTypeArchitecture is the only type accepted today.
const ChallengeTypeArchitecture ChallengeType = "ARCHITECTURE"

// ChallengeKind says how a challenge is shaped. It is separate from
// ChallengeType, which says which lab the challenge belongs to. See
// specs/phases/phase-03-architecture-lab/18-practice-challenges.md.
type ChallengeKind string

// The two challenge kinds. An interview simulates a job interview: a broad
// product scenario, a time limit and the full rubric. A practice trains one
// concept: a short focused scenario, no time limit and a lean rubric.
const (
	ChallengeKindInterview ChallengeKind = "INTERVIEW"
	ChallengeKindPractice  ChallengeKind = "PRACTICE"
)

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
	Kind                      ChallengeKind
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
	TimeLimitMinutes          int // exam length; copied into an attempt's Deadline. 0 means untimed (PRACTICE only)
	DefenseQuestions          int // questions the tutor asks after the diagram evaluation, 3 to 5
	// StarterDiagram is the opaque diagram (mxGraphModel XML) a practice
	// attempt starts from. Empty means a blank board, and an interview is
	// always blank. See specs/phases/phase-03-architecture-lab/19-practice-starter-diagram.md.
	StarterDiagram string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SourceSession is what a challenge needs from the study session it comes
// from: the topic copied into the challenge, and the session's goal, which
// challenge generation reads.
type SourceSession struct {
	Topic string
	Goal  string
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
	if err := c.validateTimeLimit(); err != nil {
		return err
	}
	if c.Kind == ChallengeKindInterview && c.StarterDiagram != "" {
		return ErrStarterDiagramNotAllowed
	}
	if c.DefenseQuestions < MinDefenseQuestions || c.DefenseQuestions > MaxDefenseQuestions {
		return ErrDefenseQuestionsInvalid
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

// Untimed reports whether the challenge has no time limit, which only a
// practice challenge may have.
func (c Challenge) Untimed() bool {
	return c.Kind == ChallengeKindPractice && c.TimeLimitMinutes == 0
}

func (c Challenge) validateTimeLimit() error {
	if c.Untimed() {
		return nil
	}
	if c.TimeLimitMinutes < MinTimeLimitMinutes || c.TimeLimitMinutes > MaxTimeLimitMinutes {
		return ErrTimeLimitInvalid
	}
	return nil
}

func (c Challenge) validateEnums() error {
	if c.Type != ChallengeTypeArchitecture {
		return ErrChallengeTypeInvalid
	}
	switch c.Kind {
	case ChallengeKindInterview, ChallengeKindPractice:
	default:
		return ErrChallengeKindInvalid
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

// DefaultTimeLimit returns the exam length in minutes a challenge of the given
// difficulty starts with, or 0 for an unknown difficulty.
func DefaultTimeLimit(d Difficulty) int {
	switch d {
	case DifficultyBeginner:
		return 30
	case DifficultyIntermediate:
		return 45
	case DifficultyAdvanced:
		return 60
	}
	return 0
}

// TimeLimitFor returns the exam length in minutes a new challenge of the
// given kind and difficulty starts with: none (0) for a practice, the
// difficulty's default for an interview.
func TimeLimitFor(kind ChallengeKind, d Difficulty) int {
	if kind == ChallengeKindPractice {
		return 0
	}
	return DefaultTimeLimit(d)
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
