package exercise

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defenseRubric returns a rubric of criteria c1..cN with the given weights; the
// mandatory ones are named by index.
func defenseRubric(weights []int, mandatory ...int) RubricVersion {
	rubric := scoringRubric(weights...)
	for _, index := range mandatory {
		rubric.Criteria[index].Mandatory = true
	}
	return rubric
}

// diagramEvaluation builds a diagram evaluation with one result per level; the
// criteria at the flagged indexes carry InsufficientEvidence.
func diagramEvaluation(levels []int, flagged ...int) Evaluation {
	results := levelResults(levels...)
	for _, index := range flagged {
		results[index].InsufficientEvidence = true
	}
	return Evaluation{Kind: EvaluationKindDiagram, Results: results}
}

func TestSelectDefenseCriteria_asksMandatoryFlaggedCriteriaFirst(t *testing.T) {
	// Given a flagged optional criterion that outweighs a flagged mandatory one
	rubric := defenseRubric([]int{20, 40, 20, 20}, 0)
	diagram := diagramEvaluation([]int{0, 0, 3, 3}, 0, 1)

	// When selecting two questions
	selected := SelectDefenseCriteria(rubric, diagram, 2)

	// Then the mandatory flagged criterion comes first, then the other flagged one
	assert.Equal(t, []string{"c1", "c2"}, selected)
}

func TestSelectDefenseCriteria_ordersFlaggedCriteriaOfAGroupByWeight(t *testing.T) {
	// Given three flagged mandatory criteria with different weights
	rubric := defenseRubric([]int{10, 50, 40}, 0, 1, 2)
	diagram := diagramEvaluation([]int{0, 0, 0}, 0, 1, 2)

	// When selecting all of them
	selected := SelectDefenseCriteria(rubric, diagram, 3)

	// Then the heaviest comes first
	assert.Equal(t, []string{"c2", "c3", "c1"}, selected)
}

func TestSelectDefenseCriteria_keepsRubricOrderForEqualWeights(t *testing.T) {
	// Given flagged criteria with the same weight
	rubric := defenseRubric([]int{20, 20, 20}, 0, 1, 2)
	diagram := diagramEvaluation([]int{0, 0, 0}, 0, 1, 2)

	// When selecting them
	selected := SelectDefenseCriteria(rubric, diagram, 3)

	// Then the order is the rubric's, so the choice is deterministic
	assert.Equal(t, []string{"c1", "c2", "c3"}, selected)
}

func TestSelectDefenseCriteria_thenAsksTheLowestLevelsByWeight(t *testing.T) {
	// Given nothing flagged and criteria judged at levels 3, 1, 1, 0
	rubric := defenseRubric([]int{25, 25, 30, 20})
	diagram := diagramEvaluation([]int{3, 1, 1, 0})

	// When selecting all four
	selected := SelectDefenseCriteria(rubric, diagram, 4)

	// Then the lowest level comes first and equal levels are ordered by weight
	assert.Equal(t, []string{"c4", "c3", "c2", "c1"}, selected)
}

func TestSelectDefenseCriteria_putsFlaggedBeforeLowLevelCriteria(t *testing.T) {
	// Given a flagged criterion judged at level 4 and an unflagged one at level 0
	rubric := defenseRubric([]int{50, 50})
	diagram := diagramEvaluation([]int{4, 0}, 0)

	// When selecting both
	selected := SelectDefenseCriteria(rubric, diagram, 2)

	// Then the flagged one is asked first
	assert.Equal(t, []string{"c1", "c2"}, selected)
}

func TestSelectDefenseCriteria_leavesOutMandatoryCriteriaBeyondTheQuestions(t *testing.T) {
	// Given four flagged mandatory criteria and three questions
	rubric := defenseRubric([]int{10, 40, 30, 20}, 0, 1, 2, 3)
	diagram := diagramEvaluation([]int{0, 0, 0, 0}, 0, 1, 2, 3)

	// When selecting
	selected := SelectDefenseCriteria(rubric, diagram, 3)

	// Then the lightest one is not asked
	assert.Equal(t, []string{"c2", "c3", "c4"}, selected)
}

