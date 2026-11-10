package exercise

import (
	"context"
	"fmt"
	"sort"
	"strings"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// Bounds of the feedback a study session receives. See
// specs/phases/phase-03-architecture-lab/16-session-feedback.md.
const (
	// MaxFeedbackAttempts is how many finished attempts a session reports.
	MaxFeedbackAttempts = 3
	// MaxFeedbackItems caps the gaps and the recommendations of one attempt.
	MaxFeedbackItems = 3
)

// SessionFeedback is the short result of one finished attempt that was started
// from a study session: only values the evaluation already stored, never the
// diagram, the tutor's questions and answers, the justifications or the hints.
type SessionFeedback struct {
	ChallengeTitle     string
	Topic              string
	Difficulty         domainexercise.Difficulty
	Result             domainexercise.AttemptResult // PASSED or FAILED
	DiagramScore       int
	UnderstandingScore int
	PassScore          int
	UnmetMandatory     []string // names of the mandatory criteria that were not met
	Gaps               []string
	Recommendations    []string
}

// FeedbackService reports how a study session's exams went. It is read-only
// and adds no rule: the outcome is the pass policy of spec 3.6 applied to the
// stored evaluations.
type FeedbackService struct {
	links       domainexercise.SessionExerciseRepository
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	evaluations domainexercise.EvaluationRepository
}

// NewFeedbackService creates a FeedbackService backed by the given ports.
func NewFeedbackService(
	links domainexercise.SessionExerciseRepository,
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	evaluations domainexercise.EvaluationRepository,
) *FeedbackService {
	return &FeedbackService{links: links, challenges: challenges, rubrics: rubrics, attempts: attempts, evaluations: evaluations}
}

// SessionExerciseFeedback returns the session's most recent finished attempts
// (at most MaxFeedbackAttempts), newest first. An attempt belongs to the
// session through its SessionExercise; an attempt started from the catalog, a
// running one and a discarded one are never reported. A session with none
// yields an empty list.
func (s *FeedbackService) SessionExerciseFeedback(ctx context.Context, sessionID string) ([]SessionFeedback, error) {
	links, err := s.links.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing session exercises: %w", err)
	}
	var finished []domainexercise.ChallengeAttempt
	for _, link := range links {
		attempts, err := s.attempts.ListByChallenge(ctx, link.ChallengeID)
		if err != nil {
			return nil, fmt.Errorf("exercise: listing attempts: %w", err)
		}
		for _, attempt := range attempts {
			if attempt.SessionExerciseID == link.ID && attempt.State == domainexercise.AttemptFinished {
				finished = append(finished, attempt)
			}
		}
	}
	// One attempt runs at a time, so the submission time orders the sittings.
	sort.SliceStable(finished, func(i, j int) bool { return finished[i].SubmittedAt.After(finished[j].SubmittedAt) })
	if len(finished) > MaxFeedbackAttempts {
		finished = finished[:MaxFeedbackAttempts]
	}

	feedback := make([]SessionFeedback, 0, len(finished))
	for _, attempt := range finished {
		item, err := s.feedbackOf(ctx, attempt)
		if err != nil {
			return nil, err
		}
		feedback = append(feedback, item)
	}
	return feedback, nil
}

func (s *FeedbackService) feedbackOf(ctx context.Context, attempt domainexercise.ChallengeAttempt) (SessionFeedback, error) {
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return SessionFeedback{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return SessionFeedback{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	diagram, err := s.evaluations.GetByAttempt(ctx, attempt.ID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return SessionFeedback{}, fmt.Errorf("exercise: getting diagram evaluation: %w", err)
	}
	understanding, err := s.evaluations.GetByAttempt(ctx, attempt.ID, domainexercise.EvaluationKindUnderstanding)
	if err != nil {
		return SessionFeedback{}, fmt.Errorf("exercise: getting understanding evaluation: %w", err)
	}
	outcome := domainexercise.EvaluateOutcome(attempt.PassScore, rubric, &diagram, &understanding)
	return SessionFeedback{
		ChallengeTitle: challenge.Title, Topic: challenge.Topic, Difficulty: challenge.Difficulty,
		Result: outcome.Result, DiagramScore: diagram.Score, UnderstandingScore: understanding.Score,
		PassScore:       attempt.PassScore,
		UnmetMandatory:  unmetMandatoryNames(rubric, outcome),
		Gaps:            firstItems(diagram.Gaps, understanding.Gaps),
		Recommendations: firstItems(diagram.Recommendations, understanding.Recommendations),
	}, nil
}

// unmetMandatoryNames lists, once each and in the order they were found, the
// names of the mandatory criteria the outcome failed on.
func unmetMandatoryNames(rubric domainexercise.RubricVersion, outcome domainexercise.AttemptOutcome) []string {
	var names []string
	seen := map[string]bool{}
	for _, reason := range outcome.Reasons {
		if reason.Kind != domainexercise.FailureMandatoryCriterion || seen[reason.CriterionID] {
			continue
		}
		seen[reason.CriterionID] = true
		names = append(names, criterionByID(rubric, reason.CriterionID).Name)
	}
	return names
}

// firstItems takes the non-blank items of the lists in order, up to
// MaxFeedbackItems, quoting the model's wording untouched.
func firstItems(lists ...[]string) []string {
	var items []string
	for _, list := range lists {
		for _, item := range list {
			if strings.TrimSpace(item) == "" {
				continue
			}
			if len(items) == MaxFeedbackItems {
				return items
			}
			items = append(items, item)
		}
	}
	return items
}
