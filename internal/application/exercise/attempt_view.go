package exercise

import domainexercise "github.com/santaniello/athena/internal/domain/exercise"

// AttemptView is the read model of an attempt: the attempt, what the UI needs
// to show about its challenge, the frozen rubric's criteria and, once
// submitted, the submission. The challenge statement is read through
// ChallengeService, not copied here.
type AttemptView struct {
	Attempt        domainexercise.ChallengeAttempt
	ChallengeTitle string
	ChallengeTopic string
	Criteria       []domainexercise.RubricCriterion
	Submission     *domainexercise.Submission // nil until submitted
}
