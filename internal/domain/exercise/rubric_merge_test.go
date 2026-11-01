package exercise

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func challengeCriterion(key string, weight int) RubricCriterion {
	return RubricCriterion{
		Key:         key,
		Name:        "Name " + key,
		Description: "Description " + key,
		Weight:      weight,
		Mandatory:   key == "mandatory",
		Levels:      [LevelCount]string{"zero", "one", "two", "three", "four"},
	}
}

func weightsOf(criteria []RubricCriterion) []int {
	weights := make([]int, len(criteria))
	for i, criterion := range criteria {
		weights[i] = criterion.Weight
	}
	return weights
}

func TestMergeRubric_putsBaseFirstThenChallengeCriteriaInOrder(t *testing.T) {
	// Given the base rubric and two challenge criteria
	base := BaseRubric()
	challenge := []RubricCriterion{challengeCriterion("cache-policy", 10), challengeCriterion("mandatory", 10)}

	// When merging them
	merged, err := MergeRubric(base, challenge)

	// Then the base criteria come first, then the challenge ones, each with its source
	require.NoError(t, err)
	keys := make([]string, len(merged))
	for i, criterion := range merged {
		keys[i] = criterion.Key
	}
	assert.Equal(t, []string{"architecture", "scalability", "consistency", "resilience", "tradeoffs", "cache-policy", "mandatory"}, keys)
	for _, criterion := range merged[:5] {
		assert.Equal(t, CriterionSourceBase, criterion.Source, criterion.Key)
	}
	for _, criterion := range merged[5:] {
		assert.Equal(t, CriterionSourceChallenge, criterion.Source, criterion.Key)
	}
	assert.True(t, merged[6].Mandatory)
	assert.Equal(t, "Name cache-policy", merged[5].Name)
	assert.Equal(t, [LevelCount]string{"zero", "one", "two", "three", "four"}, merged[5].Levels)
}

func TestMergeRubric_scalesRelativeWeightsDistributingTheRemainderByPosition(t *testing.T) {
	// Given five base criteria of 20 and one challenge criterion of 20 (total 120)
	// When merging them
	merged, err := MergeRubric(BaseRubric(), []RubricCriterion{challengeCriterion("extra", 20)})

	// Then each is 16.67 and the 4 leftover points go to the earliest positions on equal remainders
	require.NoError(t, err)
	assert.Equal(t, []int{17, 17, 17, 17, 16, 16}, weightsOf(merged))
}

func TestMergeRubric_remainderGoesToTheLargestFractionFirst(t *testing.T) {
	// Given base weights 1,1,1,1,1 and one challenge weight of 2 (total 7)
	base := BaseRubric()
	for i := range base {
		base[i].Weight = 1
	}

	// When merging them
	merged, err := MergeRubric(base, []RubricCriterion{challengeCriterion("heavy", 2)})

	// Then 100/7=14.28 (rem 2) for the base ones and 28.57 (rem 4) for the heavy one:
	// floors are 14*5+28=98, and the 2 leftover points go to the heavy one first, then the first base one
	require.NoError(t, err)
	assert.Equal(t, []int{15, 14, 14, 14, 14, 29}, weightsOf(merged))
}

func TestMergeRubric_smallChallengeWeightsStillGetOnePoint(t *testing.T) {
	// Given six challenge criteria of weight 1 next to the base rubric (total 106)
	challenge := make([]RubricCriterion, 6)
	for i := range challenge {
		challenge[i] = challengeCriterion(fmt.Sprintf("c%d", i), 1)
	}

	// When merging them
	merged, err := MergeRubric(BaseRubric(), challenge)

	// Then every challenge criterion keeps 1 point and the total is 100
	require.NoError(t, err)
	assert.Equal(t, []int{19, 19, 19, 19, 18, 1, 1, 1, 1, 1, 1}, weightsOf(merged))
}

