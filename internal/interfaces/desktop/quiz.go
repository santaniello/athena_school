package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	applicationquiz "github.com/santaniello/athena/internal/application/quiz"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

// quizGenerator is the use case behind GenerateQuiz (spec 3.21).
type quizGenerator interface {
	Generate(ctx context.Context, sessionID string, spec domainquiz.Spec) (domainquiz.Quiz, error)
}

// WithQuizGenerationService wires quiz generation from a studied session.
func WithQuizGenerationService(generator quizGenerator) Option {
	return func(a *App) { a.quizGenerator = generator }
}

// quizUseCases is the use cases behind the catalog, submission and history
// bindings of the Interactive Quiz (spec 3.21).
type quizUseCases interface {
	ListQuizzes(ctx context.Context) ([]domainquiz.Summary, error)
	ListSessionQuizzes(ctx context.Context, sessionID string) ([]domainquiz.Summary, error)
	GetQuiz(ctx context.Context, id string) (domainquiz.Quiz, error)
	SubmitAttempt(ctx context.Context, quizID string, selections []domainquiz.Selection) (domainquiz.Attempt, error)
	ListAttempts(ctx context.Context, quizID string) ([]domainquiz.Attempt, error)
	GetAttemptResult(ctx context.Context, attemptID string) (applicationquiz.AttemptResult, error)
	DeleteQuiz(ctx context.Context, id string) error
}

// WithQuizService wires the quiz catalog, submission and history.
func WithQuizService(quizzes quizUseCases) Option {
	return func(a *App) { a.quizzes = quizzes }
}

// quizOffers is the use case behind the quiz offer bindings of the study chat
// (spec 3.23).
type quizOffers interface {
	GetPending(ctx context.Context, sessionID string) (domainquiz.Offer, bool, error)
	Accept(ctx context.Context, offerID string, spec domainquiz.Spec) (domainquiz.Summary, error)
	Decline(ctx context.Context, offerID string) error
}

// WithQuizOfferService wires the quiz offers of the study chat.
func WithQuizOfferService(offers quizOffers) Option {
	return func(a *App) { a.quizOffers = offers }
}

// Stable QuizError.Code values the frontend switches on.
const (
	quizCodeInvalidInput         = "invalid_input"
	quizCodeSessionNotFound      = "source_session_not_found"
	quizCodeQuizNotFound         = "quiz_not_found"
	quizCodeAttemptNotFound      = "attempt_not_found"
	quizCodeOfferNotFound        = "offer_not_found"
	quizCodeOfferPending         = "offer_pending"
	quizCodeInvalidGeneration    = "invalid_generation"
	quizCodeGenerationInProgress = "generation_in_progress"
	quizCodeSelectionInvalid     = "selection_invalid"
	quizCodeCancelled            = "cancelled"
	quizCodeUnavailable          = "unavailable"
	quizCodeInternal             = "internal"
)

// QuizError is the error the quiz bindings return. Wails hands the frontend
// only the error's text, so Error() is this struct as JSON:
// {"code": "...", "message": "..."}.
type QuizError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *QuizError) Error() string {
	encoded, err := json.Marshal(e)
	if err != nil {
		return e.Code
	}
	return string(encoded)
}

