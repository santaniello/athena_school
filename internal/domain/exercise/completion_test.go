package exercise

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mandatoryRubric returns a rubric c1..cN where the criteria at the given
// indexes are mandatory.
func mandatoryRubric(count int, mandatory ...int) RubricVersion {
	weights := make([]int, count)
	for i := range weights {
		weights[i] = 100 / count
	}
	rubric := scoringRubric(weights...)
	for _, index := range mandatory {
		rubric.Criteria[index].Mandatory = true
	}
	return rubric
}

// evaluationAt builds an evaluation of the kind with the score and levels.
func evaluationAt(kind EvaluationKind, score int, levels ...int) *Evaluation {
	return &Evaluation{Kind: kind, Score: score, Results: levelResults(levels...)}
}

func flagged(evaluation *Evaluation, index int) *Evaluation {
	evaluation.Results[index].InsufficientEvidence = true
	return evaluation
}

func TestValidatePassScore_acceptsTheInclusiveRange(t *testing.T) {
	tests := []struct {
		score   int
		wantErr bool
	}{
		{MinPassScore - 1, true},
		{MinPassScore, false},
		{DefaultPassScore, false},
		{MaxPassScore, false},
		{MaxPassScore + 1, true},
	}
	for _, tt := range tests {
		// Given a minimum score // When validating it
		err := ValidatePassScore(tt.score)

		// Then only the range 50 to 100 is accepted
		if tt.wantErr {
			assert.ErrorIs(t, err, ErrInvalidPassScore, "score %d", tt.score)
		} else {
			assert.NoError(t, err, "score %d", tt.score)
		}
	}
}

func TestEvaluateOutcome_isPendingUntilBothEvaluationsExist(t *testing.T) {
	// Given a rubric and a high diagram score
	rubric := mandatoryRubric(2)
	diagram := evaluationAt(EvaluationKindDiagram, 100, 4, 4)
	understanding := evaluationAt(EvaluationKindUnderstanding, 100, 4, 4)

	// When either evaluation is missing
	withoutUnderstanding := EvaluateOutcome(80, rubric, diagram, nil)
	withoutDiagram := EvaluateOutcome(80, rubric, nil, understanding)
	withNeither := EvaluateOutcome(80, rubric, nil, nil)

	// Then the result is PENDING with no reasons, never PASSED or FAILED
	for _, outcome := range []AttemptOutcome{withoutUnderstanding, withoutDiagram, withNeither} {
		assert.Equal(t, ResultPending, outcome.Result)
		assert.Empty(t, outcome.Reasons)
	}
}

func TestEvaluateOutcome_passesWhenScoresAndMandatoryCriteriaHold(t *testing.T) {
	// Given both scores at or above the minimum and mandatory criteria at level 2
	rubric := mandatoryRubric(3, 0, 1)
	diagram := evaluationAt(EvaluationKindDiagram, 80, 2, 2, 4)
	understanding := evaluationAt(EvaluationKindUnderstanding, 81, 3, 2, 0)

	// When evaluating the outcome
	outcome := EvaluateOutcome(80, rubric, diagram, understanding)

	// Then it passes, a score exactly at the minimum included
	assert.Equal(t, ResultPassed, outcome.Result)
	assert.Empty(t, outcome.Reasons)
}

func TestEvaluateOutcome_neverAveragesTheScores(t *testing.T) {
	// Given a diagram of 95 and an understanding of 70 at a minimum of 80
	rubric := mandatoryRubric(2)
	diagram := evaluationAt(EvaluationKindDiagram, 95, 4, 4)
	understanding := evaluationAt(EvaluationKindUnderstanding, 70, 3, 3)

	// When evaluating the outcome
	outcome := EvaluateOutcome(80, rubric, diagram, understanding)

	// Then it fails only on the understanding score
	assert.Equal(t, ResultFailed, outcome.Result)
	assert.Equal(t, []FailureReason{{Kind: FailureUnderstandingScore, Evaluation: EvaluationKindUnderstanding}}, outcome.Reasons)
}