func TestSelectDefenseCriteria_neverAsksACriterionTwiceNorMoreThanExist(t *testing.T) {
	// Given a rubric with two criteria and five questions
	rubric := defenseRubric([]int{50, 50})
	diagram := diagramEvaluation([]int{1, 2}, 0)

	// When selecting five
	selected := SelectDefenseCriteria(rubric, diagram, 5)

	// Then each criterion appears once and there are only two
	assert.Equal(t, []string{"c1", "c2"}, selected)
}

func TestSelectDefenseCriteria_treatsACriterionMissingFromTheEvaluationAsLevelZero(t *testing.T) {
	// Given an evaluation that has no result for c2
	rubric := defenseRubric([]int{50, 50})
	diagram := Evaluation{Results: levelResults(3)}

	// When selecting one question
	selected := SelectDefenseCriteria(rubric, diagram, 1)

	// Then the criterion without a result is the weakest
	assert.Equal(t, []string{"c2"}, selected)
}

func TestSelectDefenseCriteria_asksNothingForZeroQuestions(t *testing.T) {
	// Given a valid rubric and evaluation
	rubric := defenseRubric([]int{50, 50})

	// When selecting zero questions
	selected := SelectDefenseCriteria(rubric, diagramEvaluation([]int{1, 1}), 0)

	// Then nothing is selected
	assert.Empty(t, selected)
}

func TestValidateAnswer_acceptsTextAndTheFixedIDontKnow(t *testing.T) {
	for _, answer := range []string{"Because a queue absorbs bursts.", IDontKnowAnswer, "x", strings.Repeat("é", MaxAnswerLength)} {
		// Given an answer that is 1 to 4,000 characters
		// When validating it
		err := ValidateAnswer(answer)

		// Then it is accepted
		assert.NoError(t, err)
	}
}

func TestValidateAnswer_rejectsBlankAndTooLongAnswers(t *testing.T) {
	for name, answer := range map[string]string{
		"empty":    "",
		"blank":    "  \n\t ",
		"too long": strings.Repeat("é", MaxAnswerLength+1),
	} {
		t.Run(name, func(t *testing.T) {
			// Given an answer that is empty, blank or over 4,000 characters
			// When validating it
			err := ValidateAnswer(answer)

			// Then it is refused
			assert.ErrorIs(t, err, ErrInvalidAnswer)
		})
	}
}

func TestIDontKnowAnswer_isTheFixedText(t *testing.T) {
	// Given the button's fixed answer
	// When reading it
	// Then it is the text the evaluation looks for
	assert.Equal(t, "I don't know", IDontKnowAnswer)
}

func TestDefenseTurn_Answered_isTrueOnceAnAnswerWasStored(t *testing.T) {
	// Given a question with no answer and one with an answer time
	pending := DefenseTurn{Question: "Why a queue?"}
	answered := DefenseTurn{Question: "Why a queue?", Answer: "To absorb bursts.", AnsweredAt: evaluationNow}

	// When asking whether each was answered
	// Then only the second was
	assert.False(t, pending.Answered())
	assert.True(t, answered.Answered())
}

func TestChallengeAttempt_StartDefense_movesEvaluatedToDefending(t *testing.T) {
	// Given an evaluated attempt
	attempt := attemptIn(AttemptEvaluated)

	// When starting the defense
	started, err := attempt.StartDefense()

	// Then it is DEFENDING and the receiver is untouched
	require.NoError(t, err)
	assert.Equal(t, AttemptDefending, started.State)
	assert.Equal(t, AttemptEvaluated, attempt.State)
}

func TestChallengeAttempt_StartDefense_refusesEveryOtherState(t *testing.T) {
	for _, from := range []AttemptState{AttemptInProgress, AttemptSubmitted, AttemptEvaluating, AttemptEvaluationFailed, AttemptDefending, AttemptFinished} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that is not EVALUATED
			// When starting the defense
			_, err := attemptIn(from).StartDefense()

			// Then it is refused
			assert.ErrorIs(t, err, ErrDefenseNotActive)
		})
	}
}

func TestChallengeAttempt_FinishDefense_movesDefendingToFinished(t *testing.T) {
	// Given a defending attempt
	attempt := attemptIn(AttemptDefending)

	// When finishing the defense
	finished, err := attempt.FinishDefense()

	// Then it is FINISHED and the receiver is untouched
	require.NoError(t, err)
	assert.Equal(t, AttemptFinished, finished.State)
	assert.Equal(t, AttemptDefending, attempt.State)
}

