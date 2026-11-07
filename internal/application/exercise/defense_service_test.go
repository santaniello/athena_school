package exercise

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type defenseFixture struct {
	service     *DefenseService
	challenges  *domainmocks.MockChallengeRepository
	rubrics     *domainmocks.MockRubricRepository
	attempts    *domainmocks.MockAttemptRepository
	submissions *domainmocks.MockSubmissionRepository
	evaluations *domainmocks.MockEvaluationRepository
	defense     *domainmocks.MockDefenseRepository
	generator   *domainmocks.MockQuestionGenerator
	evaluator   *domainmocks.MockUnderstandingEvaluator
	tx          *exercisemocks.MockTransactor
	clock       *fakeClock
}

func newDefenseFixture(t *testing.T) *defenseFixture {
	t.Helper()
	f := &defenseFixture{
		challenges:  domainmocks.NewMockChallengeRepository(t),
		rubrics:     domainmocks.NewMockRubricRepository(t),
		attempts:    domainmocks.NewMockAttemptRepository(t),
		submissions: domainmocks.NewMockSubmissionRepository(t),
		evaluations: domainmocks.NewMockEvaluationRepository(t),
		defense:     domainmocks.NewMockDefenseRepository(t),
		generator:   domainmocks.NewMockQuestionGenerator(t),
		evaluator:   domainmocks.NewMockUnderstandingEvaluator(t),
		tx:          exercisemocks.NewMockTransactor(t),
		clock:       &fakeClock{now: fixedNow},
	}
	f.service = NewDefenseService(f.challenges, f.rubrics, f.attempts, f.submissions, f.evaluations, f.defense,
		f.generator, f.evaluator, f.tx, f.clock)
	f.service.newID = func() string { return "turn-id" }
	return f
}

func defenseChallenge() domainexercise.Challenge {
	challenge := attemptChallenge()
	challenge.DefenseQuestions = 3
	return challenge
}

// defenseRubric has four criteria of weight 25; c1 is mandatory.
func defenseRubric() domainexercise.RubricVersion {
	return domainexercise.RubricVersion{
		ID: "rubric-2", ChallengeID: "challenge-1", Version: 2,
		Criteria: []domainexercise.RubricCriterion{
			{ID: "c1", Key: "architecture", Name: "Architecture", Weight: 25, Mandatory: true},
			{ID: "c2", Key: "scalability", Name: "Scalability", Weight: 25},
			{ID: "c3", Key: "consistency", Name: "Consistency", Weight: 25},
			{ID: "c4", Key: "resilience", Name: "Resilience", Weight: 25},
		},
	}
}

// defenseDiagram flags c1 and judges c2, c3 and c4 at levels 3, 1 and 2, so the
// three questions target c1, c3 and c4 in that order.
func defenseDiagram() domainexercise.Evaluation {
	return domainexercise.Evaluation{
		ID: "diagram-1", AttemptID: "attempt-1", Kind: domainexercise.EvaluationKindDiagram, Score: 55,
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", InsufficientEvidence: true, Justification: "not shown"},
			{CriterionID: "c2", Level: 3, Justification: "ok"},
			{CriterionID: "c3", Level: 1, Justification: "weak"},
			{CriterionID: "c4", Level: 2, Justification: "fair"},
		},
	}
}

func defenseAttempt(state domainexercise.AttemptState) domainexercise.ChallengeAttempt {
	return domainexercise.ChallengeAttempt{
		ID: "attempt-1", ChallengeID: "challenge-1", RubricVersionID: "rubric-2", State: state, PassScore: 80,
	}
}

var defenseSubmission = domainexercise.Submission{ID: "submission-1", AttemptID: "attempt-1", ContentType: "drawio-xml", Content: "<mxfile/>"}

func askedTurn(position int, criterionID, question, answer string) domainexercise.DefenseTurn {
	turn := domainexercise.DefenseTurn{
		ID: "turn-id", AttemptID: "attempt-1", Position: position, CriterionID: criterionID,
		Question: question, AskedAt: fixedNow.Add(-time.Minute),
	}
	if answer != "" {
		turn.Answer = answer
		turn.AnsweredAt = fixedNow.Add(-30 * time.Second)
	}
	return turn
}

