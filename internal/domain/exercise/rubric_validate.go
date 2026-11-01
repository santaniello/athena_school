package exercise

import "errors"

// Size limits for rubric text, counted in characters.
const (
	MaxCriterionKeyLength         = 50
	MaxCriterionNameLength        = 100
	MaxCriterionDescriptionLength = 500
	MaxLevelDescriptorLength      = 300
	maxRubricCriteria             = 11
)

var (
	// ErrBaseCriteriaMissing is returned when a base criterion is absent.
	ErrBaseCriteriaMissing = errors.New("rubric is missing a base criterion")
	// ErrTooManyCriteria is returned when a rubric has more than 11 criteria.
	ErrTooManyCriteria = errors.New("rubric has too many criteria")
	// ErrCriterionSourceInvalid is returned for an unknown criterion source.
	ErrCriterionSourceInvalid = errors.New("rubric criterion source is invalid")
	// ErrCriterionTextRequired is returned when a key, name, description or
	// level descriptor is blank.
	ErrCriterionTextRequired = errors.New("rubric criterion text is required")
	// ErrCriterionTextTooLong is returned when a key, name, description or
	// level descriptor exceeds its limit.
	ErrCriterionTextTooLong = errors.New("rubric criterion text is too long")
)

// Validate reports the first rule the rubric breaks, each with its own
// error: every base criterion present, 1 to 6 challenge criteria, at most 11
// in total, weights of at least 1 summing to exactly 100, unique keys and
// non-empty, bounded text.
func (v RubricVersion) Validate() error {
	challengeCount, err := countBySource(v.Criteria)
	if err != nil {
		return err
	}
	if err := checkBaseCriteriaPresent(v.Criteria); err != nil {
		return err
	}
	if challengeCount < 1 || challengeCount > MaxChallengeCriteria {
		return ErrChallengeCriteriaCount
	}
	if len(v.Criteria) > maxRubricCriteria {
		return ErrTooManyCriteria
	}
	if err := checkWeights(v.Criteria); err != nil {
		return err
	}
	return checkCriteriaText(v.Criteria)
}

// countBySource returns the number of challenge criteria, or
// ErrCriterionSourceInvalid if any criterion has an unknown source.
func countBySource(criteria []RubricCriterion) (int, error) {
	challenge := 0
	for _, criterion := range criteria {
		switch criterion.Source {
		case CriterionSourceBase:
		case CriterionSourceChallenge:
			challenge++
		default:
			return 0, ErrCriterionSourceInvalid
		}
	}
	return challenge, nil
}

func checkBaseCriteriaPresent(criteria []RubricCriterion) error {
	present := make(map[string]struct{}, len(criteria))
	for _, criterion := range criteria {
		if criterion.Source == CriterionSourceBase {
			present[criterion.Key] = struct{}{}
		}
	}
	for _, base := range BaseRubric() {
		if _, ok := present[base.Key]; !ok {
			return ErrBaseCriteriaMissing
		}
	}
	return nil
}

func checkWeights(criteria []RubricCriterion) error {
	total := 0
	for _, criterion := range criteria {
		if criterion.Weight < 1 {
			return ErrWeightsCannotSum
		}
		total += criterion.Weight
	}
	if total != rubricTotalWeight {
		return ErrWeightsCannotSum
	}
	return nil
}

func checkCriteriaText(criteria []RubricCriterion) error {
	keys := make(map[string]struct{}, len(criteria))
	for _, criterion := range criteria {
		if err := validateCriterionText(criterion); err != nil {
			return err
		}
		if _, duplicate := keys[criterion.Key]; duplicate {
			return ErrCriterionKeyDuplicate
		}
		keys[criterion.Key] = struct{}{}
	}
	return nil
}

func validateCriterionText(criterion RubricCriterion) error {
	if err := validateCriterionField(criterion.Key, MaxCriterionKeyLength); err != nil {
		return err
	}
	if err := validateCriterionField(criterion.Name, MaxCriterionNameLength); err != nil {
		return err
	}
	if err := validateCriterionField(criterion.Description, MaxCriterionDescriptionLength); err != nil {
		return err
	}
	for _, level := range criterion.Levels {
		if err := validateCriterionField(level, MaxLevelDescriptorLength); err != nil {
			return err
		}
	}
	return nil
}

func validateCriterionField(value string, limit int) error {
	return validateText(value, ErrCriterionTextRequired, limit, ErrCriterionTextTooLong)
}
