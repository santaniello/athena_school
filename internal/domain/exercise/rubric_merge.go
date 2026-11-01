package exercise

import (
	"errors"
	"sort"
)

// Rubric shape limits.
const (
	MaxChallengeCriteria = 6
	MaxCriterionWeight   = 100
	rubricTotalWeight    = 100
)

var (
	// ErrChallengeCriteriaCount is returned when a rubric has no challenge
	// criterion or more than MaxChallengeCriteria.
	ErrChallengeCriteriaCount = errors.New("rubric needs between 1 and 6 challenge criteria")
	// ErrCriterionKeyDuplicate is returned when two criteria share a key,
	// including a challenge criterion reusing a base key.
	ErrCriterionKeyDuplicate = errors.New("rubric criterion key is duplicated")
	// ErrCriterionWeightInvalid is returned for a weight outside 1..MaxCriterionWeight.
	ErrCriterionWeightInvalid = errors.New("rubric criterion weight is invalid")
	// ErrWeightsCannotSum is returned when the weights cannot be scaled to
	// sum to 100 with every criterion keeping at least one point.
	ErrWeightsCannotSum = errors.New("rubric weights cannot sum to 100")
)

// MergeRubric returns the final criteria list: the base criteria first, then
// the challenge criteria, in the given order, with the challenge criteria
// marked CriterionSourceChallenge. The input weights are relative; they are
// scaled to whole numbers that sum to exactly 100, the rounding remainder
// going to the largest fractional parts first and, on a tie, to the earliest
// position. The inputs are not modified.
func MergeRubric(base, challenge []RubricCriterion) ([]RubricCriterion, error) {
	if len(challenge) == 0 || len(challenge) > MaxChallengeCriteria {
		return nil, ErrChallengeCriteriaCount
	}
	merged := make([]RubricCriterion, 0, len(base)+len(challenge))
	merged = append(merged, base...)
	for _, criterion := range challenge {
		criterion.Source = CriterionSourceChallenge
		merged = append(merged, criterion)
	}
	if err := checkMergeInput(merged); err != nil {
		return nil, err
	}
	if err := scaleWeights(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func checkMergeInput(criteria []RubricCriterion) error {
	seen := make(map[string]struct{}, len(criteria))
	for _, criterion := range criteria {
		if _, duplicate := seen[criterion.Key]; duplicate {
			return ErrCriterionKeyDuplicate
		}
		seen[criterion.Key] = struct{}{}
		if criterion.Weight < 1 || criterion.Weight > MaxCriterionWeight {
			return ErrCriterionWeightInvalid
		}
	}
	return nil
}

// scaleWeights rewrites the weights in place so they sum to 100 (largest
// remainder method). The criteria are a private copy owned by MergeRubric.
func scaleWeights(criteria []RubricCriterion) error {
	total := 0
	for _, criterion := range criteria {
		total += criterion.Weight
	}
	remainders := make([]int, len(criteria))
	assigned := 0
	for i := range criteria {
		scaled := criteria[i].Weight * rubricTotalWeight
		criteria[i].Weight = scaled / total
		remainders[i] = scaled % total
		assigned += criteria[i].Weight
	}

	order := make([]int, len(criteria))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return remainders[order[a]] > remainders[order[b]] })
	for _, index := range order[:rubricTotalWeight-assigned] {
		criteria[index].Weight++
	}

	for _, criterion := range criteria {
		if criterion.Weight < 1 {
			return ErrWeightsCannotSum
		}
	}
	return nil
}
