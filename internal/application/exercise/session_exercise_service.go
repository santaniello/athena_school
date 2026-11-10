package exercise

import (
	"context"
	"fmt"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// SessionExerciseCard is the chat card of an accepted exercise: the challenge
// without its scenario, requirements or rubric, how the user is doing on it and
// where it hangs in the conversation.
type SessionExerciseCard struct {
	SessionExerciseID string
	MessageID         string
	ChallengeID       string
	Title             string
	Difficulty        domainexercise.Difficulty
	TimeLimitMinutes  int
	Status            domainexercise.ProgressStatus
	Attempts          int
	Latest            *LatestResult // nil when no finished attempt
}

// LatestResult is the outcome of the most recent finished attempt.
type LatestResult struct {
	Outcome            domainexercise.AttemptResult // PASSED or FAILED
	DiagramScore       int
	UnderstandingScore int
}

// SessionExerciseService reads the exercise cards of a study session. It is
// read-only. See specs/phases/phase-03-architecture-lab/17-session-exercises-ui.md.
type SessionExerciseService struct {
	links      domainexercise.SessionExerciseRepository
	challenges domainexercise.ChallengeRepository
	records    domainexercise.AttemptRecordRepository
}

// NewSessionExerciseService creates a SessionExerciseService backed by the given ports.
func NewSessionExerciseService(
	links domainexercise.SessionExerciseRepository,
	challenges domainexercise.ChallengeRepository,
	records domainexercise.AttemptRecordRepository,
) *SessionExerciseService {
	return &SessionExerciseService{links: links, challenges: challenges, records: records}
}

// ListSessionExercises returns one card per link of the session, in link
// order. Progress and counts come from every finished attempt of the
// challenge, not only those started from this session, so a card agrees with
// the catalog.
func (s *SessionExerciseService) ListSessionExercises(ctx context.Context, sessionID string) ([]SessionExerciseCard, error) {
	links, err := s.links.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing session exercises: %w", err)
	}
	if len(links) == 0 {
		return []SessionExerciseCard{}, nil
	}
	challenges := make([]domainexercise.Challenge, len(links))
	ids := make([]string, len(links))
	for i, link := range links {
		if challenges[i], err = s.challenges.Get(ctx, link.ChallengeID); err != nil {
			return nil, fmt.Errorf("exercise: getting challenge: %w", err)
		}
		ids[i] = link.ChallengeID
	}
	records, err := s.records.ListByChallenges(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("exercise: reading attempts of challenges: %w", err)
	}
	cards := make([]SessionExerciseCard, len(links))
	for i, link := range links {
		cards[i], err = cardOf(link, challenges[i], records[link.ChallengeID])
		if err != nil {
			return nil, err
		}
	}
	return cards, nil
}

// cardOf derives the status from every attempt of the challenge, as the
// catalog does, so the two always agree; the count and the latest result only
// concern the finished ones.
func cardOf(link domainexercise.SessionExercise, challenge domainexercise.Challenge, records []domainexercise.AttemptRecord) (SessionExerciseCard, error) {
	summaries := make([]domainexercise.AttemptSummary, len(records))
	var finished []domainexercise.AttemptRecord
	var latest domainexercise.AttemptRecord
	var latestOutcome domainexercise.AttemptResult
	for i, record := range records {
		summary, err := summarizeRecord(record)
		if err != nil {
			return SessionExerciseCard{}, err
		}
		summaries[i] = summary
		// A finished attempt always has both evaluations; one without is not a result.
		if record.Attempt.State != domainexercise.AttemptFinished || record.Diagram == nil || record.Understanding == nil {
			continue
		}
		finished = append(finished, record)
		if !record.Attempt.SubmittedAt.Before(latest.Attempt.SubmittedAt) {
			latest, latestOutcome = record, summary.Outcome.Result
		}
	}
	card := SessionExerciseCard{
		SessionExerciseID: link.ID, MessageID: link.MessageID, ChallengeID: challenge.ID, Title: challenge.Title,
		Difficulty: challenge.Difficulty, TimeLimitMinutes: challenge.TimeLimitMinutes,
		Status: domainexercise.DeriveProgress(summaries).Status, Attempts: len(finished),
	}
	if len(finished) > 0 {
		card.Latest = &LatestResult{
			Outcome: latestOutcome, DiagramScore: latest.Diagram.Score, UnderstandingScore: latest.Understanding.Score,
		}
	}
	return card, nil
}