// toQuizError maps a use case error to its coded form. Anything unknown is
// logged with its detail and reported as a generic internal error.
func toQuizError(err error) *QuizError {
	switch {
	case errors.Is(err, applicationquiz.ErrSourceSessionNotFound):
		return &QuizError{Code: quizCodeSessionNotFound, Message: "study session not found"}
	case errors.Is(err, domainquiz.ErrQuizNotFound):
		return &QuizError{Code: quizCodeQuizNotFound, Message: "quiz not found"}
	case errors.Is(err, domainquiz.ErrAttemptNotFound):
		return &QuizError{Code: quizCodeAttemptNotFound, Message: "quiz attempt not found"}
	case errors.Is(err, domainquiz.ErrOfferNotFound):
		return &QuizError{Code: quizCodeOfferNotFound, Message: "the quiz offer is no longer pending"}
	case errors.Is(err, domainquiz.ErrOfferPending):
		return &QuizError{Code: quizCodeOfferPending, Message: "a quiz offer is already pending"}
	case errors.Is(err, applicationquiz.ErrInvalidGeneration):
		return &QuizError{Code: quizCodeInvalidGeneration, Message: "the generated quiz was not valid, try again"}
	case errors.Is(err, domainquiz.ErrQuestionCountInvalid), errors.Is(err, domainquiz.ErrDifficultyInvalid):
		return &QuizError{Code: quizCodeInvalidInput, Message: err.Error()}
	case errors.Is(err, domainquiz.ErrSelectionUnknownQuestion),
		errors.Is(err, domainquiz.ErrSelectionDuplicate),
		errors.Is(err, domainquiz.ErrSelectionOutOfRange):
		return &QuizError{Code: quizCodeSelectionInvalid, Message: err.Error()}
	case errors.Is(err, context.Canceled):
		return &QuizError{Code: quizCodeCancelled, Message: "cancelled"}
	}
	log.Printf("quiz binding failed: %v", err)
	return &QuizError{Code: quizCodeInternal, Message: "something went wrong"}
}

// quizUnavailable is the error of a quiz binding whose service was not wired,
// which the chat would otherwise hit with a nil dereference on every session it
// opens.
func quizUnavailable() *QuizError {
	return &QuizError{Code: quizCodeUnavailable, Message: "the interactive test is not available"}
}

// QuizSummaryDTO is a quiz as the catalog lists it. CreatedAt is RFC 3339.
// LatestScore is null until the quiz has been taken. MessageID is the study-chat
// message whose offer produced the quiz, empty for a quiz from the menu.
type QuizSummaryDTO struct {
	ID            string   `json:"id"`
	SessionID     string   `json:"sessionId"`
	SessionTitle  string   `json:"sessionTitle"`
	Difficulty    string   `json:"difficulty"`
	QuestionCount int      `json:"questionCount"`
	CreatedAt     string   `json:"createdAt"`
	AttemptCount  int      `json:"attemptCount"`
	LatestScore   *float64 `json:"latestScore"`
	MessageID     string   `json:"messageId"`
}

func toQuizSummaryDTO(quiz domainquiz.Quiz) QuizSummaryDTO {
	return QuizSummaryDTO{
		ID: quiz.ID, SessionID: quiz.SessionID, SessionTitle: quiz.SessionTitle, Difficulty: string(quiz.Difficulty),
		QuestionCount: len(quiz.Questions), CreatedAt: quiz.CreatedAt.Format(time.RFC3339), MessageID: quiz.MessageID,
	}
}

func toQuizSummaryDTOs(summaries []domainquiz.Summary) []QuizSummaryDTO {
	dtos := make([]QuizSummaryDTO, 0, len(summaries))
	for _, summary := range summaries {
		dtos = append(dtos, toQuizCardDTO(summary))
	}
	return dtos
}

func toQuizCardDTO(summary domainquiz.Summary) QuizSummaryDTO {
	return QuizSummaryDTO{
		ID: summary.ID, SessionID: summary.SessionID, SessionTitle: summary.SessionTitle,
		Difficulty: string(summary.Difficulty), QuestionCount: summary.QuestionCount,
		CreatedAt: summary.CreatedAt.Format(time.RFC3339), AttemptCount: summary.AttemptCount, LatestScore: summary.LatestScore,
		MessageID: summary.MessageID,
	}
}

// QuizQuestionDTO is a question as the user answers it: no correct option and no
// explanation, which come only with a result.
type QuizQuestionDTO struct {
	ID       string   `json:"id"`
	Position int      `json:"position"`
	Kind     string   `json:"kind"`
	Prompt   string   `json:"prompt"`
	Options  []string `json:"options"`
}

// QuizDTO is a quiz the user takes.
type QuizDTO struct {
	ID           string            `json:"id"`
	SessionTitle string            `json:"sessionTitle"`
	Difficulty   string            `json:"difficulty"`
	Questions    []QuizQuestionDTO `json:"questions"`
}

// QuizSelectionDTO is what the user chose for one question. A null selectedIndex is
// a blank answer.
type QuizSelectionDTO struct {
	QuestionID    string `json:"questionId"`
	SelectedIndex *int   `json:"selectedIndex"`
}