func TestMergeRubric_isDeterministicAndDoesNotModifyItsInput(t *testing.T) {
	// Given the same inputs
	base := BaseRubric()
	challenge := []RubricCriterion{challengeCriterion("a", 7), challengeCriterion("b", 13)}

	// When merging twice
	first, err := MergeRubric(base, challenge)
	require.NoError(t, err)
	second, err := MergeRubric(base, challenge)
	require.NoError(t, err)

	// Then the results are equal and the inputs keep their relative weights and empty source
	assert.Equal(t, first, second)
	assert.Equal(t, []int{20, 20, 20, 20, 20}, weightsOf(base))
	assert.Equal(t, []int{7, 13}, weightsOf(challenge))
	assert.Empty(t, challenge[0].Source)
}

func TestMergeRubric_alwaysSumsToExactlyOneHundred(t *testing.T) {
	// Given many combinations of challenge counts and weights
	for count := 1; count <= MaxChallengeCriteria; count++ {
		for weight := 1; weight <= 100; weight += 3 {
			challenge := make([]RubricCriterion, count)
			for i := range challenge {
				challenge[i] = challengeCriterion(fmt.Sprintf("c%d", i), min(weight+i, MaxCriterionWeight))
			}

			// When merging them
			merged, err := MergeRubric(BaseRubric(), challenge)

			// Then the weights are whole numbers, each at least 1, summing to exactly 100
			require.NoError(t, err, "count %d weight %d", count, weight)
			total := 0
			for _, criterion := range merged {
				assert.GreaterOrEqual(t, criterion.Weight, 1)
				total += criterion.Weight
			}
			assert.Equal(t, 100, total, "count %d weight %d", count, weight)
		}
	}
}

func TestMergeRubric_rejectsInvalidInput(t *testing.T) {
	seven := make([]RubricCriterion, MaxChallengeCriteria+1)
	for i := range seven {
		seven[i] = challengeCriterion(fmt.Sprintf("c%d", i), 10)
	}
	heavyBase := BaseRubric()
	for i := range heavyBase {
		heavyBase[i].Weight = 100
	}

	tests := []struct {
		name      string
		base      []RubricCriterion
		challenge []RubricCriterion
		want      error
	}{
		{"no challenge criteria", BaseRubric(), nil, ErrChallengeCriteriaCount},
		{"too many challenge criteria", BaseRubric(), seven, ErrChallengeCriteriaCount},
		{"challenge key reuses a base key", BaseRubric(), []RubricCriterion{challengeCriterion("scalability", 10)}, ErrCriterionKeyDuplicate},
		{"challenge keys repeat", BaseRubric(), []RubricCriterion{challengeCriterion("a", 10), challengeCriterion("a", 10)}, ErrCriterionKeyDuplicate},
		{"zero weight", BaseRubric(), []RubricCriterion{challengeCriterion("a", 0)}, ErrCriterionWeightInvalid},
		{"negative weight", BaseRubric(), []RubricCriterion{challengeCriterion("a", -5)}, ErrCriterionWeightInvalid},
		{"weight above 100", BaseRubric(), []RubricCriterion{challengeCriterion("a", 101)}, ErrCriterionWeightInvalid},
		{"weight that cannot reach one point", heavyBase, []RubricCriterion{challengeCriterion("a", 1)}, ErrWeightsCannotSum},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given invalid input
			// When merging it
			merged, err := MergeRubric(tt.base, tt.challenge)

			// Then it is rejected with the specific error and nothing is returned
			assert.ErrorIs(t, err, tt.want)
			assert.Nil(t, merged)
		})
	}
}

func TestMergeRubric_acceptsExactlyTheMaximumNumberOfChallengeCriteria(t *testing.T) {
	// Given six challenge criteria
	challenge := make([]RubricCriterion, MaxChallengeCriteria)
	for i := range challenge {
		challenge[i] = challengeCriterion(fmt.Sprintf("c%d", i), 10)
	}

	// When merging them
	merged, err := MergeRubric(BaseRubric(), challenge)

	// Then all eleven criteria are kept
	require.NoError(t, err)
	assert.Len(t, merged, 11)
}
