package exercise

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidEvaluation is returned when an evaluator's output does not fit the
// attempt's frozen rubric, so nothing is stored.
var ErrInvalidEvaluation = errors.New("evaluation is invalid")

// MaxCriterionLevel is the highest level a criterion can be judged at.
const MaxCriterionLevel = LevelCount - 1

// CriterionResult is the evaluator's judgement of one criterion of the frozen
// rubric. When InsufficientEvidence is set the criterion is not judged: it is
// left out of the score and Level is ignored.
type CriterionResult struct {
	CriterionID          string
	Level                int // 0..4
	Justification        string
	Evidence             string // optional quote or reference into the submission
	InsufficientEvidence bool
}

// ComputeScore returns the score, 0 to 100, of the results against the rubric:
// the weighted sum of level/4 over the judged criteria, rescaled to 100 and
// rounded half up. Flagged criteria are left out, unless the judged ones hold
// less than half of the rubric's weight: then the full rubric is used and
// flagged criteria count as level 0, so a nearly empty diagram cannot score
// high. The model never supplies the number. See
// specs/phases/phase-03-architecture-lab/05-evaluation-engine.md.
func ComputeScore(rubric RubricVersion, results []CriterionResult) (int, error) {
	byCriterion, err := resultsByCriterion(rubric, results)
	if err != nil {
		return 0, err
	}
	var totalWeight, judgedWeight, weightedLevels int
	for _, criterion := range rubric.Criteria {
		totalWeight += criterion.Weight
		result := byCriterion[criterion.ID]
		if result.InsufficientEvidence {
			continue
		}
		judgedWeight += criterion.Weight
		weightedLevels += criterion.Weight * result.Level
	}
	if totalWeight <= 0 {
		return 0, ErrInvalidEvaluation
	}
	denominatorWeight := judgedWeight
	if 2*judgedWeight < totalWeight {
		denominatorWeight = totalWeight
	}
	// round(100 × weightedLevels / (4 × denominatorWeight)), half up, in integers.
	denominator := MaxCriterionLevel * denominatorWeight
	return (200*weightedLevels + denominator) / (2 * denominator), nil
}

// resultsByCriterion indexes the results and checks they cover exactly the
// rubric's criteria, once each, at a level from 0 to 4.
func resultsByCriterion(rubric RubricVersion, results []CriterionResult) (map[string]CriterionResult, error) {
	if len(results) != len(rubric.Criteria) {
		return nil, ErrInvalidEvaluation
	}
	byCriterion := make(map[string]CriterionResult, len(results))
	for _, result := range results {
		if _, repeated := byCriterion[result.CriterionID]; repeated {
			return nil, ErrInvalidEvaluation
		}
		if result.Level < 0 || result.Level > MaxCriterionLevel {
			return nil, ErrInvalidEvaluation
		}
		byCriterion[result.CriterionID] = result
	}
	for _, criterion := range rubric.Criteria {
		if _, ok := byCriterion[criterion.ID]; !ok {
			return nil, ErrInvalidEvaluation
		}
	}
	return byCriterion, nil
}

// Bounds on the evaluator's text, in characters, so a runaway model cannot
// bloat the stored evaluation.
const (
	MaxSummaryLength       = 2000
	MaxJustificationLength = 1000
	MaxEvidenceLength      = 500
	MaxFeedbackItems       = 10
	MaxFeedbackItemLength  = 500
)

// EvaluationKind says what an evaluation judged. UNDERSTANDING is added by
// spec 3.12 on the same tables.
type EvaluationKind string

// EvaluationKindDiagram is the evaluation of the submitted artifact.
const EvaluationKindDiagram EvaluationKind = "DIAGRAM"

// Evaluation is the final, immutable scoring of one attempt. Score and the
// criterion levels are the technical result; Summary, Strengths, Gaps and
// Recommendations are feedback text and never change a score.
type Evaluation struct {
	ID              string
	AttemptID       string
	Kind            EvaluationKind
	Model           string // exact model id that produced it
	Score           int    // 0..100, computed by the domain
	Summary         string
	Strengths       []string
	Gaps            []string
	Recommendations []string
	Results         []CriterionResult
	CreatedAt       time.Time
}

// EvaluatorOutput is what an Evaluator returns: a level and a justification
// per criterion, plus feedback text. It carries no score.
type EvaluatorOutput struct {
	Model           string
	Summary         string
	Strengths       []string
	Gaps            []string
	Recommendations []string
	Results         []CriterionResult
}

// NewEvaluation validates the evaluator's output against the attempt's frozen
// rubric version and builds the DIAGRAM evaluation with the score computed
// here. Any defect is ErrInvalidEvaluation and nothing should be stored. The
// caller assigns the ID.
func NewEvaluation(attempt ChallengeAttempt, rubric RubricVersion, output EvaluatorOutput, model string, now time.Time) (Evaluation, error) {
	if attempt.RubricVersionID != rubric.ID {
		return Evaluation{}, fmt.Errorf("%w: rubric version is not the one the attempt froze", ErrInvalidEvaluation)
	}
	if err := validateOutputText(output); err != nil {
		return Evaluation{}, err
	}
	score, err := ComputeScore(rubric, output.Results)
	if err != nil {
		return Evaluation{}, err
	}
	return Evaluation{
		AttemptID:       attempt.ID,
		Kind:            EvaluationKindDiagram,
		Model:           model,
		Score:           score,
		Summary:         output.Summary,
		Strengths:       output.Strengths,
		Gaps:            output.Gaps,
		Recommendations: output.Recommendations,
		Results:         output.Results,
		CreatedAt:       now,
	}, nil
}

func validateOutputText(output EvaluatorOutput) error {
	if tooLong(output.Summary, MaxSummaryLength) {
		return fmt.Errorf("%w: summary is too long", ErrInvalidEvaluation)
	}
	for name, items := range map[string][]string{
		"strengths": output.Strengths, "gaps": output.Gaps, "recommendations": output.Recommendations,
	} {
		if err := validateFeedbackList(name, items); err != nil {
			return err
		}
	}
	for _, result := range output.Results {
		if strings.TrimSpace(result.Justification) == "" {
			return fmt.Errorf("%w: criterion %s has no justification", ErrInvalidEvaluation, result.CriterionID)
		}
		if tooLong(result.Justification, MaxJustificationLength) || tooLong(result.Evidence, MaxEvidenceLength) {
			return fmt.Errorf("%w: criterion %s has text that is too long", ErrInvalidEvaluation, result.CriterionID)
		}
	}
	return nil
}

func validateFeedbackList(name string, items []string) error {
	if len(items) > MaxFeedbackItems {
		return fmt.Errorf("%w: too many %s", ErrInvalidEvaluation, name)
	}
	for _, item := range items {
		if tooLong(item, MaxFeedbackItemLength) {
			return fmt.Errorf("%w: a %s item is too long", ErrInvalidEvaluation, name)
		}
	}
	return nil
}

func tooLong(text string, limit int) bool {
	return utf8.RuneCountInString(text) > limit
}
