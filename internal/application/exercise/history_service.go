package exercise

import (
	"context"
	"fmt"
	"sort"
	"time"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// AttemptListItem is one finished attempt in a challenge's history.
type AttemptListItem struct {
	AttemptID          string
	FinishedAt         time.Time
	DiagramScore       int
	UnderstandingScore int
	Outcome            domainexercise.AttemptResult // PASSED or FAILED
}

// ChallengeSnapshot is the challenge as the attempt answered it: its facts and
// the statement. A challenge is immutable, so it is the same one forever.
type ChallengeSnapshot struct {
	ID         string
	Title      string
	Topic      string
	Difficulty domainexercise.Difficulty
	Statement  ChallengeStatement
}

// CriterionRow is one criterion's result joined with the frozen rubric's
// criterion, so the screen needs no second lookup.
type CriterionRow struct {
	Criterion domainexercise.RubricCriterion
	Result    domainexercise.CriterionResult
}

// EvaluationView is an evaluation with a row per criterion, in rubric order.
type EvaluationView struct {
	Evaluation domainexercise.Evaluation
	Rows       []CriterionRow
}

// AttemptResultView is everything the result screen shows of a finished
// attempt.
type AttemptResultView struct {
	Challenge     ChallengeSnapshot
	Attempt       domainexercise.ChallengeAttempt
	FinishedAt    time.Time
	TimeSpent     time.Duration // from start to submission
	HintsUsed     int
	PassScore     int // frozen for the attempt
	Outcome       domainexercise.AttemptOutcome
	Criteria      []domainexercise.RubricCriterion // the frozen rubric's, to name the outcome's reasons
	Diagram       EvaluationView
	Understanding EvaluationView
	Defense       []domainexercise.DefenseTurn
	Submission    string // the stored diagram XML
}

// HistoryService reads finished attempts: the list of a challenge's attempts
// and the full result of one. It is read-only and adds no rule. See
// specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
type HistoryService struct {
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	submissions domainexercise.SubmissionRepository
	evaluations domainexercise.EvaluationRepository
	defense     domainexercise.DefenseRepository
	hints       domainexercise.HintRepository
}

// NewHistoryService creates a HistoryService backed by the given ports.
func NewHistoryService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	submissions domainexercise.SubmissionRepository,
	evaluations domainexercise.EvaluationRepository,
	defense domainexercise.DefenseRepository,
	hints domainexercise.HintRepository,
) *HistoryService {
	return &HistoryService{
		challenges: challenges, rubrics: rubrics, attempts: attempts, submissions: submissions,
		evaluations: evaluations, defense: defense, hints: hints,
	}
}

// finishedEvaluations is what a FINISHED attempt's outcome is computed from.
type finishedEvaluations struct {
	rubric        domainexercise.RubricVersion
	diagram       domainexercise.Evaluation
	understanding domainexercise.Evaluation
}

func (e finishedEvaluations) outcome(attempt domainexercise.ChallengeAttempt) domainexercise.AttemptOutcome {
	return domainexercise.EvaluateOutcome(attempt.PassScore, e.rubric, &e.diagram, &e.understanding)
}

// ListHistory returns the challenge's finished attempts, newest first.
// Attempts that are still running or being evaluated are not history.
func (s *HistoryService) ListHistory(ctx context.Context, challengeID string) ([]AttemptListItem, error) {
	attempts, err := s.attempts.ListByChallenge(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing attempts: %w", err)
	}
	items := []AttemptListItem{}
	for _, attempt := range attempts {
		if attempt.State != domainexercise.AttemptFinished {
			continue
		}
		evaluations, err := s.loadEvaluations(ctx, attempt)
		if err != nil {
			return nil, err
		}
		items = append(items, AttemptListItem{
			AttemptID: attempt.ID, FinishedAt: evaluations.understanding.CreatedAt,
			DiagramScore: evaluations.diagram.Score, UnderstandingScore: evaluations.understanding.Score,
			Outcome: evaluations.outcome(attempt).Result,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].FinishedAt.After(items[j].FinishedAt) })
	return items, nil
}