// expectContext makes the service read the attempt, challenge, rubric,
// submission, diagram evaluation and turns.
func (f *defenseFixture) expectContext(ctx context.Context, state domainexercise.AttemptState, turns []domainexercise.DefenseTurn) {
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(state), nil)
	f.expectReads(ctx, turns)
}

func (f *defenseFixture) expectReads(ctx context.Context, turns []domainexercise.DefenseTurn) {
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(defenseChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(defenseDiagram(), nil)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return(turns, nil)
}

func (f *defenseFixture) expectSubmission(ctx context.Context) {
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(defenseSubmission, nil)
}

func (f *defenseFixture) runTx(ctx context.Context) {
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func expectedQuestionInput(position int, criterion domainexercise.RubricCriterion, previous []domainexercise.DefenseTurn) domainexercise.QuestionInput {
	if previous == nil {
		previous = []domainexercise.DefenseTurn{}
	}
	return domainexercise.QuestionInput{
		Challenge: defenseChallenge(), Rubric: defenseRubric(), Submission: defenseSubmission, Diagram: defenseDiagram(),
		Criterion: criterion, Position: position, Total: 3, Previous: previous,
	}
}

func TestDefenseService_NextQuestion_firstCallStartsTheDefenseAndAsksAboutAFlaggedMandatoryCriterion(t *testing.T) {
	// Given an evaluated attempt with no question yet
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptEvaluated, []domainexercise.DefenseTurn{})
	f.expectSubmission(ctx)
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), expectedQuestionInput(1, defenseRubric().Criteria[0], nil)).
		Return("  Where does the cache live?  ", nil)
	f.runTx(ctx)
	f.attempts.EXPECT().StartDefense(ctx, "attempt-1").Return(nil)
	asked := askedTurn(1, "c1", "Where does the cache live?", "")
	asked.AskedAt = fixedNow
	f.defense.EXPECT().AddQuestion(ctx, asked).Return(nil)

	// When asking for the next question
	turn, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then the defense started and question 1 targets the flagged mandatory criterion
	require.NoError(t, err)
	assert.Equal(t, asked, turn)
}

func TestDefenseService_NextQuestion_laterQuestionsFollowTheAnswersAndTheSelectionOrder(t *testing.T) {
	// Given a defending attempt whose first question was answered
	f := newDefenseFixture(t)
	ctx := context.Background()
	previous := []domainexercise.DefenseTurn{askedTurn(1, "c1", "Where does the cache live?", "In Redis.")}
	f.expectContext(ctx, domainexercise.AttemptDefending, previous)
	f.expectSubmission(ctx)
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), expectedQuestionInput(2, defenseRubric().Criteria[2], previous)).
		Return("How do you keep it consistent?", nil)
	asked := askedTurn(2, "c3", "How do you keep it consistent?", "")
	asked.AskedAt = fixedNow
	f.defense.EXPECT().AddQuestion(ctx, asked).Return(nil)

	// When asking for the next question
	turn, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then it targets the next criterion in order, without starting the defense again
	require.NoError(t, err)
	assert.Equal(t, asked, turn)
}

func TestDefenseService_NextQuestion_returnsThePendingQuestionWithoutCallingTheModel(t *testing.T) {
	// Given a defending attempt whose last question has no answer yet
	f := newDefenseFixture(t)
	ctx := context.Background()
	pending := askedTurn(1, "c1", "Where does the cache live?", "")
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{pending})

	// When asking for the next question again, as a retried screen would
	turn, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then the same question comes back and no model call is spent
	require.NoError(t, err)
	assert.Equal(t, pending, turn)
}

func TestDefenseService_NextQuestion_endsAfterTheLastAnswer(t *testing.T) {
	// Given a defending attempt with all three questions answered
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{
		askedTurn(1, "c1", "q1", "a1"), askedTurn(2, "c3", "q2", "a2"), askedTurn(3, "c4", "q3", "a3"),
	})

	// When asking for another question
	_, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then there is none
	assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
}

