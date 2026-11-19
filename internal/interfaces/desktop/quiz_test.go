package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationquiz "github.com/santaniello/athena/internal/application/quiz"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

var quizTime = time.Date(2026, 11, 16, 9, 30, 0, 0, time.UTC)

func quizCodeOf(t *testing.T, err error) QuizError {
	t.Helper()
	var coded *QuizError
	require.ErrorAs(t, err, &coded)
	var decoded QuizError
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
	assert.Equal(t, *coded, decoded)
	return decoded
}

func boundQuiz() domainquiz.Quiz {
	return domainquiz.Quiz{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: domainquiz.DifficultyHard, CreatedAt: quizTime,
		Questions: []domainquiz.Question{
			{ID: "q1", Position: 1, Kind: domainquiz.KindMultipleChoice, Prompt: "Which?", Options: []string{"A", "B", "C"}, CorrectIndex: 2, Explanation: "Because C."},
			{ID: "q2", Position: 2, Kind: domainquiz.KindTrueFalse, Prompt: "True?", Options: []string{"True", "False"}, CorrectIndex: 0, Explanation: "Yes."},
		},
	}
}

type fakeQuizGenerator struct {
	quiz      domainquiz.Quiz
	err       error
	sessionID string
	spec      domainquiz.Spec
}

func (g *fakeQuizGenerator) Generate(_ context.Context, sessionID string, spec domainquiz.Spec) (domainquiz.Quiz, error) {
	g.sessionID, g.spec = sessionID, spec
	return g.quiz, g.err
}

type blockingQuizGenerator struct{ started chan struct{} }

func (g *blockingQuizGenerator) Generate(ctx context.Context, _ string, _ domainquiz.Spec) (domainquiz.Quiz, error) {
	close(g.started)
	<-ctx.Done()
	return domainquiz.Quiz{}, fmt.Errorf("generating: %w", ctx.Err())
}

type fakeQuizzes struct {
	summaries  []domainquiz.Summary
	sessionID  string
	quiz       domainquiz.Quiz
	attempt    domainquiz.Attempt
	attempts   []domainquiz.Attempt
	result     applicationquiz.AttemptResult
	err        error
	quizID     string
	selections []domainquiz.Selection
}

func (q *fakeQuizzes) ListQuizzes(context.Context) ([]domainquiz.Summary, error) {
	return q.summaries, q.err
}

func (q *fakeQuizzes) ListSessionQuizzes(_ context.Context, sessionID string) ([]domainquiz.Summary, error) {
	q.sessionID = sessionID
	return q.summaries, q.err
}

func (q *fakeQuizzes) GetQuiz(_ context.Context, id string) (domainquiz.Quiz, error) {
	q.quizID = id
	return q.quiz, q.err
}

func (q *fakeQuizzes) SubmitAttempt(_ context.Context, quizID string, selections []domainquiz.Selection) (domainquiz.Attempt, error) {
	q.quizID, q.selections = quizID, selections
	return q.attempt, q.err
}

func (q *fakeQuizzes) ListAttempts(_ context.Context, quizID string) ([]domainquiz.Attempt, error) {
	q.quizID = quizID
	return q.attempts, q.err
}

func (q *fakeQuizzes) GetAttemptResult(_ context.Context, id string) (applicationquiz.AttemptResult, error) {
	q.quizID = id
	return q.result, q.err
}

func (q *fakeQuizzes) DeleteQuiz(_ context.Context, id string) error {
	q.quizID = id
	return q.err
}

func newQuizApp(generator quizGenerator, quizzes quizUseCases) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithQuizGenerationService(generator), WithQuizService(quizzes))
	app.ctx = context.Background()
	return app
}

func TestApp_GenerateQuiz_returnsTheSummaryOfTheNewQuiz(t *testing.T) {
	// Given a generator that creates a quiz
	generator := &fakeQuizGenerator{quiz: boundQuiz()}
	app := newQuizApp(generator, nil)

	// When the frontend generates 2 hard questions for a session
	got, err := app.GenerateQuiz(" session-1 ", 2, " HARD ")

	// Then the summary is returned and the use case got the trimmed id and the spec
	require.NoError(t, err)
	assert.Equal(t, QuizSummaryDTO{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: "HARD", QuestionCount: 2,
		CreatedAt: "2026-11-16T09:30:00Z",
	}, got)
	assert.Equal(t, "session-1", generator.sessionID)
	assert.Equal(t, domainquiz.Spec{Count: 2, Difficulty: domainquiz.DifficultyHard}, generator.spec)
}