func TestEvaluateOutcome_failsBelowTheMinimumByOnePoint(t *testing.T) {
	rubric := mandatoryRubric(2)

	diagramShort := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 79, 4, 4), evaluationAt(EvaluationKindUnderstanding, 100, 4, 4))
	understandingShort := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 100, 4, 4), evaluationAt(EvaluationKindUnderstanding, 79, 4, 4))

	assert.Equal(t, []FailureReason{{Kind: FailureDiagramScore, Evaluation: EvaluationKindDiagram}}, diagramShort.Reasons)
	assert.Equal(t, []FailureReason{{Kind: FailureUnderstandingScore, Evaluation: EvaluationKindUnderstanding}}, understandingShort.Reasons)
}

func TestEvaluateOutcome_listsEveryReasonThatApplies(t *testing.T) {
	// Given a diagram of 86 with a mandatory criterion at level 0, and a low understanding
	rubric := mandatoryRubric(3, 1)
	diagram := evaluationAt(EvaluationKindDiagram, 86, 4, 0, 4)
	understanding := evaluationAt(EvaluationKindUnderstanding, 60, 4, 1, 4)

	// When evaluating the outcome
	outcome := EvaluateOutcome(80, rubric, diagram, understanding)

	// Then every reason is listed, not only the first
	assert.Equal(t, ResultFailed, outcome.Result)
	assert.Equal(t, []FailureReason{
		{Kind: FailureUnderstandingScore, Evaluation: EvaluationKindUnderstanding},
		{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindDiagram, CriterionID: "c2"},
		{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindUnderstanding, CriterionID: "c2"},
	}, outcome.Reasons)
}

func TestEvaluateOutcome_requiresMandatoryCriteriaAtLevelTwoOrMore(t *testing.T) {
	rubric := mandatoryRubric(2, 0)

	atOne := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 90, 1, 4), evaluationAt(EvaluationKindUnderstanding, 90, 2, 4))
	atTwo := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 90, 2, 4), evaluationAt(EvaluationKindUnderstanding, 90, 2, 4))

	assert.Equal(t, []FailureReason{{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindDiagram, CriterionID: "c1"}}, atOne.Reasons)
	assert.Equal(t, ResultPassed, atTwo.Result)
}

func TestEvaluateOutcome_ignoresNonMandatoryCriteria(t *testing.T) {
	// Given a non-mandatory criterion at level 0 in both evaluations
	rubric := mandatoryRubric(2, 0)

	outcome := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 90, 4, 0), evaluationAt(EvaluationKindUnderstanding, 90, 4, 0))

	// Then it does not fail the attempt
	assert.Equal(t, ResultPassed, outcome.Result)
}

func TestEvaluateOutcome_judgesAFlaggedMandatoryCriterionInTheOtherEvaluation(t *testing.T) {
	rubric := mandatoryRubric(2, 0)

	// Given a criterion flagged in the diagram and judged at level 3 in the defense
	passes := EvaluateOutcome(80, rubric,
		flagged(evaluationAt(EvaluationKindDiagram, 90, 0, 4), 0), evaluationAt(EvaluationKindUnderstanding, 90, 3, 4))
	// And one flagged in the diagram and judged at level 1 in the defense
	failsInOther := EvaluateOutcome(80, rubric,
		flagged(evaluationAt(EvaluationKindDiagram, 90, 0, 4), 0), evaluationAt(EvaluationKindUnderstanding, 90, 1, 4))
	// And one flagged in the defense and judged at level 3 in the diagram
	passesReversed := EvaluateOutcome(80, rubric,
		evaluationAt(EvaluationKindDiagram, 90, 3, 4), flagged(evaluationAt(EvaluationKindUnderstanding, 90, 0, 4), 0))

	// Then the flagged side is not checked, the judged side still is
	assert.Equal(t, ResultPassed, passes.Result)
	assert.Equal(t, []FailureReason{{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindUnderstanding, CriterionID: "c1"}}, failsInOther.Reasons)
	assert.Equal(t, ResultPassed, passesReversed.Result)
}

func TestEvaluateOutcome_failsAMandatoryCriterionFlaggedInBothEvaluations(t *testing.T) {
	rubric := mandatoryRubric(2, 0)

	outcome := EvaluateOutcome(80, rubric,
		flagged(evaluationAt(EvaluationKindDiagram, 90, 4, 4), 0),
		flagged(evaluationAt(EvaluationKindUnderstanding, 90, 4, 4), 0))

	// Then nothing showed the user's knowledge of it, so it counts as not met
	require.Equal(t, ResultFailed, outcome.Result)
	assert.Equal(t, []FailureReason{
		{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindDiagram, CriterionID: "c1"},
		{Kind: FailureMandatoryCriterion, Evaluation: EvaluationKindUnderstanding, CriterionID: "c1"},
	}, outcome.Reasons)
}

