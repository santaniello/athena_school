package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	applicationarchitecture "github.com/santaniello/athena/internal/application/architecture"
	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// WithAttemptService wires the exam attempt use cases (spec 3.4).
func WithAttemptService(service *applicationexercise.AttemptService) Option {
	return func(a *App) { a.exercise = service }
}

// challengeGenerator is the use case behind GenerateChallenge (spec 3.6.1).
type challengeGenerator interface {
	GenerateChallenge(ctx context.Context, sessionID string, spec domainexercise.ChallengeSpec) (domainexercise.Challenge, error)
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

// hintGiver is the use case behind the hint bindings (spec 3.10.1).
type hintGiver interface {
	RequestHint(ctx context.Context, attemptID, boardXML string) (domainexercise.Hint, error)
	ListHints(ctx context.Context, attemptID string) ([]domainexercise.Hint, error)
}

// WithHintService wires the tutor hints given during an exam.
func WithHintService(hints hintGiver) Option {
	return func(a *App) { a.hints = hints }
}

// attemptEvaluator is the use case behind the evaluation bindings (spec 3.11).
type attemptEvaluator interface {
	Evaluate(ctx context.Context, attemptID string) (domainexercise.Evaluation, error)
	GetEvaluation(ctx context.Context, attemptID string) (domainexercise.Evaluation, error)
	Criteria(ctx context.Context, attemptID string) ([]domainexercise.RubricCriterion, error)
}

// WithEvaluationService wires the evaluation of a submitted attempt.
func WithEvaluationService(evaluations attemptEvaluator) Option {
	return func(a *App) { a.evaluations = evaluations }
}

// defenseRunner is the use case behind the defense bindings (spec 3.12).
type defenseRunner interface {
	NextQuestion(ctx context.Context, attemptID string) (domainexercise.DefenseTurn, error)
	Answer(ctx context.Context, attemptID, answer string) (applicationexercise.DefenseView, error)
	Evaluate(ctx context.Context, attemptID string) (domainexercise.Evaluation, error)
	GetDefense(ctx context.Context, attemptID string) (applicationexercise.DefenseView, error)
	Outcome(ctx context.Context, attemptID string) (applicationexercise.DefenseOutcome, error)
}

// WithDefenseService wires the tutor's questions and the final evaluation.
func WithDefenseService(defense defenseRunner) Option {
	return func(a *App) { a.defense = defense }
}

// historyReader is the use case behind the history bindings (spec 3.13).
type historyReader interface {
	ListHistory(ctx context.Context, challengeID string) ([]applicationexercise.AttemptListItem, error)
	GetAttemptResult(ctx context.Context, attemptID string) (applicationexercise.AttemptResultView, error)
}

// WithHistoryService wires the history and result of finished attempts.
func WithHistoryService(history historyReader) Option {
	return func(a *App) { a.history = history }
}

// exerciseOffers is the use case behind the exercise offer bindings (spec 3.14).
type exerciseOffers interface {
	GetPending(ctx context.Context, sessionID string) (domainexercise.Offer, bool, error)
	Accept(ctx context.Context, offerID string, spec domainexercise.ChallengeSpec) (applicationexercise.SessionExerciseView, error)
	Decline(ctx context.Context, offerID string) error
}

// WithOfferService wires the exercise offers of the study chat.
func WithOfferService(offers exerciseOffers) Option {
	return func(a *App) { a.offers = offers }
}

// sessionExerciseCards is the use case behind the exercise cards of the study
// chat (spec 3.17).
type sessionExerciseCards interface {
	ListSessionExercises(ctx context.Context, sessionID string) ([]applicationexercise.SessionExerciseCard, error)
}

// WithSessionExerciseService wires the exercise cards of the study chat.
func WithSessionExerciseService(cards sessionExerciseCards) Option {
	return func(a *App) { a.sessionCards = cards }
}

// Stable ExerciseError.Code values the frontend switches on.
const (
	exerciseCodeInvalidInput         = "invalid_input"
	exerciseCodeUnavailable          = "unavailable"
	exerciseCodeChallengeNotFound    = "challenge_not_found"
	exerciseCodeChallengeHasNoRubric = "challenge_has_no_rubric"
	exerciseCodeAttemptInProgress    = "attempt_in_progress"
	exerciseCodeSessionNotFound      = "source_session_not_found"
	exerciseCodeSessionNotLinked     = "session_exercise_not_found"
	exerciseCodeAttemptNotFound      = "attempt_not_found"
	exerciseCodeAttemptNotInProgress = "attempt_not_in_progress"
	exerciseCodeAttemptFinished      = "attempt_finished"
	exerciseCodeAttemptNotFinished   = "attempt_not_finished"
	exerciseCodeSubmissionTooLate    = "submission_too_late"
	exerciseCodeSubmissionInvalid    = "submission_invalid"
	exerciseCodeSubmissionTooLarge   = "submission_too_large"
	exerciseCodeInvalidGeneration    = "invalid_generation"
	exerciseCodeCancelled            = "cancelled"
	exerciseCodeNoHintsLeft          = "no_hints_left"
	exerciseCodeHintInProgress       = "hint_in_progress"
	exerciseCodeGenerationInProgress = "generation_in_progress"
	exerciseCodeInvalidDiagram       = "invalid_diagram"
	exerciseCodeEmptyDiagram         = "empty_diagram"
	exerciseCodeInvalidEvaluation    = "invalid_evaluation"
	exerciseCodeAttemptNotEvaluable  = "attempt_not_evaluable"
	exerciseCodeEvaluationNotFound   = "evaluation_not_found"
	exerciseCodeDefenseNotActive     = "defense_not_active"
	exerciseCodeQuestionPending      = "question_pending"
	exerciseCodeNoQuestionPending    = "no_question_pending"
	exerciseCodeInvalidAnswer        = "invalid_answer"
	exerciseCodeEmptyQuestion        = "empty_question"
	exerciseCodeOfferNotFound        = "offer_not_found"
	exerciseCodeOfferPending         = "offer_pending"
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
	case errors.Is(err, applicationexercise.ErrAttemptNotFinished):
		return &ExerciseError{Code: exerciseCodeAttemptNotFinished, Message: "the attempt is not finished"}
	case errors.Is(err, applicationexercise.ErrSubmissionTooLate):
		return &ExerciseError{Code: exerciseCodeSubmissionTooLate, Message: "the submission arrived after the time limit and the attempt was discarded"}
	case errors.Is(err, domainexercise.ErrNoHintsLeft):
		return &ExerciseError{Code: exerciseCodeNoHintsLeft, Message: "no hints left for this attempt"}
	case errors.Is(err, domainexercise.ErrHintInProgress):
		return &ExerciseError{Code: exerciseCodeHintInProgress, Message: "a hint is already being requested"}
	case errors.Is(err, applicationexercise.ErrInvalidGeneration):
		return &ExerciseError{Code: exerciseCodeInvalidGeneration, Message: "the generated challenge was not valid, try again"}
	case errors.Is(err, applicationarchitecture.ErrInvalidDiagram):
		return &ExerciseError{Code: exerciseCodeInvalidDiagram, Message: "the diagram could not be read"}
	case errors.Is(err, applicationarchitecture.ErrEmptyDiagram):
		return &ExerciseError{Code: exerciseCodeEmptyDiagram, Message: "the diagram is empty"}
	case errors.Is(err, domainexercise.ErrInvalidEvaluation):
		return &ExerciseError{Code: exerciseCodeInvalidEvaluation, Message: "the evaluator returned an invalid evaluation"}
	case errors.Is(err, domainexercise.ErrAttemptNotEvaluable):
		return &ExerciseError{Code: exerciseCodeAttemptNotEvaluable, Message: "the attempt cannot be evaluated now"}
	case errors.Is(err, domainexercise.ErrEvaluationNotFound):
		return &ExerciseError{Code: exerciseCodeEvaluationNotFound, Message: "the attempt has no evaluation"}
	case errors.Is(err, domainexercise.ErrDefenseNotActive):
		return &ExerciseError{Code: exerciseCodeDefenseNotActive, Message: "the defense is not active"}
	case errors.Is(err, domainexercise.ErrQuestionPending):
		return &ExerciseError{Code: exerciseCodeQuestionPending, Message: "a question is waiting for its answer"}
	case errors.Is(err, domainexercise.ErrNoQuestionPending):
		return &ExerciseError{Code: exerciseCodeNoQuestionPending, Message: "there is no question waiting for an answer"}
	case errors.Is(err, domainexercise.ErrInvalidAnswer):
		return &ExerciseError{Code: exerciseCodeInvalidAnswer, Message: "the answer must be 1 to 4000 characters"}
	case errors.Is(err, applicationexercise.ErrEmptyQuestion):
		return &ExerciseError{Code: exerciseCodeEmptyQuestion, Message: "the tutor wrote an empty question"}
	case errors.Is(err, domainexercise.ErrOfferNotFound):
		return &ExerciseError{Code: exerciseCodeOfferNotFound, Message: "the exercise offer is no longer pending"}
	case errors.Is(err, domainexercise.ErrOfferPending):
		return &ExerciseError{Code: exerciseCodeOfferPending, Message: "an exercise offer is already pending"}
	case errors.Is(err, context.Canceled):
		return &ExerciseError{Code: exerciseCodeCancelled, Message: "cancelled"}
	case errors.Is(err, domainexercise.ErrSubmissionTooLarge):
		return &ExerciseError{Code: exerciseCodeSubmissionTooLarge, Message: err.Error()}
	case isInvalidSubmission(err):
		return &ExerciseError{Code: exerciseCodeSubmissionInvalid, Message: err.Error()}
	}
	log.Printf("exercise binding failed: %v", err)
	return &ExerciseError{Code: exerciseCodeInternal, Message: "something went wrong"}
}