func TestApp_GenerateQuiz_mapsUseCaseErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unknown session", fmt.Errorf("wrapped: %w", applicationquiz.ErrSourceSessionNotFound), quizCodeSessionNotFound},
		{"invalid model output", fmt.Errorf("wrapped: %w", applicationquiz.ErrInvalidGeneration), quizCodeInvalidGeneration},
		{"bad count", domainquiz.ErrQuestionCountInvalid, quizCodeInvalidInput},
		{"bad difficulty", domainquiz.ErrDifficultyInvalid, quizCodeInvalidInput},
		{"provider failure", errors.New("openrouter is down"), quizCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a generator that fails
			app := newQuizApp(&fakeQuizGenerator{err: tt.err}, nil)

			// When the frontend generates a quiz
			_, err := app.GenerateQuiz("session-1", 5, "EASY")

			// Then the error carries the stable code
			assert.Equal(t, tt.code, quizCodeOf(t, err).Code)
		})
	}
}

func TestApp_CancelQuizGeneration_stopsTheRunningGeneration(t *testing.T) {
	// Given a generation that is still running
	generator := &blockingQuizGenerator{started: make(chan struct{})}
	app := newQuizApp(generator, nil)
	result := make(chan error, 1)
	go func() {
		_, err := app.GenerateQuiz("session-1", 5, "EASY")
		result <- err
	}()
	<-generator.started

	// When the frontend cancels it
	cancelErr := app.CancelQuizGeneration()

	// Then the generation ends with the cancelled code
	require.NoError(t, cancelErr)
	assert.Equal(t, quizCodeCancelled, quizCodeOf(t, <-result).Code)
}

func TestApp_CancelQuizGeneration_withNothingRunningDoesNothing(t *testing.T) {
	app := newQuizApp(&fakeQuizGenerator{}, nil)

	assert.NoError(t, app.CancelQuizGeneration())
}

func TestApp_GenerateQuiz_refusesASecondGenerationWhileOneRuns(t *testing.T) {
	// Given a generation that is still running
	generator := &blockingQuizGenerator{started: make(chan struct{})}
	app := newQuizApp(generator, nil)
	first := make(chan error, 1)
	go func() {
		_, err := app.GenerateQuiz("session-1", 5, "EASY")
		first <- err
	}()
	<-generator.started

	// When the frontend starts another one
	_, err := app.GenerateQuiz("session-2", 5, "EASY")

	// Then it is refused and the first one is not disturbed
	assert.Equal(t, quizCodeGenerationInProgress, quizCodeOf(t, err).Code)
	require.NoError(t, app.CancelQuizGeneration())
	<-first
}

func TestApp_GenerateQuiz_canRunAgainAfterOneFinished(t *testing.T) {
	// Given a generation that failed
	generator := &fakeQuizGenerator{err: errors.New("provider down")}
	app := newQuizApp(generator, nil)
	_, err := app.GenerateQuiz("session-1", 5, "EASY")
	require.Error(t, err)

	// When the frontend retries
	generator.err = nil
	generator.quiz = boundQuiz()
	_, retryErr := app.GenerateQuiz("session-1", 5, "EASY")

	// Then the slot was freed
	assert.NoError(t, retryErr)
}

