package exercise

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

// IDontKnowAnswer is the fixed answer the "I don't know" button submits. The
// understanding evaluation treats it as no knowledge of the criterion.
const IDontKnowAnswer = "I don't know"

// MaxAnswerLength bounds a defense answer, in characters.
const MaxAnswerLength = 4000

var (
	// ErrDefenseNotActive is returned when a defense step is asked of an
	// attempt that is not in the state it needs, or when no question is left.
	ErrDefenseNotActive = errors.New("the defense is not active")
	// ErrQuestionPending is returned when the defense is evaluated while a
	// question still waits for its answer.
	ErrQuestionPending = errors.New("a question is waiting for its answer")
	// ErrNoQuestionPending is returned when an answer arrives and no question waits for it.
	ErrNoQuestionPending = errors.New("there is no question waiting for an answer")
	// ErrInvalidAnswer is returned for an answer that is blank or over MaxAnswerLength characters.
	ErrInvalidAnswer = errors.New("the answer must be 1 to 4000 characters")
)

// DefenseTurn is one question the tutor asked in the defense and the user's
// answer to it. See
// specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
type DefenseTurn struct {
	ID          string
	AttemptID   string
	Position    int    // 1..Challenge.DefenseQuestions
	CriterionID string // the criterion this question targets
	Question    string
	Answer      string // empty until answered
	AskedAt     time.Time
	AnsweredAt  time.Time // zero until answered
}

// Answered reports whether the question has its answer.
func (t DefenseTurn) Answered() bool {
	return !t.AnsweredAt.IsZero()
}

// ValidateAnswer checks that an answer is 1 to MaxAnswerLength characters once
// trimmed.
func ValidateAnswer(answer string) error {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > MaxAnswerLength {
		return ErrInvalidAnswer
	}
	return nil
}

// DefenseRepository persists the defense turns. They disappear with their
// unfinished attempt, so there is deliberately no Delete.
type DefenseRepository interface {
	// AddQuestion stores a new, unanswered turn. A position the attempt already
	// has is refused by the store.
	AddQuestion(ctx context.Context, turn DefenseTurn) error
	// SaveAnswer stores the answer of a turn that has none yet, else
	// ErrNoQuestionPending.
	SaveAnswer(ctx context.Context, turnID, answer string, answeredAt time.Time) error
	// ListByAttempt returns the attempt's turns in the order asked.
	ListByAttempt(ctx context.Context, attemptID string) ([]DefenseTurn, error)
}

// QuestionInput is everything a QuestionGenerator may look at. Previous holds
// the turns already asked, answered, in order.
type QuestionInput struct {
	Challenge  Challenge
	Rubric     RubricVersion
	Submission Submission
	Diagram    Evaluation
	Criterion  RubricCriterion // the criterion the question must target
	Position   int
	Total      int
	Previous   []DefenseTurn
}

// QuestionGenerator writes the text of the next defense question. The criterion
// it targets is chosen by the domain, not by the model. It is implemented per
// lab, so the generic engine knows nothing about diagrams.
type QuestionGenerator interface {
	GenerateQuestion(ctx context.Context, input QuestionInput) (string, error)
}

// UnderstandingInput is everything an UnderstandingEvaluator may look at.
type UnderstandingInput struct {
	Challenge  Challenge
	Rubric     RubricVersion
	Submission Submission
	Diagram    Evaluation
	Turns      []DefenseTurn // every question with its answer
}

// UnderstandingEvaluator judges the user's answers against the frozen rubric.
// It returns levels and text for the criteria that were asked, never a score.
type UnderstandingEvaluator interface {
	EvaluateUnderstanding(ctx context.Context, input UnderstandingInput) (EvaluatorOutput, error)
}