func (s *HistoryService) loadEvaluations(ctx context.Context, attempt domainexercise.ChallengeAttempt) (finishedEvaluations, error) {
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return finishedEvaluations{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	diagram, err := s.evaluations.GetByAttempt(ctx, attempt.ID, domainexercise.EvaluationKindDiagram)
	if err != nil {
		return finishedEvaluations{}, fmt.Errorf("exercise: getting diagram evaluation: %w", err)
	}
	understanding, err := s.evaluations.GetByAttempt(ctx, attempt.ID, domainexercise.EvaluationKindUnderstanding)
	if err != nil {
		return finishedEvaluations{}, fmt.Errorf("exercise: getting understanding evaluation: %w", err)
	}
	return finishedEvaluations{rubric: rubric, diagram: diagram, understanding: understanding}, nil
}

// GetAttemptResult returns the full result of a FINISHED attempt. An unknown
// id is domainexercise.ErrAttemptNotFound and an attempt in any other state is
// ErrAttemptNotFinished.
func (s *HistoryService) GetAttemptResult(ctx context.Context, attemptID string) (AttemptResultView, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return AttemptResultView{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	if attempt.State != domainexercise.AttemptFinished {
		return AttemptResultView{}, ErrAttemptNotFinished
	}
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return AttemptResultView{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	evaluations, err := s.loadEvaluations(ctx, attempt)
	if err != nil {
		return AttemptResultView{}, err
	}
	turns, err := s.defense.ListByAttempt(ctx, attempt.ID)
	if err != nil {
		return AttemptResultView{}, fmt.Errorf("exercise: listing defense turns: %w", err)
	}
	hints, err := s.hints.ListByAttempt(ctx, attempt.ID)
	if err != nil {
		return AttemptResultView{}, fmt.Errorf("exercise: listing hints: %w", err)
	}
	submission, err := s.submissions.GetByAttempt(ctx, attempt.ID)
	if err != nil {
		return AttemptResultView{}, fmt.Errorf("exercise: getting submission: %w", err)
	}
	return AttemptResultView{
		Challenge:     snapshotOf(challenge),
		Attempt:       attempt,
		FinishedAt:    evaluations.understanding.CreatedAt,
		TimeSpent:     attempt.SubmittedAt.Sub(attempt.StartedAt),
		HintsUsed:     len(hints),
		PassScore:     attempt.PassScore,
		Outcome:       evaluations.outcome(attempt),
		Criteria:      evaluations.rubric.Criteria,
		Diagram:       evaluationViewOf(evaluations.diagram, evaluations.rubric),
		Understanding: evaluationViewOf(evaluations.understanding, evaluations.rubric),
		Defense:       turns,
		Submission:    submission.Content,
	}, nil
}

func snapshotOf(challenge domainexercise.Challenge) ChallengeSnapshot {
	return ChallengeSnapshot{
		ID: challenge.ID, Title: challenge.Title, Topic: challenge.Topic, Difficulty: challenge.Difficulty,
		Statement: ChallengeStatement{
			Scenario:                  challenge.Scenario,
			FunctionalRequirements:    challenge.FunctionalRequirements,
			NonFunctionalRequirements: challenge.NonFunctionalRequirements,
			Constraints:               challenge.Constraints,
			Objectives:                challenge.Objectives,
		},
	}
}

func evaluationViewOf(evaluation domainexercise.Evaluation, rubric domainexercise.RubricVersion) EvaluationView {
	rows := make([]CriterionRow, 0, len(evaluation.Results))
	for _, result := range evaluation.Results {
		rows = append(rows, CriterionRow{Criterion: criterionByID(rubric, result.CriterionID), Result: result})
	}
	return EvaluationView{Evaluation: evaluation, Rows: rows}
}