func TestDefenseService_NextQuestion_refusesAttemptsThatAreNotWaitingForTheDefense(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{
		domainexercise.AttemptInProgress, domainexercise.AttemptSubmitted, domainexercise.AttemptEvaluationFailed, domainexercise.AttemptFinished,
	} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt in a state the defense cannot run from
			f := newDefenseFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(state), nil)

			// When asking for a question
			_, err := f.service.NextQuestion(ctx, "attempt-1")

			// Then it is refused before anything else is read
			assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
		})
	}
}

func TestDefenseService_NextQuestion_failedGenerationStoresNothingAndCanBeRetried(t *testing.T) {
	// Given an evaluated attempt and a model that fails once
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptEvaluated), nil)
	f.expectReads(ctx, []domainexercise.DefenseTurn{})
	f.expectSubmission(ctx)
	modelErr := errors.New("model timeout")
	input := expectedQuestionInput(1, defenseRubric().Criteria[0], nil)
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), input).Return("", modelErr).Once()

	// When asking
	_, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then the error comes back and neither the state nor a turn was written
	assert.ErrorIs(t, err, modelErr)
	f.attempts.AssertNotCalled(t, "StartDefense", mock.Anything, mock.Anything)
	f.defense.AssertNotCalled(t, "AddQuestion", mock.Anything, mock.Anything)

	// And when the same call is repeated, the lock was released and the model is asked again
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), input).Return("Where does the cache live?", nil).Once()
	f.runTx(ctx)
	f.attempts.EXPECT().StartDefense(ctx, "attempt-1").Return(nil)
	f.defense.EXPECT().AddQuestion(ctx, mock.MatchedBy(func(turn domainexercise.DefenseTurn) bool { return turn.Position == 1 })).Return(nil)
	turn, err := f.service.NextQuestion(ctx, "attempt-1")
	require.NoError(t, err)
	assert.Equal(t, "Where does the cache live?", turn.Question)
}

func TestDefenseService_NextQuestion_refusesAnEmptyQuestion(t *testing.T) {
	// Given a model that answers with blank text
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{})
	f.expectSubmission(ctx)
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), expectedQuestionInput(1, defenseRubric().Criteria[0], nil)).Return("  \n ", nil)

	// When asking
	_, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then nothing blank is stored
	assert.ErrorIs(t, err, ErrEmptyQuestion)
}

func TestDefenseService_NextQuestion_reportsAnUnknownAttempt(t *testing.T) {
	// Given no such attempt
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(domainexercise.ChallengeAttempt{}, domainexercise.ErrAttemptNotFound)

	// When asking for a question
	_, err := f.service.NextQuestion(ctx, "attempt-1")

	// Then it is not found
	assert.ErrorIs(t, err, domainexercise.ErrAttemptNotFound)
}

func TestDefenseService_NextQuestion_concurrentCallsAskTheModelOnlyOnce(t *testing.T) {
	// Given a store that remembers the question the first call stores
	f := newDefenseFixture(t)
	ctx := context.Background()
	var mu sync.Mutex
	stored := []domainexercise.DefenseTurn{}
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptDefending), nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(defenseChallenge(), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(defenseDiagram(), nil)
	f.submissions.EXPECT().GetByAttempt(ctx, "attempt-1").Return(defenseSubmission, nil)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").RunAndReturn(func(context.Context, string) ([]domainexercise.DefenseTurn, error) {
		mu.Lock()
		defer mu.Unlock()
		return append([]domainexercise.DefenseTurn{}, stored...), nil
	})
	f.defense.EXPECT().AddQuestion(ctx, mock.MatchedBy(func(domainexercise.DefenseTurn) bool { return true })).
		RunAndReturn(func(_ context.Context, turn domainexercise.DefenseTurn) error {
			mu.Lock()
			defer mu.Unlock()
			stored = append(stored, turn)
			return nil
		})
	release := make(chan struct{})
	f.generator.EXPECT().GenerateQuestion(withinTimeout(), expectedQuestionInput(1, defenseRubric().Criteria[0], nil)).
		RunAndReturn(func(context.Context, domainexercise.QuestionInput) (string, error) {
			<-release
			return "Where does the cache live?", nil
		}).Once()

	// When two calls for the same attempt run at once
	results := make(chan domainexercise.DefenseTurn, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			turn, err := f.service.NextQuestion(ctx, "attempt-1")
			assert.NoError(t, err)
			results <- turn
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	close(results)

	// Then both get the same stored question and the model was called once
	first, second := <-results, <-results
	assert.Equal(t, first, second)
	assert.Equal(t, "Where does the cache live?", first.Question)
}