func TestChallengeAttempt_FinishDefense_refusesEveryOtherState(t *testing.T) {
	for _, from := range []AttemptState{AttemptInProgress, AttemptSubmitted, AttemptEvaluating, AttemptEvaluationFailed, AttemptEvaluated, AttemptFinished} {
		t.Run(string(from), func(t *testing.T) {
			// Given an attempt that is not DEFENDING
			// When finishing the defense
			_, err := attemptIn(from).FinishDefense()

			// Then it is refused
			assert.ErrorIs(t, err, ErrDefenseNotActive)
		})
	}
}

func TestDefenseErrors_areDistinctSentinels(t *testing.T) {
	// Given the four defense refusals
	// When reading their messages
	// Then each is its own error
	assert.Equal(t, "the defense is not active", ErrDefenseNotActive.Error())
	assert.Equal(t, "a question is waiting for its answer", ErrQuestionPending.Error())
	assert.Equal(t, "there is no question waiting for an answer", ErrNoQuestionPending.Error())
	assert.Equal(t, "the answer must be 1 to 4000 characters", ErrInvalidAnswer.Error())
}

func defendingAttempt() ChallengeAttempt {
	return ChallengeAttempt{ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-1", State: AttemptDefending}
}

func answeredTurn(position int, criterionID, answer string) DefenseTurn {
	return DefenseTurn{
		ID: criterionID + "-turn", AttemptID: "attempt-1", Position: position, CriterionID: criterionID,
		Question: "Why?", Answer: answer, AskedAt: evaluationNow, AnsweredAt: evaluationNow,
	}
}

// understandingOutput judges the given criteria (by index) at the given levels.
func understandingOutput(judged map[int]int) EvaluatorOutput {
	output := validOutput()
	output.Results = nil
	for index := 0; index < 5; index++ {
		level, ok := judged[index]
		if !ok {
			continue
		}
		output.Results = append(output.Results, CriterionResult{CriterionID: criterionID(index), Level: level, Justification: "because"})
	}
	return output
}

func TestNewUnderstandingEvaluation_flagsTheCriteriaThatWereNotAsked(t *testing.T) {
	// Given a three criteria rubric where only c1 and c3 were asked
	rubric := scoringRubric(40, 30, 30)
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), answeredTurn(2, "c3", "A queue.")}
	output := understandingOutput(map[int]int{0: 4, 2: 2})

	// When building the understanding evaluation
	evaluation, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, output, "vendor/model-1", evaluationNow)

	// Then c2 is flagged, in rubric order, and the score is rescaled over the criteria asked
	require.NoError(t, err)
	assert.Equal(t, EvaluationKindUnderstanding, evaluation.Kind)
	assert.Equal(t, "attempt-1", evaluation.AttemptID)
	assert.Equal(t, "vendor/model-1", evaluation.Model)
	assert.Equal(t, evaluationNow, evaluation.CreatedAt)
	require.Len(t, evaluation.Results, 3)
	assert.Equal(t, "c1", evaluation.Results[0].CriterionID)
	assert.Equal(t, CriterionResult{CriterionID: "c2", Justification: "Not asked in the defense.", InsufficientEvidence: true}, evaluation.Results[1])
	assert.Equal(t, "c3", evaluation.Results[2].CriterionID)
	assert.Equal(t, 79, evaluation.Score) // (40×4 + 30×2) / (70×4) = 78.57 → 79
}

func TestNewUnderstandingEvaluation_carriesOverTheFeedbackText(t *testing.T) {
	// Given a valid output
	rubric := scoringRubric(50, 50)
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), answeredTurn(2, "c2", "A queue.")}
	output := understandingOutput(map[int]int{0: 4, 1: 4})
	output.Results[0].Evidence = "their first answer"

	// When building the evaluation
	evaluation, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, output, "m", evaluationNow)

	// Then the feedback and the evidence are kept
	require.NoError(t, err)
	assert.Equal(t, output.Summary, evaluation.Summary)
	assert.Equal(t, output.Strengths, evaluation.Strengths)
	assert.Equal(t, output.Gaps, evaluation.Gaps)
	assert.Equal(t, output.Recommendations, evaluation.Recommendations)
	assert.Equal(t, "their first answer", evaluation.Results[0].Evidence)
	assert.Equal(t, 100, evaluation.Score)
}