// QuizAttemptSummaryDTO is a graded attempt without its answers. FinishedAt is RFC
// 3339 and Score is on a 0-10 scale.
type QuizAttemptSummaryDTO struct {
	ID           string  `json:"id"`
	QuizID       string  `json:"quizId"`
	CorrectCount int     `json:"correctCount"`
	Total        int     `json:"total"`
	Score        float64 `json:"score"`
	FinishedAt   string  `json:"finishedAt"`
}

func toQuizAttemptSummaryDTO(attempt domainquiz.Attempt) QuizAttemptSummaryDTO {
	return QuizAttemptSummaryDTO{
		ID: attempt.ID, QuizID: attempt.QuizID, CorrectCount: attempt.CorrectCount, Total: attempt.Total,
		Score: attempt.Score, FinishedAt: attempt.FinishedAt.Format(time.RFC3339),
	}
}

// QuizResultQuestionDTO is a question with its answer key and the user's choice.
// SelectedIndex is null for a blank answer.
type QuizResultQuestionDTO struct {
	ID            string   `json:"id"`
	Position      int      `json:"position"`
	Kind          string   `json:"kind"`
	Prompt        string   `json:"prompt"`
	Options       []string `json:"options"`
	CorrectIndex  int      `json:"correctIndex"`
	Explanation   string   `json:"explanation"`
	SelectedIndex *int     `json:"selectedIndex"`
	Correct       bool     `json:"correct"`
}

// QuizAttemptResultDTO is a graded attempt with the whole quiz: the answer key, the
// explanations and what the user chose.
type QuizAttemptResultDTO struct {
	Attempt      QuizAttemptSummaryDTO   `json:"attempt"`
	SessionTitle string                  `json:"sessionTitle"`
	Difficulty   string                  `json:"difficulty"`
	Questions    []QuizResultQuestionDTO `json:"questions"`
}

// GenerateQuiz asks for a quiz about a studied session: count questions (0 for
// the default) of the given difficulty (blank for medium). Only one generation
// runs at a time.
func (a *App) GenerateQuiz(sessionID string, count int, difficulty string) (QuizSummaryDTO, error) {
	if a.quizGenerator == nil {
		return QuizSummaryDTO{}, quizUnavailable()
	}
	spec := domainquiz.Spec{Count: count, Difficulty: domainquiz.Difficulty(strings.TrimSpace(difficulty))}
	ctx, release, ok := a.quizGeneration.begin(a.ctx)
	if !ok {
		return QuizSummaryDTO{}, &QuizError{Code: quizCodeGenerationInProgress, Message: "a quiz is already being generated"}
	}
	defer release()
	quiz, err := a.quizGenerator.Generate(ctx, strings.TrimSpace(sessionID), spec)
	if err != nil {
		return QuizSummaryDTO{}, toQuizError(err)
	}
	return toQuizSummaryDTO(quiz), nil
}

// CancelQuizGeneration cancels the quiz generation in flight, which then fails
// with the cancelled code and stores nothing. With none running it does
// nothing.
func (a *App) CancelQuizGeneration() error {
	a.quizGeneration.cancel()
	return nil
}

// generationSlot lets one generation run at a time and be cancelled.
type generationSlot struct {
	mu       sync.Mutex
	cancelFn context.CancelFunc
}

// begin reserves the slot. It returns the context the generation runs under and
// the function that frees the slot, or false when a generation already runs.
func (s *generationSlot) begin(parent context.Context) (context.Context, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		return nil, nil, false
	}
	ctx, cancel := context.WithCancel(parent)
	s.cancelFn = cancel
	return ctx, func() {
		cancel()
		s.mu.Lock()
		s.cancelFn = nil
		s.mu.Unlock()
	}, true
}

// cancel stops the running generation, if any.
func (s *generationSlot) cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
}

// ListQuizzes returns every quiz, newest first, without its questions.
func (a *App) ListQuizzes() ([]QuizSummaryDTO, error) {
	if a.quizzes == nil {
		return nil, quizUnavailable()
	}
	summaries, err := a.quizzes.ListQuizzes(a.ctx)
	if err != nil {
		return nil, toQuizError(err)
	}
	return toQuizSummaryDTOs(summaries), nil
}

