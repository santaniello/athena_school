package exercise

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scoringRubric returns a rubric whose criteria c1..cN carry the given weights.
func scoringRubric(weights ...int) RubricVersion {
	criteria := make([]RubricCriterion, len(weights))
	for i, weight := range weights {
		criteria[i] = RubricCriterion{ID: criterionID(i), Weight: weight}
	}
	return RubricVersion{ID: "rubric-1", ChallengeID: "challenge-1", Criteria: criteria}
}

func criterionID(index int) string { return fmt.Sprintf("c%d", index+1) }

// levelResults builds one judged result per level, in criterion order.
func levelResults(levels ...int) []CriterionResult {
	results := make([]CriterionResult, len(levels))
	for i, level := range levels {
		results[i] = CriterionResult{CriterionID: criterionID(i), Level: level, Justification: "because"}
	}
	return results
}

func TestComputeScore_isTheWeightedSumOfLevelsOverFour(t *testing.T) {
	tests := []struct {
		name    string
		weights []int
		levels  []int
		want    int
	}{
		{"all zero levels give 0", []int{20, 20, 20, 20, 20}, []int{0, 0, 0, 0, 0}, 0},
		{"all top levels give 100", []int{20, 20, 20, 20, 20}, []int{4, 4, 4, 4, 4}, 100},
		{"weights matter", []int{50, 30, 20}, []int{4, 2, 0}, 65},
		{"rounds half up", []int{50, 50}, []int{1, 0}, 13},
		{"rounds down below half", []int{45, 55}, []int{1, 0}, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a rubric and one level per criterion
			rubric := scoringRubric(tt.weights...)

			// When computing the score
			score, err := ComputeScore(rubric, levelResults(tt.levels...))

			// Then it is Σ weight × level / 4, rounded half up
			require.NoError(t, err)
			assert.Equal(t, tt.want, score)
		})
	}
}

func TestComputeScore_isIndependentOfTheOrderOfTheResults(t *testing.T) {
	// Given results listed in the reverse order of the rubric
	rubric := scoringRubric(50, 30, 20)
	results := levelResults(4, 2, 0)
	results[0], results[2] = results[2], results[0]

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then it matches the in-order result
	require.NoError(t, err)
	assert.Equal(t, 65, score)
}

func TestComputeScore_leavesOutCriteriaFlaggedAsInsufficientEvidence(t *testing.T) {
	// Given five criteria of weight 20, the last one flagged, levels 4, 4, 3, 3
	rubric := scoringRubric(20, 20, 20, 20, 20)
	results := levelResults(4, 4, 3, 3, 0)
	results[4].InsufficientEvidence = true

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then the flagged criterion is not judged and the rest is rescaled: 88, not 70
	require.NoError(t, err)
	assert.Equal(t, 88, score)
}

func TestComputeScore_ignoresTheStoredLevelOfAFlaggedCriterion(t *testing.T) {
	// Given a flagged criterion that still carries a level
	rubric := scoringRubric(50, 50)
	results := levelResults(4, 4)
	results[1].InsufficientEvidence = true

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then only the judged criterion counts
	require.NoError(t, err)
	assert.Equal(t, 100, score)
}

func TestComputeScore_fallsBackToTheFullRubricWhenJudgedWeightIsBelowHalf(t *testing.T) {
	// Given a judged criterion of weight 40 out of 100 at level 4, the others flagged
	rubric := scoringRubric(40, 30, 30)
	results := levelResults(4, 0, 0)
	results[1].InsufficientEvidence = true
	results[2].InsufficientEvidence = true

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then flagged criteria count as level 0 over the full rubric: 40, not 100
	require.NoError(t, err)
	assert.Equal(t, 40, score)
}

func TestComputeScore_rescalesWhenJudgedWeightIsExactlyHalf(t *testing.T) {
	// Given a judged weight of exactly half the rubric at level 4
	rubric := scoringRubric(50, 50)
	results := levelResults(4, 0)
	results[1].InsufficientEvidence = true

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then the rule does not fall back: the judged part is rescaled to 100
	require.NoError(t, err)
	assert.Equal(t, 100, score)
}

func TestComputeScore_scoresZeroWhenEveryCriterionIsFlagged(t *testing.T) {
	// Given every criterion flagged
	rubric := scoringRubric(60, 40)
	results := levelResults(0, 0)
	results[0].InsufficientEvidence = true
	results[1].InsufficientEvidence = true

	// When computing the score
	score, err := ComputeScore(rubric, results)

	// Then nothing was judged and the fallback gives 0
	require.NoError(t, err)
	assert.Equal(t, 0, score)
}

func TestComputeScore_rejectsResultsThatDoNotMatchTheRubric(t *testing.T) {
	tests := []struct {
		name    string
		results []CriterionResult
	}{
		{"a missing criterion", levelResults(4)},
		{"an unknown criterion", []CriterionResult{
			{CriterionID: "c1", Level: 4}, {CriterionID: "other", Level: 4},
		}},
		{"a repeated criterion", []CriterionResult{
			{CriterionID: "c1", Level: 4}, {CriterionID: "c1", Level: 4},
		}},
		{"a level above 4", levelResults(4, 5)},
		{"a negative level", levelResults(4, -1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given results that break the rubric's shape
			// When computing the score
			_, err := ComputeScore(scoringRubric(50, 50), tt.results)

			// Then it is an invalid evaluation
			assert.ErrorIs(t, err, ErrInvalidEvaluation)
		})
	}
}

func TestComputeScore_rejectsARubricWithoutWeight(t *testing.T) {
	// Given a rubric whose weights sum to zero
	// When computing the score
	_, err := ComputeScore(scoringRubric(0), levelResults(4))

	// Then it is refused instead of dividing by zero
	assert.ErrorIs(t, err, ErrInvalidEvaluation)
}