func TestApp_ListQuizzes_returnsSummaries(t *testing.T) {
	// Given two quizzes, one never taken and one with a latest score
	latest := 7.5
	quizzes := &fakeQuizzes{summaries: []domainquiz.Summary{
		{ID: "quiz-2", SessionID: "s", SessionTitle: "Go", Difficulty: domainquiz.DifficultyEasy, QuestionCount: 5, CreatedAt: quizTime},
		{ID: "quiz-1", SessionID: "s", SessionTitle: "Caching", Difficulty: domainquiz.DifficultyHard, QuestionCount: 10, CreatedAt: quizTime, AttemptCount: 3, LatestScore: &latest},
	}}
	app := newQuizApp(nil, quizzes)

	// When the frontend lists them
	got, err := app.ListQuizzes()

	// Then the null score marks a quiz never taken
	require.NoError(t, err)
	assert.Equal(t, []QuizSummaryDTO{
		{ID: "quiz-2", SessionID: "s", SessionTitle: "Go", Difficulty: "EASY", QuestionCount: 5, CreatedAt: "2026-11-16T09:30:00Z"},
		{ID: "quiz-1", SessionID: "s", SessionTitle: "Caching", Difficulty: "HARD", QuestionCount: 10, CreatedAt: "2026-11-16T09:30:00Z", AttemptCount: 3, LatestScore: &latest},
	}, got)
}

func TestApp_ListQuizzes_noQuizzesIsAnEmptyListNotNull(t *testing.T) {
	app := newQuizApp(nil, &fakeQuizzes{})

	got, err := app.ListQuizzes()

	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestApp_GetQuiz_hidesTheAnswerKey(t *testing.T) {
	// Given a stored quiz
	quizzes := &fakeQuizzes{quiz: boundQuiz()}
	app := newQuizApp(nil, quizzes)

	// When the frontend opens it to take it
	got, err := app.GetQuiz("quiz-1")

	// Then it gets the questions and options but neither the correct option nor the explanation
	require.NoError(t, err)
	assert.Equal(t, QuizDTO{
		ID: "quiz-1", SessionTitle: "Caching", Difficulty: "HARD",
		Questions: []QuizQuestionDTO{
			{ID: "q1", Position: 1, Kind: "MULTIPLE_CHOICE", Prompt: "Which?", Options: []string{"A", "B", "C"}},
			{ID: "q2", Position: 2, Kind: "TRUE_FALSE", Prompt: "True?", Options: []string{"True", "False"}},
		},
	}, got)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "orrect")
	assert.NotContains(t, string(encoded), "xplanation")
	assert.Equal(t, "quiz-1", quizzes.quizID)
}

func TestApp_SubmitQuizAttempt_gradesTheSelections(t *testing.T) {
	// Given a use case that grades an attempt
	two := 2
	quizzes := &fakeQuizzes{attempt: domainquiz.Attempt{ID: "attempt-1", QuizID: "quiz-1", CorrectCount: 1, Total: 2, Score: 5, FinishedAt: quizTime}}
	app := newQuizApp(nil, quizzes)

	// When the frontend submits one answer and one blank
	got, err := app.SubmitQuizAttempt("quiz-1", []QuizSelectionDTO{{QuestionID: "q1", SelectedIndex: &two}, {QuestionID: "q2"}})

	// Then the use case got the selections as the domain's and the summary comes back
	require.NoError(t, err)
	assert.Equal(t, QuizAttemptSummaryDTO{ID: "attempt-1", QuizID: "quiz-1", CorrectCount: 1, Total: 2, Score: 5, FinishedAt: "2026-11-16T09:30:00Z"}, got)
	assert.Equal(t, "quiz-1", quizzes.quizID)
	assert.Equal(t, []domainquiz.Selection{{QuestionID: "q1", SelectedIndex: &two}, {QuestionID: "q2"}}, quizzes.selections)
}

func TestApp_ListQuizAttempts_returnsSummariesNewestFirst(t *testing.T) {
	quizzes := &fakeQuizzes{attempts: []domainquiz.Attempt{
		{ID: "a2", QuizID: "quiz-1", CorrectCount: 2, Total: 2, Score: 10, FinishedAt: quizTime},
		{ID: "a1", QuizID: "quiz-1", CorrectCount: 0, Total: 2, Score: 0, FinishedAt: quizTime.Add(-time.Hour)},
	}}
	app := newQuizApp(nil, quizzes)

	got, err := app.ListQuizAttempts("quiz-1")

	require.NoError(t, err)
	assert.Equal(t, []QuizAttemptSummaryDTO{
		{ID: "a2", QuizID: "quiz-1", CorrectCount: 2, Total: 2, Score: 10, FinishedAt: "2026-11-16T09:30:00Z"},
		{ID: "a1", QuizID: "quiz-1", CorrectCount: 0, Total: 2, Score: 0, FinishedAt: "2026-11-16T08:30:00Z"},
	}, got)
	assert.Equal(t, "quiz-1", quizzes.quizID)
}