func TestDefenseService_Answer_storesTheTrimmedAnswerAndReportsTheDefenseIsNotComplete(t *testing.T) {
	// Given a defending attempt with its first question waiting
	f := newDefenseFixture(t)
	ctx := context.Background()
	pending := askedTurn(1, "c1", "Where does the cache live?", "")
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{pending})
	f.defense.EXPECT().SaveAnswer(ctx, "turn-id", "In Redis.", fixedNow).Return(nil)

	// When answering with surrounding whitespace
	view, err := f.service.Answer(ctx, "attempt-1", "  In Redis.\n")

	// Then the answer is stored trimmed and the defense goes on
	require.NoError(t, err)
	assert.Equal(t, 3, view.Total)
	assert.False(t, view.Complete)
	require.Len(t, view.Turns, 1)
	assert.Equal(t, "In Redis.", view.Turns[0].Answer)
	assert.Equal(t, fixedNow, view.Turns[0].AnsweredAt)
}

func TestDefenseService_Answer_theLastAnswerCompletesTheDefense(t *testing.T) {
	// Given a defending attempt whose third question waits
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{
		askedTurn(1, "c1", "q1", "a1"), askedTurn(2, "c3", "q2", "a2"), askedTurn(3, "c4", "q3", ""),
	})
	f.defense.EXPECT().SaveAnswer(ctx, "turn-id", domainexercise.IDontKnowAnswer, fixedNow).Return(nil)

	// When the user answers "I don't know"
	view, err := f.service.Answer(ctx, "attempt-1", domainexercise.IDontKnowAnswer)

	// Then the answer is kept and the defense is complete
	require.NoError(t, err)
	assert.True(t, view.Complete)
	assert.Len(t, view.Turns, 3)
}

func TestDefenseService_Answer_rejectsAnInvalidAnswerBeforeReadingAnything(t *testing.T) {
	for name, answer := range map[string]string{"blank": "   ", "too long": strings.Repeat("a", domainexercise.MaxAnswerLength+1)} {
		t.Run(name, func(t *testing.T) {
			// Given an answer that is blank or over 4,000 characters
			f := newDefenseFixture(t)

			// When answering
			_, err := f.service.Answer(context.Background(), "attempt-1", answer)

			// Then it is invalid
			assert.ErrorIs(t, err, domainexercise.ErrInvalidAnswer)
		})
	}
}

func TestDefenseService_Answer_failsWhenNoQuestionIsWaiting(t *testing.T) {
	// Given a defending attempt whose only question is already answered
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptDefending), nil)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.DefenseTurn{askedTurn(1, "c1", "q1", "a1")}, nil)

	// When answering again
	_, err := f.service.Answer(ctx, "attempt-1", "another")

	// Then there is nothing to answer
	assert.ErrorIs(t, err, domainexercise.ErrNoQuestionPending)
}

func TestDefenseService_Answer_failsWhenThereAreNoQuestionsYet(t *testing.T) {
	// Given a defending attempt with no question
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptDefending), nil)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.DefenseTurn{}, nil)

	// When answering
	_, err := f.service.Answer(ctx, "attempt-1", "something")

	// Then there is nothing to answer
	assert.ErrorIs(t, err, domainexercise.ErrNoQuestionPending)
}

func TestDefenseService_Answer_refusesAttemptsThatAreNotDefending(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{domainexercise.AttemptEvaluated, domainexercise.AttemptFinished} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt that is not DEFENDING
			f := newDefenseFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(state), nil)

			// When answering
			_, err := f.service.Answer(ctx, "attempt-1", "something")

			// Then the defense is not active
			assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
		})
	}
}

