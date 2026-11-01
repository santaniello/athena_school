package exercise

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// AttemptService runs one exam attempt: start, submit, discard and clean up.
// It enforces the exam rules of ADR-005 (decision 9). Scoring starts in
// 3.5, so a submitted attempt stays SUBMITTED here. See
// specs/phases/phase-03-architecture-lab/04-exercise-service.md.
type AttemptService struct {
	challenges  domainexercise.ChallengeRepository
	rubrics     domainexercise.RubricRepository
	attempts    domainexercise.AttemptRepository
	submissions domainexercise.SubmissionRepository
	links       domainexercise.SessionExerciseRepository
	sessions    SessionExists
	tx          Transactor
	clock       Clock
	newID       func() string
}

// NewAttemptService creates an AttemptService backed by the given ports.
func NewAttemptService(
	challenges domainexercise.ChallengeRepository,
	rubrics domainexercise.RubricRepository,
	attempts domainexercise.AttemptRepository,
	submissions domainexercise.SubmissionRepository,
	links domainexercise.SessionExerciseRepository,
	sessions SessionExists,
	tx Transactor,
	clock Clock,
) *AttemptService {
	return &AttemptService{
		challenges:  challenges,
		rubrics:     rubrics,
		attempts:    attempts,
		submissions: submissions,
		links:       links,
		sessions:    sessions,
		tx:          tx,
		clock:       clock,
		newID:       uuid.NewString,
	}
}

// StartInput says which challenge to start and from where.
type StartInput struct {
	ChallengeID       string
	SessionID         string // "" for a standalone attempt
	DiscardInProgress bool
}

// StartAttempt starts an exam on the challenge, frozen to its latest rubric.
func (s *AttemptService) StartAttempt(ctx context.Context, input StartInput) (AttemptView, error) {
	challenge, err := s.challenges.Get(ctx, input.ChallengeID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Latest(ctx, input.ChallengeID)
	if errors.Is(err, domainexercise.ErrRubricNotFound) {
		return AttemptView{}, ErrChallengeHasNoRubric
	}
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting latest rubric: %w", err)
	}

	linkID, err := s.sessionLinkID(ctx, input)
	if err != nil {
		return AttemptView{}, err
	}

	running, err := s.attempts.FindInProgress(ctx)
	hasRunning := err == nil
	if err != nil && !errors.Is(err, domainexercise.ErrAttemptNotFound) {
		return AttemptView{}, fmt.Errorf("exercise: looking for a running attempt: %w", err)
	}
	if hasRunning && !input.DiscardInProgress {
		return AttemptView{}, &AttemptInProgressError{AttemptID: running.ID}
	}

	attempt, err := domainexercise.NewAttempt(challenge, rubric, linkID, s.clock.Now())
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: creating attempt: %w", err)
	}
	attempt.ID = s.newID()
	if err := s.replaceAndStore(ctx, running, hasRunning, attempt); err != nil {
		return AttemptView{}, err
	}
	return viewOf(attempt, challenge, rubric, nil), nil
}

// sessionLinkID returns the id of the SessionExercise the user accepted for a
// session start, or "" for a standalone one. It never creates the link.
func (s *AttemptService) sessionLinkID(ctx context.Context, input StartInput) (string, error) {
	if input.SessionID == "" {
		return "", nil
	}
	exists, err := s.sessions.Exists(ctx, input.SessionID)
	if err != nil {
		return "", fmt.Errorf("exercise: checking session: %w", err)
	}
	if !exists {
		return "", ErrSourceSessionNotFound
	}
	link, err := s.links.GetBySessionAndChallenge(ctx, input.SessionID, input.ChallengeID)
	if err != nil {
		return "", fmt.Errorf("exercise: getting session exercise: %w", err)
	}
	return link.ID, nil
}

// replaceAndStore deletes the running attempt, if any, and stores the new one
// in a single transaction, so a failed create leaves the old attempt in place.
// Losing a race against a concurrent start surfaces as ErrAttemptInProgress.
func (s *AttemptService) replaceAndStore(ctx context.Context, running domainexercise.ChallengeAttempt, hasRunning bool, attempt domainexercise.ChallengeAttempt) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if hasRunning {
			if err := s.attempts.DeleteUnfinished(ctx, running.ID); err != nil {
				return fmt.Errorf("exercise: discarding running attempt: %w", err)
			}
		}
		return s.attempts.Create(ctx, attempt)
	})
	if errors.Is(err, domainexercise.ErrAnotherAttemptInProgress) {
		return s.currentConflict(ctx)
	}
	if err != nil {
		return fmt.Errorf("exercise: storing attempt: %w", err)
	}
	return nil
}

// currentConflict names the attempt that won a race. If it is already gone
// the conflict is still real, only its id is unknown.
func (s *AttemptService) currentConflict(ctx context.Context) error {
	winner, err := s.attempts.FindInProgress(ctx)
	if err != nil {
		return &AttemptInProgressError{}
	}
	return &AttemptInProgressError{AttemptID: winner.ID}
}