func TestApp_ListQuizAttempts_noneIsAnEmptyListNotNull(t *testing.T) {
	got, err := newQuizApp(nil, &fakeQuizzes{}).ListQuizAttempts("quiz-1")

	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestApp_GetQuizAttemptResult_showsTheAnswerKeyAndWhatTheUserChose(t *testing.T) {
	// Given a graded attempt: the first answer wrong, the second left blank
	zero := 0
	quizzes := &fakeQuizzes{result: applicationquiz.AttemptResult{
		Quiz: boundQuiz(),
		Attempt: domainquiz.Attempt{
			ID: "attempt-1", QuizID: "quiz-1", CorrectCount: 0, Total: 2, Score: 0, FinishedAt: quizTime,
			Answers: []domainquiz.Answer{{QuestionID: "q1", SelectedIndex: &zero}, {QuestionID: "q2"}},
		},
	}}
	app := newQuizApp(nil, quizzes)

	// When the frontend opens the result
	got, err := app.GetQuizAttemptResult("attempt-1")

	// Then each question carries its key, explanation and the user's choice
	require.NoError(t, err)
	assert.Equal(t, QuizAttemptResultDTO{
		Attempt:      QuizAttemptSummaryDTO{ID: "attempt-1", QuizID: "quiz-1", Total: 2, FinishedAt: "2026-11-16T09:30:00Z"},
		SessionTitle: "Caching", Difficulty: "HARD",
		Questions: []QuizResultQuestionDTO{
			{ID: "q1", Position: 1, Kind: "MULTIPLE_CHOICE", Prompt: "Which?", Options: []string{"A", "B", "C"}, CorrectIndex: 2, Explanation: "Because C.", SelectedIndex: &zero},
			{ID: "q2", Position: 2, Kind: "TRUE_FALSE", Prompt: "True?", Options: []string{"True", "False"}, CorrectIndex: 0, Explanation: "Yes."},
		},
	}, got)
	assert.Equal(t, "attempt-1", quizzes.quizID)
}

func TestApp_GetQuizAttemptResult_listsEveryQuestionEvenWhenAnAnswerIsMissing(t *testing.T) {
	// Given an attempt that holds an answer for only one of the two questions
	quizzes := &fakeQuizzes{result: applicationquiz.AttemptResult{
		Quiz:    boundQuiz(),
		Attempt: domainquiz.Attempt{ID: "a", Answers: []domainquiz.Answer{{QuestionID: "q1", Correct: true}}},
	}}

	// When the result is opened
	got, err := newQuizApp(nil, quizzes).GetQuizAttemptResult("a")

	// Then every question of the quiz is listed, the unanswered ones as blank
	require.NoError(t, err)
	assert.Len(t, got.Questions, 2)
	assert.True(t, got.Questions[0].Correct)
	assert.False(t, got.Questions[1].Correct)
}

func TestApp_DeleteQuiz_passesTheID(t *testing.T) {
	quizzes := &fakeQuizzes{}
	app := newQuizApp(nil, quizzes)

	require.NoError(t, app.DeleteQuiz("quiz-1"))

	assert.Equal(t, "quiz-1", quizzes.quizID)
}

func TestApp_quizBindings_mapUseCaseErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unknown quiz", fmt.Errorf("w: %w", domainquiz.ErrQuizNotFound), quizCodeQuizNotFound},
		{"unknown attempt", fmt.Errorf("w: %w", domainquiz.ErrAttemptNotFound), quizCodeAttemptNotFound},
		{"unknown question", domainquiz.ErrSelectionUnknownQuestion, quizCodeSelectionInvalid},
		{"answered twice", domainquiz.ErrSelectionDuplicate, quizCodeSelectionInvalid},
		{"option out of range", domainquiz.ErrSelectionOutOfRange, quizCodeSelectionInvalid},
		{"anything else", errors.New("disk on fire"), quizCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newQuizApp(nil, &fakeQuizzes{err: tt.err})

			_, listErr := app.ListQuizzes()
			_, getErr := app.GetQuiz("q")
			_, submitErr := app.SubmitQuizAttempt("q", nil)
			_, attemptsErr := app.ListQuizAttempts("q")
			_, resultErr := app.GetQuizAttemptResult("a")
			deleteErr := app.DeleteQuiz("q")
			_, sessionErr := app.ListSessionQuizzes("s")

			for _, err := range []error{listErr, getErr, submitErr, attemptsErr, resultErr, deleteErr, sessionErr} {
				assert.Equal(t, tt.code, quizCodeOf(t, err).Code)
			}
		})
	}
}

