package desktop

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// WithAttemptService wires the exam attempt use cases (spec 3.4).
func WithAttemptService(service *applicationexercise.AttemptService) Option {
	return func(a *App) { a.exercise = service }
}

// Stable ExerciseError.Code values the frontend switches on.
const (
	exerciseCodeInvalidInput         = "invalid_input"
	exerciseCodeChallengeNotFound    = "challenge_not_found"
	exerciseCodeChallengeHasNoRubric = "challenge_has_no_rubric"
	exerciseCodeAttemptInProgress    = "attempt_in_progress"
	exerciseCodeSessionNotFound      = "source_session_not_found"
	exerciseCodeSessionNotLinked     = "session_exercise_not_found"
	exerciseCodeAttemptNotFound      = "attempt_not_found"
	exerciseCodeAttemptNotInProgress = "attempt_not_in_progress"
	exerciseCodeAttemptFinished      = "attempt_finished"
	exerciseCodeSubmissionTooLate    = "submission_too_late"
	exerciseCodeSubmissionInvalid    = "submission_invalid"
	exerciseCodeInternal             = "internal"
)

// ExerciseError is the error the exercise bindings return. Wails hands the
// frontend only the error's text, so Error() is this struct as JSON:
// {"code": "...", "message": "...", "attemptID": "..."}. AttemptID is set only
// for attempt_in_progress and names the running attempt.
type ExerciseError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	AttemptID string `json:"attemptId,omitempty"`
}

func (e *ExerciseError) Error() string {
	encoded, err := json.Marshal(e)
	if err != nil {
		return e.Code
	}
	return string(encoded)
}

// toExerciseError maps a use case error to its coded form. Anything unknown
// is logged with its detail and reported as a generic internal error.
func toExerciseError(err error) *ExerciseError {
	var running *applicationexercise.AttemptInProgressError
	switch {
	case errors.As(err, &running):
		return &ExerciseError{Code: exerciseCodeAttemptInProgress, Message: "an attempt is already in progress", AttemptID: running.AttemptID}
	case errors.Is(err, domainexercise.ErrChallengeNotFound):
		return &ExerciseError{Code: exerciseCodeChallengeNotFound, Message: "challenge not found"}
	case errors.Is(err, applicationexercise.ErrChallengeHasNoRubric):
		return &ExerciseError{Code: exerciseCodeChallengeHasNoRubric, Message: "the challenge has no rubric yet"}
	case errors.Is(err, applicationexercise.ErrSourceSessionNotFound):
		return &ExerciseError{Code: exerciseCodeSessionNotFound, Message: "study session not found"}
	case errors.Is(err, domainexercise.ErrSessionExerciseNotFound):
		return &ExerciseError{Code: exerciseCodeSessionNotLinked, Message: "the session was not offered this challenge"}
	case errors.Is(err, domainexercise.ErrAttemptNotFound):
		return &ExerciseError{Code: exerciseCodeAttemptNotFound, Message: "attempt not found"}
	case errors.Is(err, domainexercise.ErrAttemptNotInProgress):
		return &ExerciseError{Code: exerciseCodeAttemptNotInProgress, Message: "the attempt is not in progress"}
	case errors.Is(err, applicationexercise.ErrAttemptFinished):
		return &ExerciseError{Code: exerciseCodeAttemptFinished, Message: "a finished attempt cannot be discarded"}
	case errors.Is(err, applicationexercise.ErrSubmissionTooLate):
		return &ExerciseError{Code: exerciseCodeSubmissionTooLate, Message: "the submission arrived after the time limit and the attempt was discarded"}
	case isInvalidSubmission(err):
		return &ExerciseError{Code: exerciseCodeSubmissionInvalid, Message: err.Error()}
	}
	log.Printf("exercise binding failed: %v", err)
	return &ExerciseError{Code: exerciseCodeInternal, Message: "something went wrong"}
}

func isInvalidSubmission(err error) bool {
	return errors.Is(err, domainexercise.ErrSubmissionContentRequired) ||
		errors.Is(err, domainexercise.ErrSubmissionContentTypeRequired) ||
		errors.Is(err, domainexercise.ErrSubmissionTooLarge)
}

func invalidInput(message string) *ExerciseError {
	return &ExerciseError{Code: exerciseCodeInvalidInput, Message: message}
}

