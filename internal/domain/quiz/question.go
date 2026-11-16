// Package quiz holds the Interactive Quiz model: a set of multiple-choice and
// true/false questions about a study session, graded from 0 to 10 by counting
// correct answers, plus the ports its adapters implement. It is a separate
// context from the exercise engine and never imports it. See
// specs/decisions/ADR-007-interactive-quiz.md and
// specs/phases/phase-03-architecture-lab/21-interactive-quiz-core.md.
package quiz

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Size limits, counted in characters, that keep a generated question from
// storing an unbounded blob.
const (
	MaxPromptLength = 1000
	MaxOptionLength = 500
	MinOptions      = 2
	MaxOptions      = 6
	// TrueFalseOptions is the fixed number of options of a true/false question.
	TrueFalseOptions = 2
)

var (
	// ErrQuestionPromptRequired is returned when a question has no prompt.
	ErrQuestionPromptRequired = errors.New("question prompt is required")
	// ErrQuestionPromptTooLong is returned when the prompt exceeds MaxPromptLength.
	ErrQuestionPromptTooLong = errors.New("question prompt is too long")
	// ErrQuestionExplanationRequired is returned when a question has no explanation.
	ErrQuestionExplanationRequired = errors.New("question explanation is required")
	// ErrQuestionKindInvalid is returned for a kind other than multiple choice or true/false.
	ErrQuestionKindInvalid = errors.New("question kind is invalid")
	// ErrQuestionOptionsInvalid is returned when the number of options does not fit the kind.
	ErrQuestionOptionsInvalid = errors.New("question options count is invalid")
	// ErrQuestionOptionEmpty is returned when an option is blank.
	ErrQuestionOptionEmpty = errors.New("question option is empty")
	// ErrQuestionOptionDuplicate is returned when two options are the same text.
	ErrQuestionOptionDuplicate = errors.New("question option is duplicated")
	// ErrQuestionOptionTooLong is returned when an option exceeds MaxOptionLength.
	ErrQuestionOptionTooLong = errors.New("question option is too long")
	// ErrQuestionCorrectIndexInvalid is returned when the correct index is not an option.
	ErrQuestionCorrectIndexInvalid = errors.New("question correct index is out of range")
	// ErrPermutationInvalid is returned when a shuffle is not a permutation of the options.
	ErrPermutationInvalid = errors.New("shuffle permutation is invalid")
)

// QuestionKind says how a question is answered.
type QuestionKind string

// The kinds of question.
const (
	KindMultipleChoice QuestionKind = "MULTIPLE_CHOICE"
	KindTrueFalse      QuestionKind = "TRUE_FALSE"
)

// Question is one question with its options, the index of the single correct
// option and the explanation shown after grading.
type Question struct {
	ID           string
	Position     int // 1-based, in the order the user sees it
	Kind         QuestionKind
	Prompt       string
	Options      []string
	CorrectIndex int
	Explanation  string
}

// Validate reports the first rule the question breaks.
func (q Question) Validate() error {
	prompt := strings.TrimSpace(q.Prompt)
	if prompt == "" {
		return ErrQuestionPromptRequired
	}
	if utf8.RuneCountInString(prompt) > MaxPromptLength {
		return ErrQuestionPromptTooLong
	}
	if strings.TrimSpace(q.Explanation) == "" {
		return ErrQuestionExplanationRequired
	}
	if err := q.validateOptions(); err != nil {
		return err
	}
	if q.CorrectIndex < 0 || q.CorrectIndex >= len(q.Options) {
		return ErrQuestionCorrectIndexInvalid
	}
	return nil
}

func (q Question) validateOptions() error {
	switch q.Kind {
	case KindMultipleChoice:
		if len(q.Options) < MinOptions || len(q.Options) > MaxOptions {
			return ErrQuestionOptionsInvalid
		}
	case KindTrueFalse:
		if len(q.Options) != TrueFalseOptions {
			return ErrQuestionOptionsInvalid
		}
	default:
		return ErrQuestionKindInvalid
	}
	seen := make(map[string]bool, len(q.Options))
	for _, option := range q.Options {
		text := strings.TrimSpace(option)
		if text == "" {
			return ErrQuestionOptionEmpty
		}
		if utf8.RuneCountInString(text) > MaxOptionLength {
			return ErrQuestionOptionTooLong
		}
		key := strings.ToLower(text)
		if seen[key] {
			return ErrQuestionOptionDuplicate
		}
		seen[key] = true
	}
	return nil
}

// Shuffled returns a copy of a multiple-choice question whose options are
// reordered: position i of the result holds the option that was at
// permutation[i]. The correct index follows its option. A true/false question
// keeps its order, so True stays before False. The permutation must hold each
// index of the options exactly once.
func (q Question) Shuffled(permutation []int) (Question, error) {
	if q.Kind != KindMultipleChoice {
		return q, nil
	}
	if !isPermutation(permutation, len(q.Options)) {
		return Question{}, ErrPermutationInvalid
	}
	shuffled := q
	shuffled.Options = make([]string, len(q.Options))
	for position, source := range permutation {
		shuffled.Options[position] = q.Options[source]
		if source == q.CorrectIndex {
			shuffled.CorrectIndex = position
		}
	}
	return shuffled, nil
}

func isPermutation(permutation []int, size int) bool {
	if len(permutation) != size {
		return false
	}
	seen := make([]bool, size)
	for _, index := range permutation {
		if index < 0 || index >= size || seen[index] {
			return false
		}
		seen[index] = true
	}
	return true
}

func questionError(position int, err error) error {
	return fmt.Errorf("question %d: %w", position, err)
}