func TestQuizError_errorIsJSON(t *testing.T) {
	err := &QuizError{Code: "x", Message: "y"}

	assert.JSONEq(t, `{"code":"x","message":"y"}`, err.Error())
}

func TestApp_ListSessionQuizzes_returnsTheCardsOfTheSession(t *testing.T) {
	// Given a session with an offer quiz that was taken once
	latest := 7.5
	quizzes := &fakeQuizzes{summaries: []domainquiz.Summary{{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: domainquiz.DifficultyHard,
		QuestionCount: 8, CreatedAt: quizTime, AttemptCount: 2, LatestScore: &latest, MessageID: "message-1",
	}}}
	app := newQuizApp(nil, quizzes)

	// When listing the session's quizzes
	got, err := app.ListSessionQuizzes(" session-1 ")

	// Then the card carries the message it hangs under and the attempt data
	require.NoError(t, err)
	assert.Equal(t, "session-1", quizzes.sessionID)
	assert.Equal(t, []QuizSummaryDTO{{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: "HARD", QuestionCount: 8,
		CreatedAt: quizTime.Format(time.RFC3339), AttemptCount: 2, LatestScore: &latest, MessageID: "message-1",
	}}, got)
}

func TestApp_ListSessionQuizzes_noQuizzesIsAnEmptyListNotNull(t *testing.T) {
	got, err := newQuizApp(nil, &fakeQuizzes{}).ListSessionQuizzes("session-1")

	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestApp_ListSessionQuizzes_requiresASession(t *testing.T) {
	_, err := newQuizApp(nil, &fakeQuizzes{}).ListSessionQuizzes("  ")

	assert.Equal(t, quizCodeInvalidInput, quizCodeOf(t, err).Code)
}

type fakeQuizOffers struct {
	offer     domainquiz.Offer
	found     bool
	summary   domainquiz.Summary
	err       error
	offerID   string
	sessionID string
	spec      domainquiz.Spec
}

func (o *fakeQuizOffers) GetPending(_ context.Context, sessionID string) (domainquiz.Offer, bool, error) {
	o.sessionID = sessionID
	return o.offer, o.found, o.err
}

func (o *fakeQuizOffers) Accept(_ context.Context, offerID string, spec domainquiz.Spec) (domainquiz.Summary, error) {
	o.offerID, o.spec = offerID, spec
	return o.summary, o.err
}

func (o *fakeQuizOffers) Decline(_ context.Context, offerID string) error {
	o.offerID = offerID
	return o.err
}

type blockingQuizOffers struct {
	fakeQuizOffers
	started chan struct{}
}

func (o *blockingQuizOffers) Accept(ctx context.Context, _ string, _ domainquiz.Spec) (domainquiz.Summary, error) {
	close(o.started)
	<-ctx.Done()
	return domainquiz.Summary{}, fmt.Errorf("generating: %w", ctx.Err())
}

func newQuizOfferApp(offers quizOffers) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithQuizOfferService(offers))
	app.ctx = context.Background()
	// The real emitter needs the Wails runtime context; tests observe or ignore events.
	app.emit = func(context.Context, string, ...interface{}) {}
	return app
}