func TestDefenseService_Answer_returnsTheStoreError(t *testing.T) {
	// Given a store that fails to save
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptDefending), nil)
	f.defense.EXPECT().ListByAttempt(ctx, "attempt-1").Return([]domainexercise.DefenseTurn{askedTurn(1, "c1", "q1", "")}, nil)
	storeErr := errors.New("disk full")
	f.defense.EXPECT().SaveAnswer(ctx, "turn-id", "a", fixedNow).Return(storeErr)

	// When answering
	_, err := f.service.Answer(ctx, "attempt-1", "a")

	// Then the failure is reported
	assert.ErrorIs(t, err, storeErr)
}

func answeredTranscript() []domainexercise.DefenseTurn {
	return []domainexercise.DefenseTurn{
		askedTurn(1, "c1", "q1", "a1"), askedTurn(2, "c3", "q2", domainexercise.IDontKnowAnswer), askedTurn(3, "c4", "q3", "a3"),
	}
}

func understandingOutput() domainexercise.EvaluatorOutput {
	return domainexercise.EvaluatorOutput{
		Model: "vendor/model-1", Summary: "Understands the cache.", Strengths: []string{"clear"},
		Gaps: []string{"consistency"}, Recommendations: []string{"read more"},
		Results: []domainexercise.CriterionResult{
			{CriterionID: "c1", Level: 3, Justification: "good"},
			{CriterionID: "c3", Level: 0, Justification: "did not know"},
			{CriterionID: "c4", Level: 4, Justification: "great"},
		},
	}
}

func expectedUnderstandingInput() domainexercise.UnderstandingInput {
	return domainexercise.UnderstandingInput{
		Challenge: defenseChallenge(), Rubric: defenseRubric(), Submission: defenseSubmission,
		Diagram: defenseDiagram(), Turns: answeredTranscript(),
	}
}

func TestDefenseService_Evaluate_scoresTheAnswersAndFinishesTheAttemptInOneTransaction(t *testing.T) {
	// Given a defending attempt with every answer given
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, answeredTranscript())
	f.expectSubmission(ctx)
	inTx := false
	f.evaluator.EXPECT().EvaluateUnderstanding(withinTimeout(), expectedUnderstandingInput()).
		RunAndReturn(func(context.Context, domainexercise.UnderstandingInput) (domainexercise.EvaluatorOutput, error) {
			assert.False(t, inTx, "the model must be called with no transaction open")
			return understandingOutput(), nil
		})
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			inTx = true
			defer func() { inTx = false }()
			return fn(ctx)
		})
	var stored domainexercise.Evaluation
	f.evaluations.EXPECT().Create(ctx, mock.MatchedBy(func(e domainexercise.Evaluation) bool {
		stored = e
		return true
	})).Return(nil)
	f.attempts.EXPECT().FinishDefense(ctx, "attempt-1").Return(nil)

	// When evaluating the defense
	evaluation, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the UNDERSTANDING evaluation is stored with the score computed over the criteria asked
	require.NoError(t, err)
	assert.Equal(t, stored, evaluation)
	assert.Equal(t, "turn-id", evaluation.ID)
	assert.Equal(t, domainexercise.EvaluationKindUnderstanding, evaluation.Kind)
	assert.Equal(t, "vendor/model-1", evaluation.Model)
	assert.Equal(t, fixedNow, evaluation.CreatedAt)
	assert.Equal(t, 58, evaluation.Score) // (25×3 + 25×0 + 25×4) / (75×4) = 58.33
	require.Len(t, evaluation.Results, 4)
	assert.True(t, evaluation.Results[1].InsufficientEvidence, "c2 was not asked")
	assert.Equal(t, "c2", evaluation.Results[1].CriterionID)
}

