package exercise

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// hintTimeout bounds one model call, so a hung provider cannot hold the
// attempt's single request slot for the provider's own, much longer, timeout.
// The call never outlives the exam either.
const hintTimeout = 60 * time.Second

// HintService gives the tutor's hints during an exam: at most
// domainexercise.MaxHints per attempt, one request at a time. A hint is spent
// only when delivered. See
// specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
type HintService struct {
	challenges domainexercise.ChallengeRepository
	rubrics    domainexercise.RubricRepository
	attempts   domainexercise.AttemptRepository
	hints      domainexercise.HintRepository
	generator  domainexercise.HintGenerator
	clock      Clock
	newID      func() string

	mu      sync.Mutex
	running map[string]struct{} // attempt ids with a request in flight
}

// NewHintService creates a HintService backed by the given ports.
func NewHintService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	hints domainexercise.HintRepository,
	generator domainexercise.HintGenerator,
	clock Clock,
) *HintService {
	return &HintService{
		challenges: challenges,
		rubrics:    rubrics,
		attempts:   attempts,
		hints:      hints,
		generator:  generator,
		clock:      clock,
		newID:      uuid.NewString,
		running:    map[string]struct{}{},
	}
}

// RequestHint asks the tutor for the next hint, given the current board. The
// attempt must be IN_PROGRESS and before its deadline (else
// domainexercise.ErrAttemptNotInProgress). It returns domainexercise.ErrNoHintsLeft
// once MaxHints were given, without calling the model, and
// domainexercise.ErrHintInProgress while another request for the attempt runs.
// Nothing is stored unless the model's hint is delivered, so a failed or
// cancelled call can be repeated.
func (s *HintService) RequestHint(ctx context.Context, attemptID, boardXML string) (domainexercise.Hint, error) {
	if !s.begin(attemptID) {
		return domainexercise.Hint{}, domainexercise.ErrHintInProgress
	}
	defer s.end(attemptID)

	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return domainexercise.Hint{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	if attempt.State != domainexercise.AttemptInProgress || attempt.Expired(s.clock.Now()) {
		return domainexercise.Hint{}, domainexercise.ErrAttemptNotInProgress
	}
	previous, err := s.hints.ListByAttempt(ctx, attemptID)
	if err != nil {
		return domainexercise.Hint{}, fmt.Errorf("exercise: listing hints: %w", err)
	}
	if len(previous) >= domainexercise.MaxHints {
		return domainexercise.Hint{}, domainexercise.ErrNoHintsLeft
	}
	if len(boardXML) > domainexercise.MaxSubmissionContentBytes {
		return domainexercise.Hint{}, domainexercise.ErrSubmissionTooLarge
	}
	input, err := s.hintInput(ctx, attempt, boardXML, previous)
	if err != nil {
		return domainexercise.Hint{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, min(hintTimeout, attempt.Deadline.Sub(s.clock.Now())))
	defer cancel()
	content, err := s.generator.GenerateHint(callCtx, input)
	if err != nil {
		return domainexercise.Hint{}, fmt.Errorf("exercise: generating hint: %w", err)
	}
	hint := domainexercise.Hint{
		ID:        s.newID(),
		AttemptID: attemptID,
		Position:  len(previous) + 1,
		Content:   strings.TrimSpace(content),
		CreatedAt: s.clock.Now(),
	}
	if err := s.hints.Add(ctx, hint); err != nil {
		return domainexercise.Hint{}, fmt.Errorf("exercise: storing hint: %w", err)
	}
	return hint, nil
}

// begin marks a request as running for the attempt. It reports false when one
// already is.
func (s *HintService) begin(attemptID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, busy := s.running[attemptID]; busy {
		return false
	}
	s.running[attemptID] = struct{}{}
	return true
}

func (s *HintService) end(attemptID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, attemptID)
}

func (s *HintService) hintInput(ctx context.Context, attempt domainexercise.ChallengeAttempt, boardXML string, previous []domainexercise.Hint) (domainexercise.HintInput, error) {
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return domainexercise.HintInput{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return domainexercise.HintInput{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	return domainexercise.HintInput{Challenge: challenge, Rubric: rubric, BoardXML: boardXML, Previous: previous}, nil
}

// ListHints returns the attempt's hints in the order given. A finished
// attempt keeps them, read-only.
func (s *HintService) ListHints(ctx context.Context, attemptID string) ([]domainexercise.Hint, error) {
	if _, err := s.attempts.Get(ctx, attemptID); err != nil {
		return nil, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	hints, err := s.hints.ListByAttempt(ctx, attemptID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing hints: %w", err)
	}
	return hints, nil
}