func TestApp_GetPendingQuizOffer_returnsTheOfferOrNull(t *testing.T) {
	t.Run("a pending offer", func(t *testing.T) {
		// Given a session with a pending offer
		offers := &fakeQuizOffers{found: true, offer: domainquiz.Offer{ID: "offer-1", SessionID: "session-1", MessageID: "message-1"}}

		// When asking for it
		got, err := newQuizOfferApp(offers).GetPendingQuizOffer(" session-1 ")

		// Then it is returned for that session
		require.NoError(t, err)
		assert.Equal(t, "session-1", offers.sessionID)
		assert.Equal(t, &QuizOfferDTO{ID: "offer-1", SessionID: "session-1", MessageID: "message-1"}, got)
	})
	t.Run("none", func(t *testing.T) {
		got, err := newQuizOfferApp(&fakeQuizOffers{}).GetPendingQuizOffer("session-1")

		require.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("a blank session", func(t *testing.T) {
		_, err := newQuizOfferApp(&fakeQuizOffers{}).GetPendingQuizOffer(" ")

		assert.Equal(t, quizCodeInvalidInput, quizCodeOf(t, err).Code)
	})
}

func TestApp_AcceptQuizOffer_returnsTheQuizCard(t *testing.T) {
	// Given an offer that generates a quiz
	offers := &fakeQuizOffers{summary: domainquiz.Summary{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: domainquiz.DifficultyHard,
		QuestionCount: 8, CreatedAt: quizTime, MessageID: "message-1",
	}}

	// When accepting it with the user's choice
	got, err := newQuizOfferApp(offers).AcceptQuizOffer(" offer-1 ", 8, " HARD ")

	// Then the use case got the choice and the card is returned
	require.NoError(t, err)
	assert.Equal(t, "offer-1", offers.offerID)
	assert.Equal(t, domainquiz.Spec{Count: 8, Difficulty: domainquiz.DifficultyHard}, offers.spec)
	assert.Equal(t, QuizSummaryDTO{
		ID: "quiz-1", SessionID: "session-1", SessionTitle: "Caching", Difficulty: "HARD", QuestionCount: 8,
		CreatedAt: quizTime.Format(time.RFC3339), MessageID: "message-1",
	}, got)
}

func TestApp_AcceptQuizOffer_announcesTheResolvedOfferToTheSession(t *testing.T) {
	observe := func(app *App) *[]emittedEvent {
		var emitted []emittedEvent
		app.emit = func(_ context.Context, name string, data ...interface{}) {
			emitted = append(emitted, emittedEvent{name: name, data: data})
		}
		return &emitted
	}
	t.Run("a saved quiz", func(t *testing.T) {
		// Given an offer that produces a quiz and an observed emitter
		app := newQuizOfferApp(&fakeQuizOffers{summary: domainquiz.Summary{ID: "quiz-1", SessionID: "session-1"}})
		emitted := observe(app)

		// When accepting it
		_, err := app.AcceptQuizOffer("offer-1", 5, "EASY")

		// Then the session is told, so a chat remounted meanwhile re-reads the offer and the cards
		require.NoError(t, err)
		require.Len(t, *emitted, 1)
		assert.Equal(t, eventQuizOfferResolved, (*emitted)[0].name)
		assert.Equal(t, []interface{}{QuizOfferResolvedEvent{SessionID: "session-1", OfferID: "offer-1"}}, (*emitted)[0].data)
	})
	t.Run("a failed accept", func(t *testing.T) {
		// Given an offer whose generation fails
		app := newQuizOfferApp(&fakeQuizOffers{err: errors.New("boom")})
		emitted := observe(app)

		// When accepting it
		_, err := app.AcceptQuizOffer("offer-1", 5, "EASY")

		// Then nothing is announced: the offer stays pending
		require.Error(t, err)
		assert.Empty(t, *emitted)
	})
}

func TestApp_AcceptQuizOffer_requiresAnOffer(t *testing.T) {
	_, err := newQuizOfferApp(&fakeQuizOffers{}).AcceptQuizOffer("  ", 5, "EASY")

	assert.Equal(t, quizCodeInvalidInput, quizCodeOf(t, err).Code)
}

func TestApp_AcceptQuizOffer_canBeCancelledLikeAGeneration(t *testing.T) {
	// Given an accept whose generation is still running
	offers := &blockingQuizOffers{started: make(chan struct{})}
	app := newQuizOfferApp(offers)
	result := make(chan error, 1)
	go func() {
		_, err := app.AcceptQuizOffer("offer-1", 5, "EASY")
		result <- err
	}()
	<-offers.started

	// When the frontend cancels the quiz generation
	require.NoError(t, app.CancelQuizGeneration())

	// Then the accept ends with the cancelled code
	assert.Equal(t, quizCodeCancelled, quizCodeOf(t, <-result).Code)
}

func TestApp_AcceptQuizOffer_refusesWhileAQuizIsBeingGenerated(t *testing.T) {
	// Given a generation from the menu that is still running
	generator := &blockingQuizGenerator{started: make(chan struct{})}
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		WithQuizGenerationService(generator), WithQuizOfferService(&fakeQuizOffers{}))
	app.ctx = context.Background()
	go func() { _, _ = app.GenerateQuiz("session-1", 5, "EASY") }()
	<-generator.started

	// When an offer is accepted meanwhile
	_, err := app.AcceptQuizOffer("offer-1", 5, "EASY")

	// Then it is refused, and so is the reverse
	assert.Equal(t, quizCodeGenerationInProgress, quizCodeOf(t, err).Code)
	require.NoError(t, app.CancelQuizGeneration())
}

func TestApp_DeclineQuizOffer_passesTheID(t *testing.T) {
	offers := &fakeQuizOffers{}

	err := newQuizOfferApp(offers).DeclineQuizOffer(" offer-1 ")

	require.NoError(t, err)
	assert.Equal(t, "offer-1", offers.offerID)
}

func TestApp_DeclineQuizOffer_requiresAnOffer(t *testing.T) {
	err := newQuizOfferApp(&fakeQuizOffers{}).DeclineQuizOffer(" ")

	assert.Equal(t, quizCodeInvalidInput, quizCodeOf(t, err).Code)
}

func TestApp_quizOfferBindings_mapUseCaseErrorsToCodes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"offer gone", fmt.Errorf("w: %w", domainquiz.ErrOfferNotFound), quizCodeOfferNotFound},
		{"offer pending", domainquiz.ErrOfferPending, quizCodeOfferPending},
		{"bad question count", domainquiz.ErrQuestionCountInvalid, quizCodeInvalidInput},
		{"invalid generation", applicationquiz.ErrInvalidGeneration, quizCodeInvalidGeneration},
		{"anything else", errors.New("disk on fire"), quizCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newQuizOfferApp(&fakeQuizOffers{err: tt.err})

			_, pendingErr := app.GetPendingQuizOffer("s")
			_, acceptErr := app.AcceptQuizOffer("o", 5, "EASY")
			declineErr := app.DeclineQuizOffer("o")

			for _, err := range []error{pendingErr, acceptErr, declineErr} {
				assert.Equal(t, tt.code, quizCodeOf(t, err).Code)
			}
		})
	}
}

