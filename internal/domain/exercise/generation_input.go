package exercise

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// GenerationProfile is what challenge generation needs from the user's
// profile.
type GenerationProfile struct {
	Area            string
	ExperienceLevel string
	Language        string // the assistant language
}

// NoteExcerpt is one chunk of a session's attached notes, offered to
// challenge generation as context.
type NoteExcerpt struct {
	Source  string
	Content string
}

// MaxPracticeGoalLength bounds the free text a user writes to say what a
// practice challenge should train, counted in characters.
const MaxPracticeGoalLength = 500

var (
	// ErrPracticeGoalRequired is returned when a practice challenge is
	// requested without saying what to practice.
	ErrPracticeGoalRequired = errors.New("practice goal is required")
	// ErrPracticeGoalTooLong is returned when the practice goal exceeds
	// MaxPracticeGoalLength.
	ErrPracticeGoalTooLong = errors.New("practice goal is too long")
)

// ChallengeSpec is what the user asks for when creating a challenge: how hard
// it is, which kind it is and, for a practice, what concept to train. See
// specs/phases/phase-03-architecture-lab/18-practice-challenges.md.
type ChallengeSpec struct {
	Difficulty   Difficulty
	Kind         ChallengeKind
	PracticeGoal string
}

// Normalized returns the spec with an empty kind read as an interview and the
// goal trimmed. An interview carries no goal, so it is dropped.
func (s ChallengeSpec) Normalized() ChallengeSpec {
	if s.Kind == "" {
		s.Kind = ChallengeKindInterview
	}
	s.PracticeGoal = strings.TrimSpace(s.PracticeGoal)
	if s.Kind != ChallengeKindPractice {
		s.PracticeGoal = ""
	}
	return s
}

// Validate reports the first rule the normalized spec breaks: a known
// difficulty and kind, and for a practice a goal that is not blank or too
// long.
func (s ChallengeSpec) Validate() error {
	if DefaultTimeLimit(s.Difficulty) == 0 {
		return ErrDifficultyInvalid
	}
	switch s.Kind {
	case ChallengeKindInterview:
		return nil
	case ChallengeKindPractice:
	default:
		return ErrChallengeKindInvalid
	}
	if s.PracticeGoal == "" {
		return ErrPracticeGoalRequired
	}
	if utf8.RuneCountInString(s.PracticeGoal) > MaxPracticeGoalLength {
		return ErrPracticeGoalTooLong
	}
	return nil
}
