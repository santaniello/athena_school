package desktop

import (
	"context"
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

// challengeGenerator is the use case behind GenerateChallenge (spec 3.6.1).
type challengeGenerator interface {
	GenerateChallenge(ctx context.Context, sessionID string, difficulty domainexercise.Difficulty) (domainexercise.Challenge, error)
}

// WithGenerationService wires challenge generation from a studied session.
func WithGenerationService(generator challengeGenerator) Option {
	return func(a *App) { a.generator = generator }
}

// challengeCatalog is the use case behind the catalog bindings (spec 3.7).
type challengeCatalog interface {
	ListCatalog(ctx context.Context, filter applicationexercise.CatalogFilter) (applicationexercise.Catalog, error)
	ListTopics(ctx context.Context) ([]string, error)
}

// WithCatalogService wires the challenge catalog.
func WithCatalogService(catalog challengeCatalog) Option {
	return func(a *App) { a.catalog = catalog }
}

// sessionLister is the use case behind ListSessionsForGeneration.
type sessionLister interface {
	ListSessionsForGeneration(ctx context.Context) ([]domainexercise.SessionOption, error)
}

// WithSessionLister wires the list of studied sessions a challenge can be
// generated from.
func WithSessionLister(sessions sessionLister) Option {
	return func(a *App) { a.sessionLister = sessions }
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
	exerciseCodeInvalidGeneration    = "invalid_generation"
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
	case errors.Is(err, applicationexercise.ErrInvalidGeneration):
		return &ExerciseError{Code: exerciseCodeInvalidGeneration, Message: "the generated challenge was not valid, try again"}
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

// ChallengeSummaryDTO is the short read model of a challenge: enough for the
// catalog to list it.
type ChallengeSummaryDTO struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Topic            string `json:"topic"`
	Difficulty       string `json:"difficulty"`
	TimeLimitMinutes int    `json:"timeLimitMinutes"`
}

// GenerateChallenge asks the AI for a challenge and its rubric from a studied
// session and saves them to the catalog; it does not start the exam. A failure
// stores nothing and may be retried. See
// specs/phases/phase-03-architecture-lab/06-01-challenge-generation.md.
func (a *App) GenerateChallenge(sessionID, difficulty string) (ChallengeSummaryDTO, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ChallengeSummaryDTO{}, invalidInput("session id is required")
	}
	level := domainexercise.Difficulty(difficulty)
	if domainexercise.DefaultTimeLimit(level) == 0 {
		return ChallengeSummaryDTO{}, invalidInput("difficulty must be BEGINNER, INTERMEDIATE or ADVANCED")
	}
	challenge, err := a.generator.GenerateChallenge(a.ctx, sessionID, level)
	if err != nil {
		return ChallengeSummaryDTO{}, toExerciseError(err)
	}
	return ChallengeSummaryDTO{
		ID:               challenge.ID,
		Title:            challenge.Title,
		Topic:            challenge.Topic,
		Difficulty:       string(challenge.Difficulty),
		TimeLimitMinutes: challenge.TimeLimitMinutes,
	}, nil
}

// CatalogFilterDTO is the catalog filter the frontend sends. Empty fields do
// not filter.
type CatalogFilterDTO struct {
	Query      string `json:"query"`
	Status     string `json:"status"`
	Difficulty string `json:"difficulty"`
	Topic      string `json:"topic"`
}

// ChallengeCatalogItemDTO is one catalog row. It deliberately has no scenario,
// requirements or rubric: a row must not give the exam away.
type ChallengeCatalogItemDTO struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Topic            string `json:"topic"`
	Difficulty       string `json:"difficulty"`
	TimeLimitMinutes int    `json:"timeLimitMinutes"`
	Status           string `json:"status"`
	Attempts         int    `json:"attempts"`
	BestDiagramScore int    `json:"bestDiagramScore"`
}

// ChallengeCatalogDTO is the catalog with the counts shown above it.
type ChallengeCatalogDTO struct {
	Total      int                       `json:"total"`
	Completed  int                       `json:"completed"`
	InProgress int                       `json:"inProgress"`
	Items      []ChallengeCatalogItemDTO `json:"items"`
}

// SessionOptionDTO is a studied session the generation dialog offers. StartedAt
// is RFC 3339.
type SessionOptionDTO struct {
	ID         string `json:"id"`
	Topic      string `json:"topic"`
	FolderName string `json:"folderName"`
	StartedAt  string `json:"startedAt"`
}

// ListChallengeCatalog lists the challenges matching the filter with their
// progress and the counts above the list. See
// specs/phases/phase-03-architecture-lab/07-challenge-catalog-ui.md.
func (a *App) ListChallengeCatalog(filter CatalogFilterDTO) (ChallengeCatalogDTO, error) {
	status, difficulty := domainexercise.ProgressStatus(filter.Status), domainexercise.Difficulty(filter.Difficulty)
	if !isCatalogStatus(status) {
		return ChallengeCatalogDTO{}, invalidInput("status must be NOT_STARTED, IN_PROGRESS, NEEDS_IMPROVEMENT or COMPLETED")
	}
	if difficulty != "" && domainexercise.DefaultTimeLimit(difficulty) == 0 {
		return ChallengeCatalogDTO{}, invalidInput("difficulty must be BEGINNER, INTERMEDIATE or ADVANCED")
	}
	catalog, err := a.catalog.ListCatalog(a.ctx, applicationexercise.CatalogFilter{
		Query: strings.TrimSpace(filter.Query), Status: status, Difficulty: difficulty, Topic: strings.TrimSpace(filter.Topic),
	})
	if err != nil {
		return ChallengeCatalogDTO{}, toExerciseError(err)
	}
	dto := ChallengeCatalogDTO{
		Total: catalog.Total, Completed: catalog.Completed, InProgress: catalog.InProgress,
		Items: make([]ChallengeCatalogItemDTO, 0, len(catalog.Items)),
	}
	for _, item := range catalog.Items {
		dto.Items = append(dto.Items, ChallengeCatalogItemDTO{
			ID: item.ChallengeID, Title: item.Title, Topic: item.Topic, Difficulty: string(item.Difficulty),
			TimeLimitMinutes: item.TimeLimitMinutes, Status: string(item.Status),
			Attempts: item.Attempts, BestDiagramScore: item.BestDiagramScore,
		})
	}
	return dto, nil
}

func isCatalogStatus(status domainexercise.ProgressStatus) bool {
	switch status {
	case "", domainexercise.ProgressNotStarted, domainexercise.ProgressInProgress,
		domainexercise.ProgressNeedsImprovement, domainexercise.ProgressCompleted:
		return true
	}
	return false
}

// ListChallengeTopics returns the distinct topics present in the catalog, for
// the topic filter.
func (a *App) ListChallengeTopics() ([]string, error) {
	topics, err := a.catalog.ListTopics(a.ctx)
	if err != nil {
		return nil, toExerciseError(err)
	}
	return topics, nil
}

// ListSessionsForGeneration returns every study session a challenge can be
// generated from, newest first. It carries no message content.
func (a *App) ListSessionsForGeneration() ([]SessionOptionDTO, error) {
	options, err := a.sessionLister.ListSessionsForGeneration(a.ctx)
	if err != nil {
		return nil, toExerciseError(err)
	}
	dtos := make([]SessionOptionDTO, 0, len(options))
	for _, option := range options {
		dtos = append(dtos, SessionOptionDTO{
			ID: option.SessionID, Topic: option.Topic, FolderName: option.FolderName,
			StartedAt: option.StartedAt.Format(time.RFC3339),
		})
	}
	return dtos, nil
}
