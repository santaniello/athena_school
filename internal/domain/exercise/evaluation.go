package exercise

import "errors"

// ErrInvalidEvaluation is returned when an evaluator's output does not fit the
// attempt's frozen rubric, so nothing is stored.
var ErrInvalidEvaluation = errors.New("evaluation is invalid")

// MaxCriterionLevel is the highest level a criterion can be judged at.
const MaxCriterionLevel = LevelCount - 1

// CriterionResult is the evaluator's judgement of one criterion of the frozen
// rubric. When InsufficientEvidence is set the criterion is not judged: it is
// left out of the score and Level is ignored.
type CriterionResult struct {
	CriterionID          string
	Level                int // 0..4
	Justification        string
	Evidence             string // optional quote or reference into the submission
	InsufficientEvidence bool
}

// ComputeScore returns the score, 0 to 100, of the results against the rubric:
// the weighted sum of level/4 over the judged criteria, rescaled to 100 and
// rounded half up. Flagged criteria are left out, unless the judged ones hold
// less than half of the rubric's weight: then the full rubric is used and
// flagged criteria count as level 0, so a nearly empty diagram cannot score
// high. The model never supplies the number. See
// specs/phases/phase-03-architecture-lab/05-evaluation-engine.md.
func ComputeScore(rubric RubricVersion, results []CriterionResult) (int, error) {
	byCriterion, err := resultsByCriterion(rubric, results)
	if err != nil {
		return 0, err
	}
	var totalWeight, judgedWeight, weightedLevels int
	for _, criterion := range rubric.Criteria {
		totalWeight += criterion.Weight
		result := byCriterion[criterion.ID]
		if result.InsufficientEvidence {
			continue
		}
		judgedWeight += criterion.Weight
		weightedLevels += criterion.Weight * result.Level
	}
	if totalWeight <= 0 {
		return 0, ErrInvalidEvaluation
	}
	denominatorWeight := judgedWeight
	if 2*judgedWeight < totalWeight {
		denominatorWeight = totalWeight
	}
	// round(100 × weightedLevels / (4 × denominatorWeight)), half up, in integers.
	denominator := MaxCriterionLevel * denominatorWeight
	return (200*weightedLevels + denominator) / (2 * denominator), nil
}

// resultsByCriterion indexes the results and checks they cover exactly the
// rubric's criteria, once each, at a level from 0 to 4.
func resultsByCriterion(rubric RubricVersion, results []CriterionResult) (map[string]CriterionResult, error) {
	if len(results) != len(rubric.Criteria) {
		return nil, ErrInvalidEvaluation
	}
	byCriterion := make(map[string]CriterionResult, len(results))
	for _, result := range results {
		if _, repeated := byCriterion[result.CriterionID]; repeated {
			return nil, ErrInvalidEvaluation
		}
		if result.Level < 0 || result.Level > MaxCriterionLevel {
			return nil, ErrInvalidEvaluation
		}
		byCriterion[result.CriterionID] = result
	}
	for _, criterion := range rubric.Criteria {
		if _, ok := byCriterion[criterion.ID]; !ok {
			return nil, ErrInvalidEvaluation
		}
	}
	return byCriterion, nil
}
