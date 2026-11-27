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

func challengeOf(weights ...int) []RubricCriterion {
	challenge := make([]RubricCriterion, len(weights))
	for i, weight := range weights {
		challenge[i] = challengeCriterion(fmt.Sprintf("c%d", i), weight)
	}
	return challenge
}

func sumOf(weights []int) int {
	total := 0
	for _, weight := range weights {
		total += weight
	}
	return total
}

func TestMergeRubric_interviewGivesFiftyToTheBaseAndFiftyToTheChallengeCriteria(t *testing.T) {
	// Given five base criteria of 20 and one challenge criterion of any weight
	// When merging them
	merged, err := MergeRubric(BaseRubric(), challengeOf(3))

	// Then each base criterion has 10 and the challenge criterion has the other 50
	require.NoError(t, err)
	assert.Equal(t, []int{10, 10, 10, 10, 10, 50}, weightsOf(merged))
}

func TestMergeRubric_challengeWeightsAreRelativeWhateverTheScale(t *testing.T) {
	// Given the same relative weights written on two scales
	first, err := MergeRubric(BaseRubric(), challengeOf(26, 20, 20, 16, 14, 4))
	require.NoError(t, err)
	second, err := MergeRubric(BaseRubric(), challengeOf(52, 40, 40, 32, 28, 8))
	require.NoError(t, err)

	// Then both give the same distribution, with the smallest weight lifted by the floor
	assert.Equal(t, weightsOf(first), weightsOf(second))
	assert.Equal(t, []int{10, 10, 10, 10, 10, 10, 9, 9, 8, 8, 6}, weightsOf(first))
}

func TestMergeRubric_everyChallengeCriterionKeepsAtLeastTheFloor(t *testing.T) {
	// Given six challenge criteria where one weighs almost nothing next to the others
	merged, err := MergeRubric(BaseRubric(), challengeOf(100, 100, 100, 100, 100, 1))

	// Then the light one still has 5 points and the challenge share is exactly 50
	require.NoError(t, err)
	challengeWeights := weightsOf(merged)[5:]
	assert.Equal(t, 5, challengeWeights[5])
	assert.Equal(t, 50, sumOf(challengeWeights))
}

func TestMergeRubric_remainderGoesToTheLargestFractionFirstThenByPosition(t *testing.T) {
	// Given three equal challenge weights: 50-15=35 to spread, 11.67 each
	merged, err := MergeRubric(BaseRubric(), challengeOf(1, 1, 1))
	require.NoError(t, err)
	assert.Equal(t, []int{17, 17, 16}, weightsOf(merged)[5:])

	// And weights 2 and 1: 40*2/3=26.67 and 13.33
	merged, err = MergeRubric(BaseRubric(), challengeOf(2, 1))
	require.NoError(t, err)
	assert.Equal(t, []int{32, 18}, weightsOf(merged)[5:])

	// And a base split by weights 1,1,1,1,1 where 50/5 is exact
	base := BaseRubric()
	for i := range base {
		base[i].Weight = 1
	}
	merged, err = MergeRubric(base, challengeOf(1))
	require.NoError(t, err)
	assert.Equal(t, []int{10, 10, 10, 10, 10, 50}, weightsOf(merged))
}

func TestMergeRubric_baseSharePositionTiesGoToTheEarliest(t *testing.T) {
	// Given base weights 1,1,1 over 50 points (16.67 each)
	base := BaseRubric()[:3]
	for i := range base {
		base[i].Weight = 1
	}

	// When merging them
	merged, err := MergeRubric(base, challengeOf(1))

	// Then the extra point goes to the earliest positions
	require.NoError(t, err)
	assert.Equal(t, []int{17, 17, 16, 50}, weightsOf(merged))
}

func TestMergeRubric_leanRubricGivesAllHundredToTheChallengeCriteria(t *testing.T) {
	// Given no base criteria
	merged, err := MergeRubric(nil, challengeOf(10, 10, 10))

	// Then the three share 100 with a remainder going to the earliest
	require.NoError(t, err)
	assert.Equal(t, []int{34, 33, 33}, weightsOf(merged))
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

			// Then the base sums to 50, the challenge to 50 with each at least 5, and the total is 100
			require.NoError(t, err, "count %d weight %d", count, weight)
			weights := weightsOf(merged)
			assert.Equal(t, 50, sumOf(weights[:5]), "count %d weight %d", count, weight)
			assert.Equal(t, 50, sumOf(weights[5:]), "count %d weight %d", count, weight)
			for _, w := range weights[5:] {
				assert.GreaterOrEqual(t, w, 5)
			}
		}
	}
}

func TestMergeRubric_rejectsInvalidInput(t *testing.T) {
	seven := make([]RubricCriterion, MaxChallengeCriteria+1)
	for i := range seven {
		seven[i] = challengeCriterion(fmt.Sprintf("c%d", i), 10)
	}
	lopsidedBase := BaseRubric()
	lopsidedBase[0].Weight = 100
	for i := 1; i < len(lopsidedBase); i++ {
		lopsidedBase[i].Weight = 1
	}
	lopsidedBase = append(lopsidedBase, BaseRubric()...)
	for i := range lopsidedBase[5:] {
		lopsidedBase[5+i].Key += "-2"
		lopsidedBase[5+i].Weight = 1
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
		{"base weight that cannot reach one point", lopsidedBase, []RubricCriterion{challengeCriterion("a", 1)}, ErrWeightsCannotSum},
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

func TestMaxChallengeCriteriaFor_dependsOnKindAndDifficulty(t *testing.T) {
	tests := []struct {
		kind       ChallengeKind
		difficulty Difficulty
		want       int
	}{
		{ChallengeKindInterview, DifficultyBeginner, 3},
		{ChallengeKindInterview, DifficultyIntermediate, 4},
		{ChallengeKindInterview, DifficultyAdvanced, 6},
		{ChallengeKindPractice, DifficultyBeginner, 3},
		{ChallengeKindPractice, DifficultyAdvanced, 3},
	}
	for _, tt := range tests {
		t.Run(string(tt.kind)+"/"+string(tt.difficulty), func(t *testing.T) {
			// Given a kind and a difficulty
			// When asking for the cap
			// Then it matches the spec
			assert.Equal(t, tt.want, MaxChallengeCriteriaFor(tt.kind, tt.difficulty))
		})
	}
}

func TestAssignShare_acceptsAShareThatIsExactlyTheFloors(t *testing.T) {
	// Given ten criteria and a share of 50 with a floor of 5, so nothing is left to spread
	criteria := challengeOf(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	// When assigning the share
	err := assignShare(criteria, 50, 5)

	// Then every criterion keeps exactly the floor
	require.NoError(t, err)
	assert.Equal(t, []int{5, 5, 5, 5, 5, 5, 5, 5, 5, 5}, weightsOf(criteria))
}

func TestAssignShare_acceptsACriterionWithExactlyOnePoint(t *testing.T) {
	// Given two equal criteria sharing 2 points with no floor
	criteria := challengeOf(1, 1)

	// When assigning the share
	err := assignShare(criteria, 2, 0)

	// Then each has one point, which is enough
	require.NoError(t, err)
	assert.Equal(t, []int{1, 1}, weightsOf(criteria))
}

func TestAssignShare_rejectsAShareSmallerThanTheFloors(t *testing.T) {
	// Given three criteria and a share of 14 with a floor of 5
	// When assigning the share
	err := assignShare(challengeOf(1, 1, 1), 14, 5)

	// Then it cannot be done
	assert.ErrorIs(t, err, ErrWeightsCannotSum)
}
