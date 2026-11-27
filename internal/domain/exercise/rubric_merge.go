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

// Shares of an interview rubric and the floor of a challenge criterion.
const (
	interviewBaseShare      = 50
	interviewChallengeShare = 50
	challengeCriterionFloor = 5
)

// MergeRubric returns the final criteria list: the base criteria first, then
// the challenge criteria, in the given order, with the challenge criteria
// marked CriterionSourceChallenge. A non-empty base means an interview split:
// the base criteria take 50 points between them by their own weights and the
// challenge criteria the other 50. An empty base means the lean rubric: the
// challenge criteria take all 100. The challenge weights are relative, so the
// scale used does not matter: every challenge criterion first receives
// challengeCriterionFloor points and the rest of the share is spread by the
// weights. Rounding remainders go to the largest fractional parts first and,
// on a tie, to the earliest position. The inputs are not modified.
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

	challengeShare := rubricTotalWeight
	if len(base) > 0 {
		challengeShare = interviewChallengeShare
		if err := assignShare(merged[:len(base)], interviewBaseShare, 0); err != nil {
			return nil, err
		}
	}
	if err := assignShare(merged[len(base):], challengeShare, challengeCriterionFloor); err != nil {
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

// assignShare rewrites the weights in place so they sum to share: each
// criterion gets floor points, and the rest is spread by the relative weights
// (largest remainder method). The criteria are a private copy owned by
// MergeRubric. A criterion that would end below one point is an error.
func assignShare(criteria []RubricCriterion, share, floor int) error {
	distributable := share - floor*len(criteria)
	if distributable < 0 {
		return ErrWeightsCannotSum
	}
	total := 0
	for _, criterion := range criteria {
		total += criterion.Weight
	}
	remainders := make([]int, len(criteria))
	assigned := 0
	for i := range criteria {
		scaled := criteria[i].Weight * distributable
		criteria[i].Weight = scaled / total
		remainders[i] = scaled % total
		assigned += criteria[i].Weight
	}

	order := make([]int, len(criteria))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return remainders[order[a]] > remainders[order[b]] })
	for _, index := range order[:distributable-assigned] {
		criteria[index].Weight++
	}

	for i := range criteria {
		criteria[i].Weight += floor
		if criteria[i].Weight < 1 {
			return ErrWeightsCannotSum
		}
	}
	return nil
}

// MaxChallengeCriteriaFor is the most challenge criteria a generated rubric
// may have: 3 for a practice, and 3, 4 or 6 for an interview by difficulty.
// RubricVersion.Validate keeps the wider MaxChallengeCriteria limit so stored
// rubrics stay valid.
func MaxChallengeCriteriaFor(kind ChallengeKind, difficulty Difficulty) int {
	if kind == ChallengeKindPractice {
		return MaxPracticeCriteria
	}
	switch difficulty {
	case DifficultyBeginner:
		return 3
	case DifficultyIntermediate:
		return 4
	default:
		return MaxChallengeCriteria
	}
}