// DiscardAttempt deletes an unfinished attempt, in any state, when the user
// leaves the lab. A FINISHED attempt is refused with ErrAttemptFinished.
func (s *AttemptService) DiscardAttempt(ctx context.Context, attemptID string) error {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("exercise: getting attempt: %w", err)
	}
	if attempt.State == domainexercise.AttemptFinished {
		return ErrAttemptFinished
	}
	if err := s.attempts.DeleteUnfinished(ctx, attemptID); err != nil {
		return fmt.Errorf("exercise: discarding attempt: %w", err)
	}
	return nil
}

// DiscardStaleAttempts deletes every unfinished attempt, the ones a crash or
// a close left behind, and returns how many it removed. The caller runs it
// once at startup and logs a failure instead of stopping the app.
func (s *AttemptService) DiscardStaleAttempts(ctx context.Context) (int, error) {
	removed, err := s.attempts.DeleteAllUnfinished(ctx)
	if err != nil {
		return 0, fmt.Errorf("exercise: discarding stale attempts: %w", err)
	}
	return removed, nil
}

// submitGracePeriod is how long after the deadline a submission is still
// accepted, as TIME_EXPIRED, to absorb the UI's countdown and call latency.
const submitGracePeriod = 15 * time.Second

// SubmitAttempt ends the attempt with the given artifact. The backend decides
// from its own clock: before the deadline the reason is USER, up to the grace
// period after it TIME_EXPIRED, and later the attempt is discarded and the
// call fails with ErrSubmissionTooLate.
func (s *AttemptService) SubmitAttempt(ctx context.Context, attemptID, contentType, content string) (AttemptView, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	if attempt.State != domainexercise.AttemptInProgress {
		return AttemptView{}, domainexercise.ErrAttemptNotInProgress
	}

	now := s.clock.Now()
	if now.After(attempt.Deadline.Add(submitGracePeriod)) {
		return AttemptView{}, s.discardLate(ctx, attempt.ID)
	}
	reason := domainexercise.SubmitReasonUser
	if attempt.Expired(now) {
		reason = domainexercise.SubmitReasonTimeExpired
	}

	submitted, submission, err := attempt.Submit(contentType, content, reason, now)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: submitting attempt: %w", err)
	}
	submission.ID = s.newID()
	if err := s.attempts.Submit(ctx, submitted, submission); err != nil {
		return AttemptView{}, fmt.Errorf("exercise: storing submission: %w", err)
	}
	return s.buildView(ctx, submitted, &submission)
}

// discardLate deletes an attempt whose submission came too late and returns
// ErrSubmissionTooLate, joined with the failure if the delete did not work.
func (s *AttemptService) discardLate(ctx context.Context, attemptID string) error {
	if err := s.attempts.DeleteUnfinished(ctx, attemptID); err != nil {
		return errors.Join(ErrSubmissionTooLate, fmt.Errorf("exercise: discarding late attempt: %w", err))
	}
	return ErrSubmissionTooLate
}

// GetAttempt returns the attempt's read model, with its submission once it is
// submitted.
func (s *AttemptService) GetAttempt(ctx context.Context, attemptID string) (AttemptView, error) {
	attempt, err := s.attempts.Get(ctx, attemptID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting attempt: %w", err)
	}
	return s.viewWithSubmission(ctx, attempt)
}

// ListAttempts returns the read model of each attempt on the challenge,
// oldest first. A challenge nobody attempted yields an empty list.
func (s *AttemptService) ListAttempts(ctx context.Context, challengeID string) ([]AttemptView, error) {
	attempts, err := s.attempts.ListByChallenge(ctx, challengeID)
	if err != nil {
		return nil, fmt.Errorf("exercise: listing attempts: %w", err)
	}
	views := make([]AttemptView, 0, len(attempts))
	for _, attempt := range attempts {
		view, err := s.viewWithSubmission(ctx, attempt)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

// viewWithSubmission builds the view of a stored attempt, loading its
// submission when it has been submitted.
func (s *AttemptService) viewWithSubmission(ctx context.Context, attempt domainexercise.ChallengeAttempt) (AttemptView, error) {
	if attempt.State == domainexercise.AttemptInProgress {
		return s.buildView(ctx, attempt, nil)
	}
	submission, err := s.submissions.GetByAttempt(ctx, attempt.ID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting submission: %w", err)
	}
	return s.buildView(ctx, attempt, &submission)
}

// buildView loads what an AttemptView needs besides the attempt itself.
func (s *AttemptService) buildView(ctx context.Context, attempt domainexercise.ChallengeAttempt, submission *domainexercise.Submission) (AttemptView, error) {
	challenge, err := s.challenges.Get(ctx, attempt.ChallengeID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting challenge: %w", err)
	}
	rubric, err := s.rubrics.Get(ctx, attempt.RubricVersionID)
	if err != nil {
		return AttemptView{}, fmt.Errorf("exercise: getting rubric: %w", err)
	}
	return viewOf(attempt, challenge, rubric, submission), nil
}

func viewOf(attempt domainexercise.ChallengeAttempt, challenge domainexercise.Challenge, rubric domainexercise.RubricVersion, submission *domainexercise.Submission) AttemptView {
	return AttemptView{
		Attempt:        attempt,
		ChallengeTitle: challenge.Title,
		ChallengeTopic: challenge.Topic,
		Criteria:       rubric.Criteria,
		Submission:     submission,
	}
}
