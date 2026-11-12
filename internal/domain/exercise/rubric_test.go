package exercise

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseRubric_returnsTheFiveBaseCriteriaInOrder(t *testing.T) {
	// Given the built-in base rubric
	// When reading it
	criteria := BaseRubric()

	// Then it has the five base criteria in the documented order, each marked BASE
	keys := make([]string, len(criteria))
	for i, criterion := range criteria {
		keys[i] = criterion.Key
		assert.Equal(t, CriterionSourceBase, criterion.Source, criterion.Key)
		assert.Equal(t, 20, criterion.Weight, criterion.Key)
		assert.False(t, criterion.Mandatory, criterion.Key)
	}
	assert.Equal(t, []string{"architecture", "scalability", "consistency", "resilience", "tradeoffs"}, keys)
}

func TestBaseRubric_everyCriterionHasTextAndFiveDistinctDescriptors(t *testing.T) {
	// Given the built-in base rubric
	for _, criterion := range BaseRubric() {
		t.Run(criterion.Key, func(t *testing.T) {
			// When inspecting a criterion
			// Then its name, description and all five level descriptors are filled and distinct
			assert.NotEmpty(t, criterion.Name)
			assert.NotEmpty(t, criterion.Description)
			seen := map[string]struct{}{}
			for level, descriptor := range criterion.Levels {
				assert.NotEmpty(t, descriptor, "level %d", level)
				seen[descriptor] = struct{}{}
			}
			assert.Len(t, seen, 5)
		})
	}
}

func TestBaseRubric_returnsAFreshCopyEachCall(t *testing.T) {
	// Given a caller that mutates the returned base rubric
	first := BaseRubric()
	first[0].Name = "changed"
	first[0].Weight = 99

	// When reading it again
	second := BaseRubric()

	// Then the built-in definition is unchanged
	require.NotEqual(t, "changed", second[0].Name)
	assert.Equal(t, 20, second[0].Weight)
}

func TestBaseCriteriaFor_isEmptyForAPractice(t *testing.T) {
	// Given the two challenge kinds
	// When reading the base criteria and base version of each
	// Then an interview gets the base rubric and a practice gets none
	assert.Equal(t, BaseRubric(), BaseCriteriaFor(ChallengeKindInterview))
	assert.Equal(t, BaseRubricVersion, BaseVersionFor(ChallengeKindInterview))
	assert.Empty(t, BaseCriteriaFor(ChallengeKindPractice))
	assert.Equal(t, NoBaseRubricVersion, BaseVersionFor(ChallengeKindPractice))
}
