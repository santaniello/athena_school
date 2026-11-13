package exercise

import domainexercise "github.com/santaniello/athena/internal/domain/exercise"

// AttemptView is the read model of an attempt: the attempt, what the UI needs
// to show about its challenge, the frozen rubric's criteria and, once
// submitted, the submission.
type AttemptView struct {
	Attempt        domainexercise.ChallengeAttempt
	ChallengeTitle string
	ChallengeTopic string
	Statement      ChallengeStatement
	Criteria       []domainexercise.RubricCriterion
	Submission     *domainexercise.Submission // nil until submitted
}

// ChallengeStatement is what the exam screen shows of a challenge: the
// scenario and the four lists. Never the rubric. StarterDiagram is the diagram
// a practice attempt starts from; it is empty for the history and the result,
// which do not need it.
type ChallengeStatement struct {
	Scenario                  string
	FunctionalRequirements    []string
	NonFunctionalRequirements []string
	Constraints               []string
	Objectives                []string
	StarterDiagram            string
}