func TestApp_quizBindings_reportUnavailableWhenTheirServiceIsNotWired(t *testing.T) {
	// Given an app composed without any quiz service
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.ctx = context.Background()

	calls := map[string]func() error{
		"GenerateQuiz":         func() error { _, err := app.GenerateQuiz("s", 0, ""); return err },
		"ListQuizzes":          func() error { _, err := app.ListQuizzes(); return err },
		"GetQuiz":              func() error { _, err := app.GetQuiz("q"); return err },
		"SubmitQuizAttempt":    func() error { _, err := app.SubmitQuizAttempt("q", nil); return err },
		"ListQuizAttempts":     func() error { _, err := app.ListQuizAttempts("q"); return err },
		"GetQuizAttemptResult": func() error { _, err := app.GetQuizAttemptResult("a"); return err },
		"DeleteQuiz":           func() error { return app.DeleteQuiz("q") },
		"ListSessionQuizzes":   func() error { _, err := app.ListSessionQuizzes("s"); return err },
		"GetPendingQuizOffer":  func() error { _, err := app.GetPendingQuizOffer("s"); return err },
		"AcceptQuizOffer":      func() error { _, err := app.AcceptQuizOffer("o", 0, ""); return err },
		"DeclineQuizOffer":     func() error { return app.DeclineQuizOffer("o") },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			// When calling the binding, Then it answers with the unavailable code instead of panicking
			var quizErr *QuizError
			require.ErrorAs(t, call(), &quizErr)
			assert.Equal(t, quizCodeUnavailable, quizErr.Code)
		})
	}
}