// SelectDefenseCriteria returns the IDs of the criteria the defense asks
// about, at most n and each once, in the order they are asked: mandatory
// criteria flagged InsufficientEvidence in the diagram evaluation, then the
// other flagged ones, then the rest from the lowest level up. Inside a group
// the heaviest criterion comes first, and equal ones keep the rubric's order,
// so the choice is deterministic. A criterion with no result counts as level 0.
func SelectDefenseCriteria(rubric RubricVersion, diagram Evaluation, n int) []string {
	results := make(map[string]CriterionResult, len(diagram.Results))
	for _, result := range diagram.Results {
		results[result.CriterionID] = result
	}
	ordered := slices.Clone(rubric.Criteria)
	slices.SortStableFunc(ordered, func(a, b RubricCriterion) int {
		ra, rb := results[a.ID], results[b.ID]
		if byGroup := defenseGroup(a, ra) - defenseGroup(b, rb); byGroup != 0 {
			return byGroup
		}
		if defenseGroup(a, ra) == defenseGroupLowLevel && ra.Level != rb.Level {
			return ra.Level - rb.Level
		}
		return b.Weight - a.Weight
	})
	selected := make([]string, 0, min(max(n, 0), len(ordered)))
	for _, criterion := range ordered {
		if len(selected) == n {
			break
		}
		selected = append(selected, criterion.ID)
	}
	return selected
}

const (
	defenseGroupMandatoryFlagged = iota
	defenseGroupFlagged
	defenseGroupLowLevel
)

func defenseGroup(criterion RubricCriterion, result CriterionResult) int {
	switch {
	case result.InsufficientEvidence && criterion.Mandatory:
		return defenseGroupMandatoryFlagged
	case result.InsufficientEvidence:
		return defenseGroupFlagged
	}
	return defenseGroupLowLevel
}

// Justifications the domain writes itself.
const (
	justificationNotAsked = "Not asked in the defense."
	justificationDontKnow = "The learner said they did not know."
)

// NewUnderstandingEvaluation validates the evaluator's output against the
// attempt's frozen rubric and the questions that were asked, and builds the
// UNDERSTANDING evaluation with the score computed here. The output must judge
// exactly the criteria of the turns, once each. Criteria that were not asked
// are flagged InsufficientEvidence, so the score is rescaled over the criteria
// asked, and a criterion answered with IDontKnowAnswer is level 0 whatever the
// model said. Every turn must be answered. Any defect is ErrInvalidEvaluation
// and nothing should be stored. The caller assigns the ID.
func NewUnderstandingEvaluation(
	attempt ChallengeAttempt, rubric RubricVersion, turns []DefenseTurn, output EvaluatorOutput, model string, now time.Time,
) (Evaluation, error) {
	for _, turn := range turns {
		if !turn.Answered() {
			return Evaluation{}, ErrQuestionPending
		}
	}
	results, err := understandingResults(rubric, turns, output.Results)
	if err != nil {
		return Evaluation{}, err
	}
	output.Results = results
	return newEvaluation(EvaluationKindUnderstanding, attempt, rubric, output, model, now)
}

// understandingResults returns one result per rubric criterion, in rubric
// order: the model's for the criteria asked, a flagged one for the others.
func understandingResults(rubric RubricVersion, turns []DefenseTurn, judged []CriterionResult) ([]CriterionResult, error) {
	asked := make(map[string]DefenseTurn, len(turns))
	for _, turn := range turns {
		asked[turn.CriterionID] = turn
	}
	byCriterion := make(map[string]CriterionResult, len(judged))
	for _, result := range judged {
		if _, wasAsked := asked[result.CriterionID]; !wasAsked {
			return nil, fmt.Errorf("%w: criterion %s was not asked", ErrInvalidEvaluation, result.CriterionID)
		}
		if _, repeated := byCriterion[result.CriterionID]; repeated {
			return nil, fmt.Errorf("%w: criterion %s is judged twice", ErrInvalidEvaluation, result.CriterionID)
		}
		byCriterion[result.CriterionID] = result
	}
	results := make([]CriterionResult, 0, len(rubric.Criteria))
	for _, criterion := range rubric.Criteria {
		turn, wasAsked := asked[criterion.ID]
		if !wasAsked {
			results = append(results, CriterionResult{CriterionID: criterion.ID, Justification: justificationNotAsked, InsufficientEvidence: true})
			continue
		}
		result, ok := byCriterion[criterion.ID]
		if !ok {
			return nil, fmt.Errorf("%w: criterion %s was asked but not judged", ErrInvalidEvaluation, criterion.ID)
		}
		if turn.Answer == IDontKnowAnswer {
			result = dontKnowResult(result)
		}
		results = append(results, result)
	}
	return results, nil
}

func dontKnowResult(result CriterionResult) CriterionResult {
	result.Level = 0
	result.InsufficientEvidence = false
	if strings.TrimSpace(result.Justification) == "" {
		result.Justification = justificationDontKnow
	}
	return result
}