func TestDefenseService_Evaluate_aFailedCallLeavesTheAttemptDefendingAndCanBeRepeated(t *testing.T) {
	// Given a model that fails
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, answeredTranscript())
	f.expectSubmission(ctx)
	modelErr := errors.New("model timeout")
	f.evaluator.EXPECT().EvaluateUnderstanding(withinTimeout(), expectedUnderstandingInput()).Return(domainexercise.EvaluatorOutput{}, modelErr)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the error is returned and nothing was stored or finished
	assert.ErrorIs(t, err, modelErr)
	f.evaluations.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	f.attempts.AssertNotCalled(t, "FinishDefense", mock.Anything, mock.Anything)
}

func TestDefenseService_Evaluate_anOutputThatDoesNotMatchTheQuestionsStoresNothing(t *testing.T) {
	// Given a model that judges a criterion that was not asked
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, answeredTranscript())
	f.expectSubmission(ctx)
	output := understandingOutput()
	output.Results = append(output.Results, domainexercise.CriterionResult{CriterionID: "c2", Level: 4, Justification: "x"})
	f.evaluator.EXPECT().EvaluateUnderstanding(withinTimeout(), expectedUnderstandingInput()).Return(output, nil)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then it is an invalid evaluation and nothing is stored
	assert.ErrorIs(t, err, domainexercise.ErrInvalidEvaluation)
	f.evaluations.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDefenseService_Evaluate_aCancelledCallStoresNothing(t *testing.T) {
	// Given a context cancelled while the model works
	f := newDefenseFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.expectContext(ctx, domainexercise.AttemptDefending, answeredTranscript())
	f.expectSubmission(ctx)
	f.evaluator.EXPECT().EvaluateUnderstanding(withinTimeout(), expectedUnderstandingInput()).
		RunAndReturn(func(context.Context, domainexercise.UnderstandingInput) (domainexercise.EvaluatorOutput, error) {
			cancel()
			return understandingOutput(), nil
		})

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the cancellation is reported and nothing is stored
	assert.ErrorIs(t, err, context.Canceled)
	f.evaluations.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestDefenseService_Evaluate_aStorageFailureLeavesTheAttemptDefending(t *testing.T) {
	// Given a transaction that fails
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, answeredTranscript())
	f.expectSubmission(ctx)
	f.evaluator.EXPECT().EvaluateUnderstanding(withinTimeout(), expectedUnderstandingInput()).Return(understandingOutput(), nil)
	storeErr := errors.New("disk full")
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).Return(storeErr)

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the failure is returned
	assert.ErrorIs(t, err, storeErr)
}

func TestDefenseService_Evaluate_refusesWhileAQuestionIsWaiting(t *testing.T) {
	// Given a defending attempt whose last question has no answer
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{
		askedTurn(1, "c1", "q1", "a1"), askedTurn(2, "c3", "q2", ""),
	})

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then a question is still pending
	assert.ErrorIs(t, err, domainexercise.ErrQuestionPending)
}

func TestDefenseService_Evaluate_refusesWhileQuestionsAreStillToBeAsked(t *testing.T) {
	// Given a defending attempt with two of three questions asked and answered
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptDefending, []domainexercise.DefenseTurn{
		askedTurn(1, "c1", "q1", "a1"), askedTurn(2, "c3", "q2", "a2"),
	})

	// When evaluating
	_, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the defense is not complete
	assert.ErrorIs(t, err, domainexercise.ErrQuestionPending)
}

func TestDefenseService_Evaluate_aFinishedAttemptReturnsItsStoredEvaluationWithoutCallingTheModel(t *testing.T) {
	// Given a finished attempt
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptFinished), nil)
	stored := domainexercise.Evaluation{ID: "understanding-1", Kind: domainexercise.EvaluationKindUnderstanding, Score: 70}
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindUnderstanding).Return(stored, nil)

	// When evaluating again
	evaluation, err := f.service.Evaluate(ctx, "attempt-1")

	// Then the stored evaluation comes back
	require.NoError(t, err)
	assert.Equal(t, stored, evaluation)
}