func isInvalidSubmission(err error) bool {
	return errors.Is(err, domainexercise.ErrSubmissionContentRequired) ||
		errors.Is(err, domainexercise.ErrSubmissionContentTypeRequired)
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

// StatementDTO is the challenge statement the exam screen shows.
type StatementDTO struct {
	Scenario                  string   `json:"scenario"`
	FunctionalRequirements    []string `json:"functionalRequirements"`
	NonFunctionalRequirements []string `json:"nonFunctionalRequirements"`
	Constraints               []string `json:"constraints"`
	Objectives                []string `json:"objectives"`
	// StarterDiagram is the mxGraphModel XML a practice attempt starts from;
	// empty when the board starts blank.
	StarterDiagram string `json:"starterDiagram"`
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
	Deadline          string         `json:"deadline"` // empty when untimed
	SubmittedAt       string         `json:"submittedAt"`
	SubmitReason      string         `json:"submitReason"`
	Statement         StatementDTO   `json:"statement"`
	Criteria          []CriterionDTO `json:"criteria"`
	Submission        *SubmissionDTO `json:"submission"`
}

// deadlineText is the attempt's deadline in RFC 3339, or empty for an untimed
// attempt, which has none.
func deadlineText(attempt domainexercise.ChallengeAttempt) string {
	if attempt.Untimed() {
		return ""
	}
	return attempt.Deadline.Format(time.RFC3339)
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
		Deadline:          deadlineText(attempt),
		SubmitReason:      string(attempt.SubmitReason),
		Statement: StatementDTO{
			Scenario:                  view.Statement.Scenario,
			FunctionalRequirements:    view.Statement.FunctionalRequirements,
			NonFunctionalRequirements: view.Statement.NonFunctionalRequirements,
			Constraints:               view.Statement.Constraints,
			Objectives:                view.Statement.Objectives,
			StarterDiagram:            view.Statement.StarterDiagram,
		},
		Criteria: make([]CriterionDTO, 0, len(view.Criteria)),
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
	ID         string `json:"id"`
	Kind       string `json:"kind"` // INTERVIEW or PRACTICE
	Title      string `json:"title"`
	Topic      string `json:"topic"`
	Difficulty string `json:"difficulty"`
	// TimeLimitMinutes is 0 for an untimed practice.
	TimeLimitMinutes int `json:"timeLimitMinutes"`
}

// GenerateChallenge asks the AI for a challenge and its rubric from a studied
// session and saves them to the catalog; it does not start the exam. The kind
// is INTERVIEW (empty means the same) or PRACTICE, and a practice needs the
// goal the user wants to practice. A failure stores nothing and may be
// retried. See specs/phases/phase-03-architecture-lab/06-01-challenge-generation.md
// and 18-practice-challenges.md.
func (a *App) GenerateChallenge(sessionID, difficulty, kind, practiceGoal string) (ChallengeSummaryDTO, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ChallengeSummaryDTO{}, invalidInput("session id is required")
	}
	spec, err := parseChallengeSpec(difficulty, kind, practiceGoal, false)
	if err != nil {
		return ChallengeSummaryDTO{}, err
	}
	ctx, release, ok := a.beginGeneration()
	if !ok {
		return ChallengeSummaryDTO{}, &ExerciseError{Code: exerciseCodeGenerationInProgress, Message: "a challenge is already being generated"}
	}
	defer release()
	challenge, err := a.generator.GenerateChallenge(ctx, sessionID, spec)
	if err != nil {
		return ChallengeSummaryDTO{}, toExerciseError(err)
	}
	return ChallengeSummaryDTO{
		ID:               challenge.ID,
		Kind:             string(challenge.Kind),
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
	Kind             string `json:"kind"`
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
			ID: item.ChallengeID, Kind: string(item.Kind), Title: item.Title, Topic: item.Topic, Difficulty: string(item.Difficulty),
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

// beginGeneration reserves the single generation slot. It returns the context
// the generation runs under and the function that frees the slot, or false
// when a generation is already running.
func (a *App) beginGeneration() (context.Context, func(), bool) {
	a.generationMu.Lock()
	defer a.generationMu.Unlock()
	if a.cancelGeneration != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelGeneration = cancel
	return ctx, func() {
		cancel()
		a.generationMu.Lock()
		a.cancelGeneration = nil
		a.generationMu.Unlock()
	}, true
}

// CancelChallengeGeneration cancels the challenge generation in flight, which
// then fails with the cancelled code and stores nothing. With none running it
// does nothing.
func (a *App) CancelChallengeGeneration() error {
	a.generationMu.Lock()
	defer a.generationMu.Unlock()
	if a.cancelGeneration != nil {
		a.cancelGeneration()
	}
	return nil
}

// HintDTO is one tutor hint. CreatedAt is RFC 3339.
type HintDTO struct {
	ID        string `json:"id"`
	AttemptID string `json:"attemptId"`
	Position  int    `json:"position"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

func toHintDTO(hint domainexercise.Hint) HintDTO {
	return HintDTO{
		ID: hint.ID, AttemptID: hint.AttemptID, Position: hint.Position, Content: hint.Content,
		CreatedAt: hint.CreatedAt.Format(time.RFC3339),
	}
}

// RequestHint asks the tutor for a hint on the running attempt, given the
// current board. A failure spends nothing and can be repeated. See
// specs/phases/phase-03-architecture-lab/10-01-tutor-hints.md.
func (a *App) RequestHint(attemptID, boardXML string) (HintDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return HintDTO{}, invalidInput("attempt id is required")
	}
	hint, err := a.hints.RequestHint(a.ctx, attemptID, boardXML)
	if err != nil {
		return HintDTO{}, toExerciseError(err)
	}
	return toHintDTO(hint), nil
}

// ListHints returns the hints already given for the attempt, in order.
func (a *App) ListHints(attemptID string) ([]HintDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return nil, invalidInput("attempt id is required")
	}
	hints, err := a.hints.ListHints(a.ctx, attemptID)
	if err != nil {
		return nil, toExerciseError(err)
	}
	dtos := make([]HintDTO, 0, len(hints))
	for _, hint := range hints {
		dtos = append(dtos, toHintDTO(hint))
	}
	return dtos, nil
}

// EvaluationResultDTO is the judgement of one rubric criterion.
type EvaluationResultDTO struct {
	CriterionKey         string `json:"criterionKey"`
	CriterionName        string `json:"criterionName"`
	Weight               int    `json:"weight"`
	Level                int    `json:"level"`
	Justification        string `json:"justification"`
	Evidence             string `json:"evidence"`
	InsufficientEvidence bool   `json:"insufficientEvidence"`
}

// EvaluationDTO is the diagram evaluation of an attempt. Score is computed by
// the domain; CreatedAt is RFC 3339.
type EvaluationDTO struct {
	AttemptID       string                `json:"attemptId"`
	Model           string                `json:"model"`
	Score           int                   `json:"score"`
	Summary         string                `json:"summary"`
	Strengths       []string              `json:"strengths"`
	Gaps            []string              `json:"gaps"`
	Recommendations []string              `json:"recommendations"`
	Results         []EvaluationResultDTO `json:"results"`
	CreatedAt       string                `json:"createdAt"`
}

func toEvaluationDTO(evaluation domainexercise.Evaluation, criteria []domainexercise.RubricCriterion) EvaluationDTO {
	byID := make(map[string]domainexercise.RubricCriterion, len(criteria))
	for _, criterion := range criteria {
		byID[criterion.ID] = criterion
	}
	dto := newEvaluationDTO(evaluation)
	for _, result := range evaluation.Results {
		dto.Results = append(dto.Results, toEvaluationResultDTO(byID[result.CriterionID], result))
	}
	return dto
}

// toEvaluationViewDTO maps an evaluation whose results are already joined
// with their criteria.
func toEvaluationViewDTO(view applicationexercise.EvaluationView) EvaluationDTO {
	dto := newEvaluationDTO(view.Evaluation)
	for _, row := range view.Rows {
		dto.Results = append(dto.Results, toEvaluationResultDTO(row.Criterion, row.Result))
	}
	return dto
}

func newEvaluationDTO(evaluation domainexercise.Evaluation) EvaluationDTO {
	return EvaluationDTO{
		AttemptID: evaluation.AttemptID, Model: evaluation.Model, Score: evaluation.Score, Summary: evaluation.Summary,
		Strengths:       append([]string{}, evaluation.Strengths...),
		Gaps:            append([]string{}, evaluation.Gaps...),
		Recommendations: append([]string{}, evaluation.Recommendations...),
		Results:         make([]EvaluationResultDTO, 0, len(evaluation.Results)),
		CreatedAt:       evaluation.CreatedAt.Format(time.RFC3339),
	}
}

func toEvaluationResultDTO(criterion domainexercise.RubricCriterion, result domainexercise.CriterionResult) EvaluationResultDTO {
	return EvaluationResultDTO{
		CriterionKey: criterion.Key, CriterionName: criterion.Name, Weight: criterion.Weight, Level: result.Level,
		Justification: result.Justification, Evidence: result.Evidence, InsufficientEvidence: result.InsufficientEvidence,
	}
}

// EvaluateAttempt evaluates the submitted attempt's diagram, or returns the
// stored evaluation if it was already evaluated. A failure leaves the attempt
// EVALUATION_FAILED and can be retried by calling it again. See
// specs/phases/phase-03-architecture-lab/11-architecture-evaluator.md.
func (a *App) EvaluateAttempt(attemptID string) (EvaluationDTO, error) {
	return a.evaluationOf(attemptID, a.evaluations.Evaluate)
}

// GetEvaluation returns the attempt's stored diagram evaluation.
func (a *App) GetEvaluation(attemptID string) (EvaluationDTO, error) {
	return a.evaluationOf(attemptID, a.evaluations.GetEvaluation)
}

func (a *App) evaluationOf(
	attemptID string, load func(context.Context, string) (domainexercise.Evaluation, error),
) (EvaluationDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return EvaluationDTO{}, invalidInput("attempt id is required")
	}
	evaluation, err := load(a.ctx, attemptID)
	if err != nil {
		return EvaluationDTO{}, toExerciseError(err)
	}
	criteria, err := a.evaluations.Criteria(a.ctx, attemptID)
	if err != nil {
		return EvaluationDTO{}, toExerciseError(err)
	}
	return toEvaluationDTO(evaluation, criteria), nil
}

// DefenseTurnDTO is one question of the defense with its answer. It carries
// the question text but not the criterion it targets. AskedAt and AnsweredAt
// are RFC 3339; Answer and AnsweredAt are empty until answered.
type DefenseTurnDTO struct {
	Position   int    `json:"position"`
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	AskedAt    string `json:"askedAt"`
	AnsweredAt string `json:"answeredAt"`
}

// DefenseQuestionDTO is the question the user must answer now, with how many
// questions the defense has. It does not say which criterion it targets.
type DefenseQuestionDTO struct {
	Position int    `json:"position"`
	Question string `json:"question"`
	Total    int    `json:"total"`
}

// DefenseDTO is the defense of an attempt: how many questions it has, the
// turns so far in order, and whether the last answer is in.
type DefenseDTO struct {
	Total    int              `json:"total"`
	Turns    []DefenseTurnDTO `json:"turns"`
	Complete bool             `json:"complete"`
}

func toDefenseTurnDTO(turn domainexercise.DefenseTurn) DefenseTurnDTO {
	dto := DefenseTurnDTO{
		Position: turn.Position, Question: turn.Question, Answer: turn.Answer,
		AskedAt: turn.AskedAt.Format(time.RFC3339),
	}
	if turn.Answered() {
		dto.AnsweredAt = turn.AnsweredAt.Format(time.RFC3339)
	}
	return dto
}

func toDefenseDTO(view applicationexercise.DefenseView) DefenseDTO {
	dto := DefenseDTO{Total: view.Total, Complete: view.Complete, Turns: make([]DefenseTurnDTO, 0, len(view.Turns))}
	for _, turn := range view.Turns {
		dto.Turns = append(dto.Turns, toDefenseTurnDTO(turn))
	}
	return dto
}

// OutcomeReasonDTO is one rule a failed attempt broke. CriterionKey and
// CriterionName are set only for MANDATORY_CRITERION.
type OutcomeReasonDTO struct {
	Kind          string `json:"kind"`
	Evaluation    string `json:"evaluation"`
	CriterionKey  string `json:"criterionKey"`
	CriterionName string `json:"criterionName"`
}

// OutcomeDTO is the end of an attempt: PASSED or FAILED, both scores, the
// minimum passing score frozen on the attempt and, when failed, every reason.
type OutcomeDTO struct {
	Result             string             `json:"result"`
	PassScore          int                `json:"passScore"`
	DiagramScore       int                `json:"diagramScore"`
	UnderstandingScore int                `json:"understandingScore"`
	Reasons            []OutcomeReasonDTO `json:"reasons"`
}

func toOutcomeDTO(outcome applicationexercise.DefenseOutcome) OutcomeDTO {
	byID := make(map[string]domainexercise.RubricCriterion, len(outcome.Criteria))
	for _, criterion := range outcome.Criteria {
		byID[criterion.ID] = criterion
	}
	dto := OutcomeDTO{
		Result: string(outcome.Outcome.Result), PassScore: outcome.PassScore,
		DiagramScore: outcome.Diagram.Score, UnderstandingScore: outcome.Understanding.Score,
		Reasons: make([]OutcomeReasonDTO, 0, len(outcome.Outcome.Reasons)),
	}
	for _, reason := range outcome.Outcome.Reasons {
		criterion := byID[reason.CriterionID]
		dto.Reasons = append(dto.Reasons, OutcomeReasonDTO{
			Kind: string(reason.Kind), Evaluation: string(reason.Evaluation),
			CriterionKey: criterion.Key, CriterionName: criterion.Name,
		})
	}
	return dto
}

// NextDefenseQuestion returns the question the user must answer now. The first
// call starts the defense; a question still waiting for its answer is returned
// again. After the last answer it fails with defense_not_active. See
// specs/phases/phase-03-architecture-lab/12-architecture-defense.md.
func (a *App) NextDefenseQuestion(attemptID string) (DefenseQuestionDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return DefenseQuestionDTO{}, invalidInput("attempt id is required")
	}
	turn, err := a.defense.NextQuestion(a.ctx, attemptID)
	if err != nil {
		return DefenseQuestionDTO{}, toExerciseError(err)
	}
	view, err := a.defense.GetDefense(a.ctx, attemptID)
	if err != nil {
		return DefenseQuestionDTO{}, toExerciseError(err)
	}
	return DefenseQuestionDTO{Position: turn.Position, Question: turn.Question, Total: view.Total}, nil
}

// AnswerDefenseQuestion stores the answer to the question that is waiting and
// returns the defense, complete once the last answer is in.
func (a *App) AnswerDefenseQuestion(attemptID, answer string) (DefenseDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return DefenseDTO{}, invalidInput("attempt id is required")
	}
	view, err := a.defense.Answer(a.ctx, attemptID, answer)
	if err != nil {
		return DefenseDTO{}, toExerciseError(err)
	}
	return toDefenseDTO(view), nil
}

// EvaluateDefense judges the answers once all are given, finishes the attempt
// and returns its outcome. A failure leaves the attempt defending and can be
// retried by calling it again.
func (a *App) EvaluateDefense(attemptID string) (OutcomeDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return OutcomeDTO{}, invalidInput("attempt id is required")
	}
	if _, err := a.defense.Evaluate(a.ctx, attemptID); err != nil {
		return OutcomeDTO{}, toExerciseError(err)
	}
	outcome, err := a.defense.Outcome(a.ctx, attemptID)
	if err != nil {
		return OutcomeDTO{}, toExerciseError(err)
	}
	return toOutcomeDTO(outcome), nil
}

// GetDefense returns the defense of an attempt whose diagram was evaluated.
func (a *App) GetDefense(attemptID string) (DefenseDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return DefenseDTO{}, invalidInput("attempt id is required")
	}
	view, err := a.defense.GetDefense(a.ctx, attemptID)
	if err != nil {
		return DefenseDTO{}, toExerciseError(err)
	}
	return toDefenseDTO(view), nil
}

// AttemptHistoryItemDTO is one finished attempt in a challenge's history.
// FinishedAt is RFC 3339.
type AttemptHistoryItemDTO struct {
	AttemptID          string `json:"attemptId"`
	FinishedAt         string `json:"finishedAt"`
	DiagramScore       int    `json:"diagramScore"`
	UnderstandingScore int    `json:"understandingScore"`
	Outcome            string `json:"outcome"`
}

// ListAttemptHistory returns the challenge's finished attempts, newest first.
// See specs/phases/phase-03-architecture-lab/13-evaluation-and-history-screen.md.
func (a *App) ListAttemptHistory(challengeID string) ([]AttemptHistoryItemDTO, error) {
	challengeID = strings.TrimSpace(challengeID)
	if challengeID == "" {
		return nil, invalidInput("challenge id is required")
	}
	items, err := a.history.ListHistory(a.ctx, challengeID)
	if err != nil {
		return nil, toExerciseError(err)
	}
	dtos := make([]AttemptHistoryItemDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, AttemptHistoryItemDTO{
			AttemptID: item.AttemptID, FinishedAt: item.FinishedAt.Format(time.RFC3339),
			DiagramScore: item.DiagramScore, UnderstandingScore: item.UnderstandingScore, Outcome: string(item.Outcome),
		})
	}
	return dtos, nil
}

// ResultChallengeDTO is the challenge as a finished attempt answered it, with
// the statement the exam showed.
type ResultChallengeDTO struct {
	ID         string       `json:"id"`
	Title      string       `json:"title"`
	Topic      string       `json:"topic"`
	Difficulty string       `json:"difficulty"`
	Statement  StatementDTO `json:"statement"`
}

// AttemptResultDTO is everything the result screen shows of a finished
// attempt. Times are RFC 3339; Submission is the stored diagram XML.
type AttemptResultDTO struct {
	Challenge        ResultChallengeDTO `json:"challenge"`
	AttemptID        string             `json:"attemptId"`
	StartedAt        string             `json:"startedAt"`
	SubmittedAt      string             `json:"submittedAt"`
	FinishedAt       string             `json:"finishedAt"`
	TimeSpentSeconds int                `json:"timeSpentSeconds"`
	SubmitReason     string             `json:"submitReason"`
	HintsUsed        int                `json:"hintsUsed"`
	MaxHints         int                `json:"maxHints"`
	Outcome          OutcomeDTO         `json:"outcome"`
	Diagram          EvaluationDTO      `json:"diagram"`
	Understanding    EvaluationDTO      `json:"understanding"`
	Defense          []DefenseTurnDTO   `json:"defense"`
	Submission       string             `json:"submission"`
}

func toAttemptResultDTO(view applicationexercise.AttemptResultView) AttemptResultDTO {
	statement := view.Challenge.Statement
	dto := AttemptResultDTO{
		Challenge: ResultChallengeDTO{
			ID: view.Challenge.ID, Title: view.Challenge.Title, Topic: view.Challenge.Topic,
			Difficulty: string(view.Challenge.Difficulty),
			Statement: StatementDTO{
				Scenario: statement.Scenario, FunctionalRequirements: statement.FunctionalRequirements,
				NonFunctionalRequirements: statement.NonFunctionalRequirements,
				Constraints:               statement.Constraints, Objectives: statement.Objectives,
			},
		},
		AttemptID:        view.Attempt.ID,
		StartedAt:        view.Attempt.StartedAt.Format(time.RFC3339),
		SubmittedAt:      view.Attempt.SubmittedAt.Format(time.RFC3339),
		FinishedAt:       view.FinishedAt.Format(time.RFC3339),
		TimeSpentSeconds: int(view.TimeSpent / time.Second),
		SubmitReason:     string(view.Attempt.SubmitReason),
		HintsUsed:        view.HintsUsed,
		MaxHints:         domainexercise.MaxHints,
		Outcome: toOutcomeDTO(applicationexercise.DefenseOutcome{
			Diagram: view.Diagram.Evaluation, Understanding: view.Understanding.Evaluation,
			Criteria: view.Criteria, PassScore: view.PassScore, Outcome: view.Outcome,
		}),
		Diagram:       toEvaluationViewDTO(view.Diagram),
		Understanding: toEvaluationViewDTO(view.Understanding),
		Defense:       make([]DefenseTurnDTO, 0, len(view.Defense)),
		Submission:    view.Submission,
	}
	for _, turn := range view.Defense {
		dto.Defense = append(dto.Defense, toDefenseTurnDTO(turn))
	}
	return dto
}

// GetAttemptResult returns the full result of a finished attempt: statement,
// outcome, both evaluations, the tutor's questions and answers, and the
// submitted diagram. An attempt that is not finished fails with
// attempt_not_finished.
func (a *App) GetAttemptResult(attemptID string) (AttemptResultDTO, error) {
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return AttemptResultDTO{}, invalidInput("attempt id is required")
	}
	view, err := a.history.GetAttemptResult(a.ctx, attemptID)
	if err != nil {
		return AttemptResultDTO{}, toExerciseError(err)
	}
	return toAttemptResultDTO(view), nil
}

// ExerciseOfferDTO is a pending exercise offer: it hangs under the assistant
// message MessageID.
type ExerciseOfferDTO struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`
}

// SessionExerciseDTO is the challenge an accepted offer linked to the session.
type SessionExerciseDTO struct {
	ChallengeID string `json:"challengeId"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Topic       string `json:"topic"`
	Difficulty  string `json:"difficulty"`
	// TimeLimitMinutes is the exam time shown in the rules dialog. It is 0 for
	// an untimed practice.
	TimeLimitMinutes int `json:"timeLimitMinutes"`
}

// GetPendingOffer returns the session's pending exercise offer, or null when
// there is none. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
func (a *App) GetPendingOffer(sessionID string) (*ExerciseOfferDTO, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, invalidInput("session id is required")
	}
	offer, found, err := a.offers.GetPending(a.ctx, sessionID)
	if err != nil {
		return nil, toExerciseError(err)
	}
	if !found {
		return nil, nil
	}
	return &ExerciseOfferDTO{ID: offer.ID, SessionID: offer.SessionID, MessageID: offer.MessageID}, nil
}

// AcceptOffer links a challenge of the chosen difficulty (BEGINNER,
// INTERMEDIATE or ADVANCED; empty uses the profile's) and kind (INTERVIEW,
// empty meaning the same, or PRACTICE with the goal to practice) to the
// offer's session, generating one when the session has none to reuse, and
// returns it. On failure the offer stays pending.
func (a *App) AcceptOffer(offerID, difficulty, kind, practiceGoal string) (SessionExerciseDTO, error) {
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return SessionExerciseDTO{}, invalidInput("offer id is required")
	}
	wanted, err := parseChallengeSpec(difficulty, kind, practiceGoal, true)
	if err != nil {
		return SessionExerciseDTO{}, err
	}
	ctx, release, ok := a.beginGeneration()
	if !ok {
		return SessionExerciseDTO{}, &ExerciseError{Code: exerciseCodeGenerationInProgress, Message: "a challenge is already being generated"}
	}
	defer release()
	view, err := a.offers.Accept(ctx, offerID, wanted)
	if err != nil {
		return SessionExerciseDTO{}, toExerciseError(err)
	}
	return SessionExerciseDTO{
		ChallengeID: view.ChallengeID, Kind: string(view.Kind), Title: view.Title, Topic: view.Topic, Difficulty: string(view.Difficulty),
		TimeLimitMinutes: view.TimeLimitMinutes,
	}, nil
}

// parseChallengeSpec reads what the user chose when creating a challenge. The
// kind defaults to INTERVIEW and a practice needs its goal. A blank difficulty
// is allowed only where the profile's is used (allowBlankDifficulty).
func parseChallengeSpec(difficulty, kind, practiceGoal string, allowBlankDifficulty bool) (domainexercise.ChallengeSpec, error) {
	spec := domainexercise.ChallengeSpec{
		Difficulty:   domainexercise.Difficulty(strings.TrimSpace(difficulty)),
		Kind:         domainexercise.ChallengeKind(strings.TrimSpace(kind)),
		PracticeGoal: practiceGoal,
	}.Normalized()
	probe := spec
	if allowBlankDifficulty && probe.Difficulty == "" {
		probe.Difficulty = domainexercise.DifficultyIntermediate
	}
	switch err := probe.Validate(); {
	case err == nil:
		return spec, nil
	case errors.Is(err, domainexercise.ErrDifficultyInvalid):
		return spec, invalidInput("difficulty must be BEGINNER, INTERMEDIATE or ADVANCED")
	case errors.Is(err, domainexercise.ErrChallengeKindInvalid):
		return spec, invalidInput("kind must be INTERVIEW or PRACTICE")
	case errors.Is(err, domainexercise.ErrPracticeGoalRequired):
		return spec, invalidInput("say what you want to practice")
	default:
		return spec, invalidInput(fmt.Sprintf("the practice goal must have at most %d characters", domainexercise.MaxPracticeGoalLength))
	}
}

// DeclineOffer removes the offer. Nothing is created.
func (a *App) DeclineOffer(offerID string) error {
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return invalidInput("offer id is required")
	}
	if err := a.offers.Decline(a.ctx, offerID); err != nil {
		return toExerciseError(err)
	}
	return nil
}

// LatestResultDTO is the outcome of the latest finished attempt on a card.
type LatestResultDTO struct {
	Outcome            string `json:"outcome"` // PASSED or FAILED
	DiagramScore       int    `json:"diagramScore"`
	UnderstandingScore int    `json:"understandingScore"`
}

// SessionExerciseCardDTO is the chat card of an accepted exercise. It never
// carries the scenario, requirements or rubric. Latest is null until the
// challenge has a finished attempt.
type SessionExerciseCardDTO struct {
	SessionExerciseID string           `json:"sessionExerciseId"`
	MessageID         string           `json:"messageId"`
	ChallengeID       string           `json:"challengeId"`
	Kind              string           `json:"kind"`
	Title             string           `json:"title"`
	Difficulty        string           `json:"difficulty"`
	TimeLimitMinutes  int              `json:"timeLimitMinutes"`
	Status            string           `json:"status"`
	Attempts          int              `json:"attempts"`
	Latest            *LatestResultDTO `json:"latest"`
}

// ListSessionExercises returns the exercise cards of a study session. See
// specs/phases/phase-03-architecture-lab/17-session-exercises-ui.md.
func (a *App) ListSessionExercises(sessionID string) ([]SessionExerciseCardDTO, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, invalidInput("session id is required")
	}
	if a.sessionCards == nil {
		return nil, &ExerciseError{Code: exerciseCodeUnavailable, Message: "session exercises are not available"}
	}
	cards, err := a.sessionCards.ListSessionExercises(a.ctx, sessionID)
	if err != nil {
		return nil, toExerciseError(err)
	}
	dtos := make([]SessionExerciseCardDTO, len(cards))
	for i, card := range cards {
		dtos[i] = SessionExerciseCardDTO{
			SessionExerciseID: card.SessionExerciseID, MessageID: card.MessageID, ChallengeID: card.ChallengeID,
			Kind: string(card.Kind), Title: card.Title, Difficulty: string(card.Difficulty), TimeLimitMinutes: card.TimeLimitMinutes,
			Status: string(card.Status), Attempts: card.Attempts,
		}
		if card.Latest != nil {
			dtos[i].Latest = &LatestResultDTO{
				Outcome: string(card.Latest.Outcome), DiagramScore: card.Latest.DiagramScore,
				UnderstandingScore: card.Latest.UnderstandingScore,
			}
		}
	}
	return dtos, nil
}