// GetQuiz returns a quiz to take. The answer key stays on the Go side.
func (a *App) GetQuiz(id string) (QuizDTO, error) {
	if a.quizzes == nil {
		return QuizDTO{}, quizUnavailable()
	}
	quiz, err := a.quizzes.GetQuiz(a.ctx, id)
	if err != nil {
		return QuizDTO{}, toQuizError(err)
	}
	dto := QuizDTO{
		ID: quiz.ID, SessionTitle: quiz.SessionTitle, Difficulty: string(quiz.Difficulty),
		Questions: make([]QuizQuestionDTO, 0, len(quiz.Questions)),
	}
	for _, question := range quiz.Questions {
		dto.Questions = append(dto.Questions, QuizQuestionDTO{
			ID: question.ID, Position: question.Position, Kind: string(question.Kind), Prompt: question.Prompt, Options: question.Options,
		})
	}
	return dto, nil
}

// SubmitQuizAttempt grades the user's answers and stores the attempt. A
// question left out, or with a null selectedIndex, is blank.
func (a *App) SubmitQuizAttempt(quizID string, selections []QuizSelectionDTO) (QuizAttemptSummaryDTO, error) {
	if a.quizzes == nil {
		return QuizAttemptSummaryDTO{}, quizUnavailable()
	}
	chosen := make([]domainquiz.Selection, len(selections))
	for i, selection := range selections {
		chosen[i] = domainquiz.Selection{QuestionID: selection.QuestionID, SelectedIndex: selection.SelectedIndex}
	}
	attempt, err := a.quizzes.SubmitAttempt(a.ctx, quizID, chosen)
	if err != nil {
		return QuizAttemptSummaryDTO{}, toQuizError(err)
	}
	return toQuizAttemptSummaryDTO(attempt), nil
}

// ListQuizAttempts returns the attempts of a quiz, newest first.
func (a *App) ListQuizAttempts(quizID string) ([]QuizAttemptSummaryDTO, error) {
	if a.quizzes == nil {
		return nil, quizUnavailable()
	}
	attempts, err := a.quizzes.ListAttempts(a.ctx, quizID)
	if err != nil {
		return nil, toQuizError(err)
	}
	dtos := make([]QuizAttemptSummaryDTO, 0, len(attempts))
	for _, attempt := range attempts {
		dtos = append(dtos, toQuizAttemptSummaryDTO(attempt))
	}
	return dtos, nil
}

// GetQuizAttemptResult returns a graded attempt with the answer key, the
// explanations and what the user chose, for every question of the quiz.
func (a *App) GetQuizAttemptResult(attemptID string) (QuizAttemptResultDTO, error) {
	if a.quizzes == nil {
		return QuizAttemptResultDTO{}, quizUnavailable()
	}
	result, err := a.quizzes.GetAttemptResult(a.ctx, attemptID)
	if err != nil {
		return QuizAttemptResultDTO{}, toQuizError(err)
	}
	answers := make(map[string]domainquiz.Answer, len(result.Attempt.Answers))
	for _, answer := range result.Attempt.Answers {
		answers[answer.QuestionID] = answer
	}
	dto := QuizAttemptResultDTO{
		Attempt:      toQuizAttemptSummaryDTO(result.Attempt),
		SessionTitle: result.Quiz.SessionTitle,
		Difficulty:   string(result.Quiz.Difficulty),
		Questions:    make([]QuizResultQuestionDTO, 0, len(result.Quiz.Questions)),
	}
	for _, question := range result.Quiz.Questions {
		answer := answers[question.ID]
		dto.Questions = append(dto.Questions, QuizResultQuestionDTO{
			ID: question.ID, Position: question.Position, Kind: string(question.Kind), Prompt: question.Prompt, Options: question.Options,
			CorrectIndex: question.CorrectIndex, Explanation: question.Explanation,
			SelectedIndex: answer.SelectedIndex, Correct: answer.Correct,
		})
	}
	return dto, nil
}