func TestDefenseService_Evaluate_refusesAttemptsThatAreNotDefending(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{domainexercise.AttemptInProgress, domainexercise.AttemptEvaluated} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt that is not DEFENDING or FINISHED
			f := newDefenseFixture(t)
			ctx := context.Background()
			f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(state), nil)

			// When evaluating
			_, err := f.service.Evaluate(ctx, "attempt-1")

			// Then the defense is not active
			assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
		})
	}
}

func TestDefenseService_GetDefense_returnsTheTranscriptOfADefendingOrFinishedAttempt(t *testing.T) {
	for _, state := range []domainexercise.AttemptState{domainexercise.AttemptDefending, domainexercise.AttemptFinished} {
		t.Run(string(state), func(t *testing.T) {
			// Given an attempt with its three answered turns
			f := newDefenseFixture(t)
			ctx := context.Background()
			f.expectContext(ctx, state, answeredTranscript())

			// When reading the defense
			view, err := f.service.GetDefense(ctx, "attempt-1")

			// Then the turns and the total are returned, complete
			require.NoError(t, err)
			assert.Equal(t, DefenseView{Total: 3, Turns: answeredTranscript(), Complete: true}, view)
		})
	}
}

func TestDefenseService_GetDefense_anEvaluatedAttemptHasNoTurnsYet(t *testing.T) {
	// Given an evaluated attempt
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.expectContext(ctx, domainexercise.AttemptEvaluated, []domainexercise.DefenseTurn{})

	// When reading the defense
	view, err := f.service.GetDefense(ctx, "attempt-1")

	// Then it has the total and no turns, and is not complete
	require.NoError(t, err)
	assert.Equal(t, DefenseView{Total: 3, Turns: []domainexercise.DefenseTurn{}}, view)
}

func TestDefenseService_GetDefense_refusesAttemptsBeforeTheirDiagramWasEvaluated(t *testing.T) {
	// Given an attempt still being drawn
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptInProgress), nil)

	// When reading the defense
	_, err := f.service.GetDefense(ctx, "attempt-1")

	// Then the defense is not active
	assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
}

func TestDefenseService_Outcome_appliesThePassPolicyToBothEvaluations(t *testing.T) {
	// Given a finished attempt frozen at a pass score of 80, whose defense scored 58
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptFinished), nil)
	f.rubrics.EXPECT().Get(ctx, "rubric-2").Return(defenseRubric(), nil)
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindDiagram).Return(defenseDiagram(), nil)
	understanding := domainexercise.Evaluation{
		Kind: domainexercise.EvaluationKindUnderstanding, Score: 58,
		Results: []domainexercise.CriterionResult{{CriterionID: "c1", Level: 3}, {CriterionID: "c2", InsufficientEvidence: true}},
	}
	f.evaluations.EXPECT().GetByAttempt(ctx, "attempt-1", domainexercise.EvaluationKindUnderstanding).Return(understanding, nil)

	// When reading the outcome
	outcome, err := f.service.Outcome(ctx, "attempt-1")

	// Then both evaluations, the criteria and the failed outcome are returned
	require.NoError(t, err)
	assert.Equal(t, defenseDiagram(), outcome.Diagram)
	assert.Equal(t, understanding, outcome.Understanding)
	assert.Equal(t, defenseRubric().Criteria, outcome.Criteria)
	assert.Equal(t, 80, outcome.PassScore)
	assert.Equal(t, domainexercise.ResultFailed, outcome.Outcome.Result)
	assert.Contains(t, outcome.Outcome.Reasons, domainexercise.FailureReason{
		Kind: domainexercise.FailureUnderstandingScore, Evaluation: domainexercise.EvaluationKindUnderstanding,
	})
}

func TestDefenseService_Outcome_refusesAnAttemptThatIsNotFinished(t *testing.T) {
	// Given an attempt still defending
	f := newDefenseFixture(t)
	ctx := context.Background()
	f.attempts.EXPECT().Get(ctx, "attempt-1").Return(defenseAttempt(domainexercise.AttemptDefending), nil)

	// When reading the outcome
	_, err := f.service.Outcome(ctx, "attempt-1")

	// Then the defense is not finished
	assert.ErrorIs(t, err, domainexercise.ErrDefenseNotActive)
}
