// Package progress holds the rules of the Progress dashboard: the score and
// the status of a topic, which is a study session, derived from the quizzes,
// Arch Lab attempts and concepts that belong to it. Everything here is a pure
// function over its own input structs, so the package imports neither study,
// quiz nor exercise (ADR-001): the application layer maps what it reads into
// these inputs, including the already computed pass outcome of a lab attempt.
// See specs/phases/phase-05-progress/01-progress-dashboard.md.
package progress

import "time"

// Status is the qualitative state of a topic.
type Status string

// The statuses a topic can have, checked in this order; the first match wins.
const (
	StatusInsufficientEvidence Status = "INSUFFICIENT_EVIDENCE"
	StatusNeedsReinforcement   Status = "NEEDS_REINFORCEMENT"
	StatusImproving            Status = "IMPROVING"
	StatusStable               Status = "STABLE"
)

// ImprovementStep is how many points the latest attempt of an item must be
// above the one before it for the topic to count as improving.
const ImprovementStep = 10

// ScoredAttempt is one finished quiz attempt.
type ScoredAttempt struct {
	Percent int // 0..100
	At      time.Time
}

// QuizInput is one quiz of the topic with its attempts, oldest first.
type QuizInput struct {
	QuizID   string
	Attempts []ScoredAttempt
}

// LabAttempt is one Arch Lab attempt that has both its diagram and its defense
// evaluations.
type LabAttempt struct {
	DiagramScore       int
	UnderstandingScore int
	// Failed is the attempt's FAILED outcome, computed by the application
	// with the Arch Lab's own pass rule.
	Failed bool
	At     time.Time
}

// labScore is the mean of the diagram and the defense scores, rounded half up.
func (a LabAttempt) labScore() int {
	return roundedMean(a.DiagramScore+a.UnderstandingScore, 2)
}

// ChallengeInput is one Arch Lab challenge of the topic with its attempts
// that have both evaluations, oldest first.
type ChallengeInput struct {
	ChallengeID string
	Title       string // names the challenge in the evolution chart
	Attempts    []LabAttempt
}

// ConceptInput is what the tutor recorded about one concept of the session.
type ConceptInput struct {
	NeedsReview   bool
	EvidenceCount int
	UpdatedAt     time.Time
}

// TopicInput is everything the rules need to know about one topic.
type TopicInput struct {
	SessionID  string
	Name       string
	StartedAt  time.Time
	Quizzes    []QuizInput
	Challenges []ChallengeInput
	Concepts   []ConceptInput
}

// TopicSummary is the derived state of a topic. A nil score means there is
// nothing to score yet.
type TopicSummary struct {
	SessionID    string
	Name         string
	QuizScore    *int
	LabScore     *int
	Status       Status
	LastActivity time.Time
}

// SummarizeTopic derives the scores, the status and the last activity of a
// topic. passScore is the minimum passing score in force.
func SummarizeTopic(in TopicInput, passScore int) TopicSummary {
	quizScore := quizScore(in.Quizzes)
	return TopicSummary{
		SessionID:    in.SessionID,
		Name:         in.Name,
		QuizScore:    quizScore,
		LabScore:     labScore(in.Challenges),
		Status:       statusOf(in, quizScore, passScore),
		LastActivity: lastActivity(in),
	}
}

// quizScore is the mean of the latest attempt of each quiz that has one, or nil.
func quizScore(quizzes []QuizInput) *int {
	var sum, count int
	for _, q := range quizzes {
		if len(q.Attempts) == 0 {
			continue
		}
		sum += q.Attempts[len(q.Attempts)-1].Percent
		count++
	}
	return meanOrNil(sum, count)
}

// labScore is the mean of the lab score of the latest attempt of each
// challenge that has one, or nil.
func labScore(challenges []ChallengeInput) *int {
	var sum, count int
	for _, c := range challenges {
		if len(c.Attempts) == 0 {
			continue
		}
		sum += c.Attempts[len(c.Attempts)-1].labScore()
		count++
	}
	return meanOrNil(sum, count)
}

func meanOrNil(sum, count int) *int {
	if count == 0 {
		return nil
	}
	mean := roundedMean(sum, count)
	return &mean
}

// roundedMean is sum/count rounded half up, for non-negative values.
func roundedMean(sum, count int) int {
	return (2*sum + count) / (2 * count)
}

func statusOf(in TopicInput, quizScore *int, passScore int) Status {
	switch {
	case !hasEvidence(in):
		return StatusInsufficientEvidence
	case needsReinforcement(in, quizScore, passScore):
		return StatusNeedsReinforcement
	case isImproving(in):
		return StatusImproving
	default:
		return StatusStable
	}
}

func hasEvidence(in TopicInput) bool {
	for _, q := range in.Quizzes {
		if len(q.Attempts) > 0 {
			return true
		}
	}
	for _, c := range in.Challenges {
		if len(c.Attempts) > 0 {
			return true
		}
	}
	for _, concept := range in.Concepts {
		if concept.EvidenceCount > 0 {
			return true
		}
	}
	return false
}

func needsReinforcement(in TopicInput, quizScore *int, passScore int) bool {
	if quizScore != nil && *quizScore < passScore {
		return true
	}
	for _, c := range in.Challenges {
		if len(c.Attempts) > 0 && c.Attempts[len(c.Attempts)-1].Failed {
			return true
		}
	}
	for _, concept := range in.Concepts {
		if concept.NeedsReview {
			return true
		}
	}
	return false
}

// isImproving reports whether some quiz or challenge rose by at least
// ImprovementStep points between its last two attempts. Only attempts of the
// same item are compared.
func isImproving(in TopicInput) bool {
	for _, q := range in.Quizzes {
		percents := make([]int, len(q.Attempts))
		for i, attempt := range q.Attempts {
			percents[i] = attempt.Percent
		}
		if rose(percents) {
			return true
		}
	}
	for _, c := range in.Challenges {
		scores := make([]int, len(c.Attempts))
		for i, attempt := range c.Attempts {
			scores[i] = attempt.labScore()
		}
		if rose(scores) {
			return true
		}
	}
	return false
}

// rose reports whether the last score is at least ImprovementStep above the
// one before it.
func rose(scores []int) bool {
	n := len(scores)
	return n >= 2 && scores[n-1]-scores[n-2] >= ImprovementStep
}

// lastActivity is the latest of the session start, every attempt and every
// concept update.
func lastActivity(in TopicInput) time.Time {
	latest := in.StartedAt
	consider := func(t time.Time) {
		if t.After(latest) {
			latest = t
		}
	}
	for _, q := range in.Quizzes {
		for _, attempt := range q.Attempts {
			consider(attempt.At)
		}
	}
	for _, c := range in.Challenges {
		for _, attempt := range c.Attempts {
			consider(attempt.At)
		}
	}
	for _, concept := range in.Concepts {
		consider(concept.UpdatedAt)
	}
	return latest
}