// DeleteQuiz removes a quiz with its attempts.
func (a *App) DeleteQuiz(id string) error {
	if a.quizzes == nil {
		return quizUnavailable()
	}
	if err := a.quizzes.DeleteQuiz(a.ctx, id); err != nil {
		return toQuizError(err)
	}
	return nil
}

// QuizOfferDTO is a pending quiz offer: it hangs under the assistant message
// MessageID of the study chat.
type QuizOfferDTO struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	MessageID string `json:"messageId"`
}

func quizInvalidInput(message string) *QuizError {
	return &QuizError{Code: quizCodeInvalidInput, Message: message}
}

// ListSessionQuizzes returns the quizzes the study chat's offers produced in the
// session, oldest first, without their questions: the cards the chat shows under
// the messages that offered them.
func (a *App) ListSessionQuizzes(sessionID string) ([]QuizSummaryDTO, error) {
	if a.quizzes == nil {
		return nil, quizUnavailable()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, quizInvalidInput("session id is required")
	}
	summaries, err := a.quizzes.ListSessionQuizzes(a.ctx, sessionID)
	if err != nil {
		return nil, toQuizError(err)
	}
	return toQuizSummaryDTOs(summaries), nil
}

// GetPendingQuizOffer returns the session's pending quiz offer, or null when
// there is none. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
func (a *App) GetPendingQuizOffer(sessionID string) (*QuizOfferDTO, error) {
	if a.quizOffers == nil {
		return nil, quizUnavailable()
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, quizInvalidInput("session id is required")
	}
	offer, found, err := a.quizOffers.GetPending(a.ctx, sessionID)
	if err != nil {
		return nil, toQuizError(err)
	}
	if !found {
		return nil, nil
	}
	return &QuizOfferDTO{ID: offer.ID, SessionID: offer.SessionID, MessageID: offer.MessageID}, nil
}

// eventQuizOfferResolved is emitted when an accepted offer finished: the quiz
// is saved and the offer is gone. A chat of that session that was mounted again
// meanwhile re-reads its offer and quiz cards.
const eventQuizOfferResolved = "quiz:offer-resolved"

// QuizOfferResolvedEvent is the payload of eventQuizOfferResolved.
type QuizOfferResolvedEvent struct {
	SessionID string `json:"sessionId"`
	OfferID   string `json:"offerId"`
}

// AcceptQuizOffer generates a quiz of count questions (0 for the default) and
// the given difficulty (blank for medium) for the offer's session, links it to
// the offer's message and returns its card. It shares the generation slot with
// GenerateQuiz, so CancelQuizGeneration cancels it. On failure the offer stays
// pending.
func (a *App) AcceptQuizOffer(offerID string, count int, difficulty string) (QuizSummaryDTO, error) {
	if a.quizOffers == nil {
		return QuizSummaryDTO{}, quizUnavailable()
	}
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return QuizSummaryDTO{}, quizInvalidInput("offer id is required")
	}
	spec := domainquiz.Spec{Count: count, Difficulty: domainquiz.Difficulty(strings.TrimSpace(difficulty))}
	ctx, release, ok := a.quizGeneration.begin(a.ctx)
	if !ok {
		return QuizSummaryDTO{}, &QuizError{Code: quizCodeGenerationInProgress, Message: "a quiz is already being generated"}
	}
	defer release()
	summary, err := a.quizOffers.Accept(ctx, offerID, spec)
	if err != nil {
		return QuizSummaryDTO{}, toQuizError(err)
	}
	// The call outlives the chat that made it, so the session is told apart from
	// the returned card, which only the original chat instance receives.
	a.emit(a.ctx, eventQuizOfferResolved, QuizOfferResolvedEvent{SessionID: summary.SessionID, OfferID: offerID})
	return toQuizCardDTO(summary), nil
}

// DeclineQuizOffer removes the offer. Nothing is created.
func (a *App) DeclineQuizOffer(offerID string) error {
	if a.quizOffers == nil {
		return quizUnavailable()
	}
	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return quizInvalidInput("offer id is required")
	}
	if err := a.quizOffers.Decline(a.ctx, offerID); err != nil {
		return toQuizError(err)
	}
	return nil
}
