package exercise

import (
	"errors"
	"time"
)

// ErrInvalidPassScore is returned for a minimum passing score outside
// MinPassScore..MaxPassScore.
var ErrInvalidPassScore = errors.New("minimum passing score must be between 50 and 100")

// The bounds and default of the minimum passing score, and the level every
// mandatory criterion must reach. The level is fixed in code, not a setting.
const (
	MinPassScore     = 50
	MaxPassScore     = 100
	DefaultPassScore = 80

	MandatoryMinLevel = 2
)

// ValidatePassScore checks that the minimum passing score is within bounds.
func ValidatePassScore(score int) error {
	if score < MinPassScore || score > MaxPassScore {
		return ErrInvalidPassScore
	}
	return nil
}

// AttemptResult is whether an attempt passed.
type AttemptResult string

// The results of an attempt. PENDING exists only inside a running sitting and
// is never stored.
const (
	ResultPending AttemptResult = "PENDING"
	ResultPassed  AttemptResult = "PASSED"
	ResultFailed  AttemptResult = "FAILED"
)

// FailureKind says which rule a failed attempt broke.
type FailureKind string

// The rules an attempt can fail.
const (
	FailureDiagramScore       FailureKind = "DIAGRAM_SCORE"
	FailureUnderstandingScore FailureKind = "UNDERSTANDING_SCORE"
	FailureMandatoryCriterion FailureKind = "MANDATORY_CRITERION"
)

// FailureReason is one reason an attempt failed.
type FailureReason struct {
	Kind        FailureKind
	Evaluation  EvaluationKind
	CriterionID string // set for FailureMandatoryCriterion
}

// AttemptOutcome is the result of an attempt and, when it failed, every
// reason why.
type AttemptOutcome struct {
	Result  AttemptResult
	Reasons []FailureReason
}

// EvaluateOutcome applies the pass policy of spec 3.6 to the two evaluations
// of an attempt. It is PENDING while either is missing. Otherwise both scores
// must reach passScore on their own (never averaged) and every mandatory
// criterion must reach MandatoryMinLevel in both evaluations. A criterion
// flagged InsufficientEvidence in one evaluation is judged in the other; flagged
// in both it counts as not met. A failed outcome lists every reason.
func EvaluateOutcome(passScore int, rubric RubricVersion, diagram, understanding *Evaluation) AttemptOutcome {
	if diagram == nil || understanding == nil {
		return AttemptOutcome{Result: ResultPending}
	}
	var reasons []FailureReason
	if diagram.Score < passScore {
		reasons = append(reasons, FailureReason{Kind: FailureDiagramScore, Evaluation: diagram.Kind})
	}
	if understanding.Score < passScore {
		reasons = append(reasons, FailureReason{Kind: FailureUnderstandingScore, Evaluation: understanding.Kind})
	}
	reasons = append(reasons, mandatoryShortfalls(rubric, diagram, understanding)...)
	reasons = append(reasons, mandatoryShortfalls(rubric, understanding, diagram)...)
	if len(reasons) > 0 {
		return AttemptOutcome{Result: ResultFailed, Reasons: reasons}
	}
	return AttemptOutcome{Result: ResultPassed}
}

// mandatoryShortfalls lists the mandatory criteria of the rubric that evaluation
// does not meet, given the other evaluation of the same attempt.
func mandatoryShortfalls(rubric RubricVersion, evaluation, other *Evaluation) []FailureReason {
	results := indexResults(evaluation)
	otherResults := indexResults(other)
	var reasons []FailureReason
	for _, criterion := range rubric.Criteria {
		if !criterion.Mandatory {
			continue
		}
		result := results[criterion.ID]
		if result.InsufficientEvidence && !otherResults[criterion.ID].InsufficientEvidence {
			continue
		}
		if result.InsufficientEvidence || result.Level < MandatoryMinLevel {
			reasons = append(reasons, FailureReason{
				Kind: FailureMandatoryCriterion, Evaluation: evaluation.Kind, CriterionID: criterion.ID,
			})
		}
	}
	return reasons
}

func indexResults(evaluation *Evaluation) map[string]CriterionResult {
	byCriterion := make(map[string]CriterionResult, len(evaluation.Results))
	for _, result := range evaluation.Results {
		byCriterion[result.CriterionID] = result
	}
	return byCriterion
}

// ProgressStatus is where a challenge stands for the user. It is derived from
// the challenge's attempts and never stored.
type ProgressStatus string

// The progress statuses of a challenge.
const (
	ProgressNotStarted       ProgressStatus = "NOT_STARTED"
	ProgressInProgress       ProgressStatus = "IN_PROGRESS"
	ProgressNeedsImprovement ProgressStatus = "NEEDS_IMPROVEMENT"
	ProgressCompleted        ProgressStatus = "COMPLETED"
)

// AttemptSummary is what DeriveProgress needs of one attempt: whether it is
// still running, its outcome, its diagram score (0 when not evaluated) and
// when it was submitted (zero while running).
type AttemptSummary struct {
	Running      bool
	Outcome      AttemptOutcome
	DiagramScore int
	SubmittedAt  time.Time
}

// ChallengeProgress is the derived progress of one challenge. BestDiagramScore
// is 0 when no attempt was evaluated and FirstPassedAt is zero when none passed.
type ChallengeProgress struct {
	Status           ProgressStatus
	Attempts         int
	BestDiagramScore int
	FirstPassedAt    time.Time
}

// DeriveProgress computes the progress of a challenge from its attempts. One
// passed attempt completes it for good: a later failure or a new attempt in
// progress never removes the completion. FirstPassedAt is the submission time
// of the earliest passed attempt.
func DeriveProgress(attempts []AttemptSummary) ChallengeProgress {
	progress := ChallengeProgress{Attempts: len(attempts)}
	var passed, pending, failed bool
	for _, attempt := range attempts {
		progress.BestDiagramScore = max(progress.BestDiagramScore, attempt.DiagramScore)
		switch {
		case attempt.Outcome.Result == ResultPassed:
			if !passed || attempt.SubmittedAt.Before(progress.FirstPassedAt) {
				progress.FirstPassedAt = attempt.SubmittedAt
			}
			passed = true
		case attempt.Running || attempt.Outcome.Result == ResultPending:
			pending = true
		default:
			failed = true
		}
	}
	progress.Status = progressStatus(passed, pending, failed)
	return progress
}

func progressStatus(passed, pending, failed bool) ProgressStatus {
	switch {
	case passed:
		return ProgressCompleted
	case pending:
		return ProgressInProgress
	case failed:
		return ProgressNeedsImprovement
	default:
		return ProgressNotStarted
	}
}