// CriterionDTO is one rubric criterion shown to the user during an attempt.
type CriterionDTO struct {
	Key         string   `json:"key"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Weight      int      `json:"weight"`
	Mandatory   bool     `json:"mandatory"`
	Levels      []string `json:"levels"`
}

// SubmissionDTO is the artifact an attempt ended with.
type SubmissionDTO struct {
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
	Checksum    string `json:"checksum"`
	SubmittedAt string `json:"submittedAt"`
}

// AttemptDTO is the desktop-facing read model of an exam attempt. Times are
// RFC 3339; SubmittedAt, SubmitReason and Submission are empty/null until the
// attempt is submitted.
type AttemptDTO struct {
	ID                string         `json:"id"`
	ChallengeID       string         `json:"challengeId"`
	ChallengeTitle    string         `json:"challengeTitle"`
	ChallengeTopic    string         `json:"challengeTopic"`
	SessionExerciseID string         `json:"sessionExerciseId"`
	State             string         `json:"state"`
	StartedAt         string         `json:"startedAt"`
	Deadline          string         `json:"deadline"`
	SubmittedAt       string         `json:"submittedAt"`
	SubmitReason      string         `json:"submitReason"`
	Criteria          []CriterionDTO `json:"criteria"`
	Submission        *SubmissionDTO `json:"submission"`
}

func toAttemptDTO(view applicationexercise.AttemptView) AttemptDTO {
	attempt := view.Attempt
	dto := AttemptDTO{
		ID:                attempt.ID,
		ChallengeID:       attempt.ChallengeID,
		ChallengeTitle:    view.ChallengeTitle,
		ChallengeTopic:    view.ChallengeTopic,
		SessionExerciseID: attempt.SessionExerciseID,
		State:             string(attempt.State),
		StartedAt:         attempt.StartedAt.Format(time.RFC3339),
		Deadline:          attempt.Deadline.Format(time.RFC3339),
		SubmitReason:      string(attempt.SubmitReason),
		Criteria:          make([]CriterionDTO, 0, len(view.Criteria)),
	}
	if !attempt.SubmittedAt.IsZero() {
		dto.SubmittedAt = attempt.SubmittedAt.Format(time.RFC3339)
	}
	for _, criterion := range view.Criteria {
		dto.Criteria = append(dto.Criteria, CriterionDTO{
			Key: criterion.Key, Name: criterion.Name, Description: criterion.Description,
			Weight: criterion.Weight, Mandatory: criterion.Mandatory, Levels: criterion.Levels[:],
		})
	}
	if view.Submission != nil {
		dto.Submission = &SubmissionDTO{
			ContentType: view.Submission.ContentType,
			Content:     view.Submission.Content,
			Checksum:    view.Submission.Checksum,
			SubmittedAt: view.Submission.SubmittedAt.Format(time.RFC3339),
		}
	}
	return dto
}

// StartAttempt starts an exam on the challenge, standalone when sessionID is
// empty. If another attempt is running it fails with attempt_in_progress
// unless discardInProgress is true, which replaces the running one. See
// specs/phases/phase-03-architecture-lab/04-exercise-service.md.
func (a *App) StartAttempt(challengeID, sessionID string, discardInProgress bool) (AttemptDTO, error) {
	if strings.TrimSpace(challengeID) == "" {
		return AttemptDTO{}, invalidInput("challenge id is required")
	}
	view, err := a.exercise.StartAttempt(a.ctx, applicationexercise.StartInput{
		ChallengeID:       challengeID,
		SessionID:         strings.TrimSpace(sessionID),
		DiscardInProgress: discardInProgress,
	})
	if err != nil {
		return AttemptDTO{}, toExerciseError(err)
	}
	return toAttemptDTO(view), nil
}

// SubmitAttempt ends the attempt with the drawn artifact. The backend checks
// its own clock; a submission far past the deadline fails with
// submission_too_late and the attempt is discarded.
func (a *App) SubmitAttempt(attemptID, contentType, content string) (AttemptDTO, error) {
	if strings.TrimSpace(attemptID) == "" {
		return AttemptDTO{}, invalidInput("attempt id is required")
	}
	view, err := a.exercise.SubmitAttempt(a.ctx, attemptID, contentType, content)
	if err != nil {
		return AttemptDTO{}, toExerciseError(err)
	}
	return toAttemptDTO(view), nil
}

// DiscardAttempt deletes an unfinished attempt when the user leaves the lab.
func (a *App) DiscardAttempt(attemptID string) error {
	if strings.TrimSpace(attemptID) == "" {
		return invalidInput("attempt id is required")
	}
	if err := a.exercise.DiscardAttempt(a.ctx, attemptID); err != nil {
		return toExerciseError(err)
	}
	return nil
}

// GetAttempt returns one attempt with its rubric criteria and, once
// submitted, its submission.
func (a *App) GetAttempt(attemptID string) (AttemptDTO, error) {
	if strings.TrimSpace(attemptID) == "" {
		return AttemptDTO{}, invalidInput("attempt id is required")
	}
	view, err := a.exercise.GetAttempt(a.ctx, attemptID)
	if err != nil {
		return AttemptDTO{}, toExerciseError(err)
	}
	return toAttemptDTO(view), nil
}

// ListAttempts returns the challenge's attempts, oldest first.
func (a *App) ListAttempts(challengeID string) ([]AttemptDTO, error) {
	if strings.TrimSpace(challengeID) == "" {
		return nil, invalidInput("challenge id is required")
	}
	views, err := a.exercise.ListAttempts(a.ctx, challengeID)
	if err != nil {
		return nil, toExerciseError(err)
	}
	dtos := make([]AttemptDTO, 0, len(views))
	for _, view := range views {
		dtos = append(dtos, toAttemptDTO(view))
	}
	return dtos, nil
}

// discardStaleAttempts removes the exams a crash or a close left behind. It
// runs once from Startup, before the frontend can start another, and only
// logs: an error here must never stop the app from starting.
func (a *App) discardStaleAttempts() {
	if a.exercise == nil {
		return
	}
	removed, err := a.exercise.DiscardStaleAttempts(a.ctx)
	if err != nil {
		log.Printf("discarding stale exam attempts: %v", err)
		return
	}
	log.Printf("discarded %d stale exam attempt(s)", removed)
}
