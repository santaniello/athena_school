package exercise

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRubricVersion(t *testing.T) RubricVersion {
	t.Helper()
	merged, err := MergeRubric(BaseRubric(), []RubricCriterion{challengeCriterion("cache-policy", 10), challengeCriterion("mandatory", 10)})
	require.NoError(t, err)
	return RubricVersion{ID: "rubric-1", ChallengeID: "challenge-1", Version: 1, BaseVersion: BaseRubricVersion, Criteria: merged}
}

func mergedChallengeCriterion(key string) RubricCriterion {
	criterion := challengeCriterion(key, 1)
	criterion.Source = CriterionSourceChallenge
	return criterion
}

func TestRubricVersion_Validate_acceptsAMergedRubric(t *testing.T) {
	// Given a rubric produced by MergeRubric
	version := validRubricVersion(t)

	// When validating it
	err := version.Validate()

	// Then it is valid
	assert.NoError(t, err)
}

func TestRubricVersion_Validate_acceptsTheLargestRubric(t *testing.T) {
	// Given the base criteria plus six challenge criteria (eleven in total)
	challenge := []RubricCriterion{}
	for _, key := range []string{"a", "b", "c", "d", "e", "f"} {
		challenge = append(challenge, challengeCriterion(key, 10))
	}
	merged, err := MergeRubric(BaseRubric(), challenge)
	require.NoError(t, err)
	version := RubricVersion{Criteria: merged}

	// When validating it
	// Then it is valid
	assert.NoError(t, version.Validate())
}

func TestRubricVersion_Validate_rejectsBrokenRubrics(t *testing.T) {
	long := func(n int) string { return strings.Repeat("x", n) }
	tests := []struct {
		name   string
		mutate func(v *RubricVersion)
		want   error
	}{
		{"a base criterion is missing", func(v *RubricVersion) { v.Criteria = v.Criteria[1:] }, ErrBaseCriteriaMissing},
		{"a base key is relabelled as challenge", func(v *RubricVersion) { v.Criteria[0].Source = CriterionSourceChallenge }, ErrBaseCriteriaMissing},
		{"no challenge criterion", func(v *RubricVersion) { v.Criteria = v.Criteria[:5] }, ErrChallengeCriteriaCount},
		{"seven challenge criteria", func(v *RubricVersion) {
			for _, key := range []string{"c", "d", "e", "f", "g"} {
				v.Criteria = append(v.Criteria, mergedChallengeCriterion(key))
			}
		}, ErrChallengeCriteriaCount},
		{"more than eleven criteria in total", func(v *RubricVersion) {
			extra := BaseRubric()[0]
			extra.Key = "extra-base"
			for _, key := range []string{"c", "d", "e", "f"} {
				v.Criteria = append(v.Criteria, mergedChallengeCriterion(key))
			}
			v.Criteria = append(v.Criteria, extra)
		}, ErrTooManyCriteria},
		{"weights below one hundred", func(v *RubricVersion) { v.Criteria[0].Weight-- }, ErrWeightsCannotSum},
		{"weights above one hundred", func(v *RubricVersion) { v.Criteria[0].Weight++ }, ErrWeightsCannotSum},
		{"a zero weight balanced by another", func(v *RubricVersion) {
			v.Criteria[1].Weight += v.Criteria[0].Weight
			v.Criteria[0].Weight = 0
		}, ErrWeightsCannotSum},
		{"duplicate keys", func(v *RubricVersion) { v.Criteria[6].Key = v.Criteria[5].Key }, ErrCriterionKeyDuplicate},
		{"unknown source", func(v *RubricVersion) { v.Criteria[5].Source = "OTHER" }, ErrCriterionSourceInvalid},
		{"blank key", func(v *RubricVersion) { v.Criteria[5].Key = "  " }, ErrCriterionTextRequired},
		{"blank name", func(v *RubricVersion) { v.Criteria[5].Name = " " }, ErrCriterionTextRequired},
		{"blank description", func(v *RubricVersion) { v.Criteria[5].Description = "" }, ErrCriterionTextRequired},
		{"blank level descriptor", func(v *RubricVersion) { v.Criteria[5].Levels[3] = "" }, ErrCriterionTextRequired},
		{"key too long", func(v *RubricVersion) { v.Criteria[5].Key = long(MaxCriterionKeyLength + 1) }, ErrCriterionTextTooLong},
		{"name too long", func(v *RubricVersion) { v.Criteria[5].Name = long(MaxCriterionNameLength + 1) }, ErrCriterionTextTooLong},
		{"description too long", func(v *RubricVersion) { v.Criteria[5].Description = long(MaxCriterionDescriptionLength + 1) }, ErrCriterionTextTooLong},
		{"level descriptor too long", func(v *RubricVersion) { v.Criteria[5].Levels[4] = long(MaxLevelDescriptorLength + 1) }, ErrCriterionTextTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a valid rubric broken in one way
			version := validRubricVersion(t)
			tt.mutate(&version)

			// When validating it
			err := version.Validate()

			// Then that rule's own error is returned
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestRubricVersion_Validate_acceptsTextExactlyAtTheLimits(t *testing.T) {
	// Given a challenge criterion whose texts are at their maximum lengths
	version := validRubricVersion(t)
	version.Criteria[5].Key = strings.Repeat("k", MaxCriterionKeyLength)
	version.Criteria[5].Name = strings.Repeat("é", MaxCriterionNameLength)
	version.Criteria[5].Description = strings.Repeat("d", MaxCriterionDescriptionLength)
	version.Criteria[5].Levels[2] = strings.Repeat("l", MaxLevelDescriptorLength)

	// When validating it
	// Then it is valid (lengths are counted in characters)
	assert.NoError(t, version.Validate())
}