func TestNewUnderstandingEvaluation_scoresIDontKnowAsLevelZeroWhatTheModelSays(t *testing.T) {
	// Given c1 answered "I don't know", which the model judged at level 4 and flagged
	rubric := scoringRubric(50, 50)
	turns := []DefenseTurn{answeredTurn(1, "c1", IDontKnowAnswer), answeredTurn(2, "c2", "A queue.")}
	output := understandingOutput(map[int]int{0: 4, 1: 4})
	output.Results[0].InsufficientEvidence = true

	// When building the evaluation
	evaluation, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, output, "m", evaluationNow)

	// Then c1 is judged at level 0, not flagged, and the score reflects it
	require.NoError(t, err)
	assert.Equal(t, 0, evaluation.Results[0].Level)
	assert.False(t, evaluation.Results[0].InsufficientEvidence)
	assert.Equal(t, "because", evaluation.Results[0].Justification)
	assert.Equal(t, 50, evaluation.Score)
}

func TestNewUnderstandingEvaluation_givesIDontKnowAJustificationWhenTheModelLeftNone(t *testing.T) {
	// Given c1 answered "I don't know" and judged by the model without a justification
	rubric := scoringRubric(50, 50)
	turns := []DefenseTurn{answeredTurn(1, "c1", IDontKnowAnswer), answeredTurn(2, "c2", "A queue.")}
	output := understandingOutput(map[int]int{0: 0, 1: 4})
	output.Results[0].Justification = " "

	// When building the evaluation
	evaluation, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, output, "m", evaluationNow)

	// Then the domain explains the level itself
	require.NoError(t, err)
	assert.Equal(t, "The learner said they did not know.", evaluation.Results[0].Justification)
}

func TestNewUnderstandingEvaluation_rejectsOutputThatDoesNotMatchTheQuestionsAsked(t *testing.T) {
	rubric := scoringRubric(40, 30, 30)
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), answeredTurn(2, "c3", "A queue.")}
	tests := map[string]map[int]int{
		"a criterion that was asked is missing": {0: 4},
		"a criterion that was not asked":        {0: 4, 1: 3, 2: 2},
		"nothing at all":                        {},
	}
	for name, judged := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a model output that does not judge exactly the criteria asked
			// When building the evaluation
			_, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, understandingOutput(judged), "m", evaluationNow)

			// Then it is an invalid evaluation
			assert.ErrorIs(t, err, ErrInvalidEvaluation)
		})
	}
}

func TestNewUnderstandingEvaluation_rejectsARepeatedCriterion(t *testing.T) {
	// Given an output that judges c1 twice
	rubric := scoringRubric(50, 50)
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), answeredTurn(2, "c2", "A queue.")}
	output := understandingOutput(map[int]int{0: 4, 1: 4})
	output.Results = append(output.Results, output.Results[0])

	// When building the evaluation
	_, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, output, "m", evaluationNow)

	// Then it is an invalid evaluation
	assert.ErrorIs(t, err, ErrInvalidEvaluation)
}

func TestNewUnderstandingEvaluation_rejectsARubricThatIsNotTheFrozenVersion(t *testing.T) {
	// Given a rubric version other than the attempt's
	rubric := scoringRubric(50, 50)
	rubric.ID = "rubric-2"
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), answeredTurn(2, "c2", "A queue.")}

	// When building the evaluation
	_, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, understandingOutput(map[int]int{0: 4, 1: 4}), "m", evaluationNow)

	// Then it is an invalid evaluation
	assert.ErrorIs(t, err, ErrInvalidEvaluation)
}

func TestNewUnderstandingEvaluation_rejectsAQuestionStillWaitingForItsAnswer(t *testing.T) {
	// Given a transcript whose last question has no answer
	rubric := scoringRubric(50, 50)
	turns := []DefenseTurn{answeredTurn(1, "c1", "A cache."), {Position: 2, CriterionID: "c2", Question: "Why?", AskedAt: time.Time{}}}

	// When building the evaluation
	_, err := NewUnderstandingEvaluation(defendingAttempt(), rubric, turns, understandingOutput(map[int]int{0: 4, 1: 4}), "m", evaluationNow)

	// Then the defense is not complete
	assert.ErrorIs(t, err, ErrQuestionPending)
}