func TestEvaluateOutcome_usesTheGivenMinimumNotTheDefault(t *testing.T) {
	rubric := mandatoryRubric(2)
	diagram := evaluationAt(EvaluationKindDiagram, 60, 4, 4)
	understanding := evaluationAt(EvaluationKindUnderstanding, 60, 4, 4)

	assert.Equal(t, ResultPassed, EvaluateOutcome(60, rubric, diagram, understanding).Result)
	assert.Equal(t, ResultFailed, EvaluateOutcome(61, rubric, diagram, understanding).Result)
}

func summary(running bool, result AttemptResult, diagramScore int, submittedAt time.Time) AttemptSummary {
	return AttemptSummary{
		Running:      running,
		Outcome:      AttemptOutcome{Result: result},
		DiagramScore: diagramScore,
		SubmittedAt:  submittedAt,
	}
}

func TestDeriveProgress_statusFollowsTheAttempts(t *testing.T) {
	day := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		attempts []AttemptSummary
		want     ProgressStatus
	}{
		{"no attempts is not started", nil, ProgressNotStarted},
		{"a running attempt is in progress", []AttemptSummary{summary(true, ResultPending, 0, time.Time{})}, ProgressInProgress},
		{"a submitted attempt not yet decided is in progress", []AttemptSummary{summary(false, ResultPending, 90, day)}, ProgressInProgress},
		{"a failed attempt needs improvement", []AttemptSummary{summary(false, ResultFailed, 70, day)}, ProgressNeedsImprovement},
		{"a passed attempt is completed", []AttemptSummary{summary(false, ResultPassed, 90, day)}, ProgressCompleted},
		{"a running attempt after a failure is in progress", []AttemptSummary{
			summary(false, ResultFailed, 70, day), summary(true, ResultPending, 0, time.Time{}),
		}, ProgressInProgress},
		{"a later failure keeps the completion", []AttemptSummary{
			summary(false, ResultPassed, 90, day), summary(false, ResultFailed, 40, day.Add(time.Hour)),
		}, ProgressCompleted},
		{"a later running attempt keeps the completion", []AttemptSummary{
			summary(false, ResultPassed, 90, day), summary(true, ResultPending, 0, time.Time{}),
		}, ProgressCompleted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given the attempts of a challenge // When deriving its progress
			progress := DeriveProgress(tt.attempts)

			// Then the status is the one the rules define
			assert.Equal(t, tt.want, progress.Status)
		})
	}
}

func TestDeriveProgress_summarizesCountScoreAndFirstPass(t *testing.T) {
	// Given three attempts: a failure, a first pass and a later, better pass
	first := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	attempts := []AttemptSummary{
		summary(false, ResultFailed, 55, first),
		summary(false, ResultPassed, 82, first.Add(48*time.Hour)),
		summary(false, ResultPassed, 95, first.Add(72*time.Hour)),
	}

	// When deriving the progress, in either order
	progress := DeriveProgress(attempts)
	reversed := DeriveProgress([]AttemptSummary{attempts[2], attempts[1], attempts[0]})

	// Then the count, the best diagram score and the earliest pass are reported
	for _, got := range []ChallengeProgress{progress, reversed} {
		assert.Equal(t, 3, got.Attempts)
		assert.Equal(t, 95, got.BestDiagramScore)
		assert.Equal(t, first.Add(48*time.Hour), got.FirstPassedAt)
	}
}

func TestDeriveProgress_hasZeroValuesWhenNothingHappened(t *testing.T) {
	progress := DeriveProgress([]AttemptSummary{summary(true, ResultPending, 0, time.Time{})})

	assert.Equal(t, 1, progress.Attempts)
	assert.Zero(t, progress.BestDiagramScore)
	assert.True(t, progress.FirstPassedAt.IsZero())
}

func TestDeriveProgress_aPassedAttemptWithoutSubmissionTimeStillCompletes(t *testing.T) {
	progress := DeriveProgress([]AttemptSummary{summary(false, ResultPassed, 90, time.Time{})})

	assert.Equal(t, ProgressCompleted, progress.Status)
}
