package exercise

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

type generationFixture struct {
	service    *GenerationService
	sessions   *exercisemocks.MockSessionTopicReader
	summaries  *exercisemocks.MockSummaryReader
	profiles   *exercisemocks.MockProfileReader
	notes      *exercisemocks.MockNotesSearcher
	llm        *llmmocks.MockProvider
	challenges *domainmocks.MockChallengeRepository
	rubrics    *domainmocks.MockRubricRepository
	tx         *exercisemocks.MockTransactor
	renderer   *exercisemocks.MockStarterDiagramRenderer
}

// txMarker is set on the context a fake transaction hands to its function, so a
// test can tell that a write ran inside it.
type txMarker struct{}

func newGenerationFixture(t *testing.T) *generationFixture {
	t.Helper()
	f := &generationFixture{
		sessions:   exercisemocks.NewMockSessionTopicReader(t),
		summaries:  exercisemocks.NewMockSummaryReader(t),
		profiles:   exercisemocks.NewMockProfileReader(t),
		notes:      exercisemocks.NewMockNotesSearcher(t),
		llm:        llmmocks.NewMockProvider(t),
		challenges: domainmocks.NewMockChallengeRepository(t),
		rubrics:    domainmocks.NewMockRubricRepository(t),
		tx:         exercisemocks.NewMockTransactor(t),
		renderer:   exercisemocks.NewMockStarterDiagramRenderer(t),
	}
	challengeService := NewChallengeService(f.challenges, f.sessions)
	challengeService.now = func() time.Time { return fixedNow }
	challengeService.newID = func() string { return "challenge-1" }
	rubricService := NewRubricService(f.challenges, f.rubrics)
	rubricService.now = func() time.Time { return fixedNow }
	ids := 0
	rubricService.newID = func() string { ids++; return fmt.Sprintf("id-%d", ids) }
	f.service = NewGenerationService(f.sessions, f.summaries, f.profiles, f.notes, f.llm, f.renderer, challengeService, rubricService, f.tx)
	return f
}

// expectContextPieces sets up the session, summary, profile and notes reads.
func (f *generationFixture) expectContextPieces(ctx context.Context) {
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, nil)
}

func (f *generationFixture) expectChat(ctx context.Context, content string, err error) {
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1",
		Task:      domainllm.TaskArchitectureLab,
		Messages:  buildGenerationPrompt(generationContextOf(), interviewSpec(domainexercise.DifficultyIntermediate)),
	}).Return(domainllm.ChatResponse{Content: content}, err).Once()
}

func generationContextOf() generationContext {
	return generationContext{
		Topic: testSession.Topic, Goal: testSession.Goal, Summary: "We covered eviction.", Profile: testProfile,
	}
}

func (f *generationFixture) runTx(ctx context.Context) {
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(context.WithValue(ctx, txMarker{}, true))
		}).Once()
}

func insideTx() any {
	return mock.MatchedBy(func(c context.Context) bool { return c.Value(txMarker{}) != nil })
}

func TestGenerationService_GenerateChallenge_savesChallengeAndRubricInOneTransaction(t *testing.T) {
	// Given a studied session and a model that answers with a valid challenge
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContextPieces(ctx)
	f.sessions.EXPECT().ReadSession(insideTx(), "session-1").Return(testSession, nil).Once()
	f.expectChat(ctx, encode(t, validOutput()), nil)
	f.runTx(ctx)
	stored := expectedGeneratedChallenge()
	f.challenges.EXPECT().Create(insideTx(), stored).Return(nil)
	f.challenges.EXPECT().Get(insideTx(), "challenge-1").Return(stored, nil)
	f.rubrics.EXPECT().Latest(insideTx(), "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	rubric := expectedGeneratedRubric(t)
	f.rubrics.EXPECT().Create(insideTx(), rubric).Return(nil)

	// When the challenge is generated
	got, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then an AI-generated challenge of that session and difficulty is saved
	require.NoError(t, err)
	assert.Equal(t, stored, got)
	assert.Equal(t, domainexercise.OriginAIGenerated, got.Origin)
	assert.Equal(t, "session-1", got.SourceSessionID)
	assert.Equal(t, "distributed caching", got.Topic)
	assert.Equal(t, domainexercise.DifficultyIntermediate, got.Difficulty)
	assert.Equal(t, 45, got.TimeLimitMinutes)
	assert.Equal(t, 5, got.DefenseQuestions)
	assert.Equal(t, "Design a global cache", got.Title)

	// And its first rubric version holds the five base criteria plus the generated ones
	assert.Equal(t, 1, rubric.Version)
	assert.Equal(t, "challenge-1", rubric.ChallengeID)
	require.Len(t, rubric.Criteria, 7)
	assert.Equal(t, "c-cache-invalidation", rubric.Criteria[5].Key)
	assert.Equal(t, domainexercise.CriterionSourceChallenge, rubric.Criteria[5].Source)
}

func TestGenerationService_GenerateChallenge_failsForAnUnknownSession(t *testing.T) {
	// Given a session that does not exist
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{}, ErrSourceSessionNotFound)

	// When a challenge is generated for it
	_, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then it fails before any model call
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_GenerateChallenge_failsForABlankSessionID(t *testing.T) {
	// Given no session id
	f := newGenerationFixture(t)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(context.Background(), "  ", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then it fails before reading anything
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_GenerateChallenge_failsForAnInvalidDifficulty(t *testing.T) {
	// Given a difficulty that does not exist
	f := newGenerationFixture(t)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(context.Background(), "session-1", interviewSpec(domainexercise.Difficulty("EXPERT")))

	// Then it fails before reading anything
	require.ErrorIs(t, err, domainexercise.ErrDifficultyInvalid)
}

func TestGenerationService_GenerateChallenge_storesNothingWhenTheModelFails(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"provider error", errors.New("openrouter is down")},
		{"cancelled context", context.Canceled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a model call that fails
			f := newGenerationFixture(t)
			ctx := context.Background()
			f.expectContextPieces(ctx)
			f.expectChat(ctx, "", tt.err)

			// When a challenge is generated
			_, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

			// Then the error is returned and no transaction is opened
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestGenerationService_GenerateChallenge_storesNothingWhenTheOutputIsInvalid(t *testing.T) {
	// Given a model that answers with text that is not JSON
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContextPieces(ctx)
	f.expectChat(ctx, "I cannot do that.", nil)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then the output is rejected and no transaction is opened
	require.ErrorIs(t, err, ErrInvalidGeneration)
}

func TestGenerationService_GenerateChallenge_failsWhenContextCannotBeBuilt(t *testing.T) {
	// Given a profile that cannot be read
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil)
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{}, errors.New("no profile"))

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then it fails before any model call
	require.Error(t, err)
}

func TestGenerationService_GenerateChallenge_returnsTheErrorWhenTheRubricCannotBeSaved(t *testing.T) {
	// Given a rubric write that fails inside the transaction
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContextPieces(ctx)
	f.sessions.EXPECT().ReadSession(insideTx(), "session-1").Return(testSession, nil).Once()
	f.expectChat(ctx, encode(t, validOutput()), nil)
	f.runTx(ctx)
	stored := expectedGeneratedChallenge()
	f.challenges.EXPECT().Create(insideTx(), stored).Return(nil)
	f.challenges.EXPECT().Get(insideTx(), "challenge-1").Return(stored, nil)
	f.rubrics.EXPECT().Latest(insideTx(), "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	saveFailure := errors.New("disk full")
	f.rubrics.EXPECT().Create(insideTx(), expectedGeneratedRubric(t)).Return(saveFailure)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(ctx, "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then the error is returned out of the transaction, which rolls the challenge back
	require.ErrorIs(t, err, saveFailure)
}

// expectedGeneratedChallenge is the challenge validOutput produces for
// session-1 at INTERMEDIATE difficulty, with the fixture's id and clock.
func expectedGeneratedChallenge() domainexercise.Challenge {
	return domainexercise.Challenge{
		ID:                        "challenge-1",
		Type:                      domainexercise.ChallengeTypeArchitecture,
		Kind:                      domainexercise.ChallengeKindInterview,
		Title:                     "Design a global cache",
		Scenario:                  "A shop serves 50k requests per second worldwide.",
		FunctionalRequirements:    []string{"Serve product pages"},
		NonFunctionalRequirements: []string{"p99 under 100 ms"},
		Constraints:               []string{"Three regions"},
		Objectives:                []string{"Justify the eviction policy"},
		Difficulty:                domainexercise.DifficultyIntermediate,
		Topic:                     "distributed caching",
		SourceSessionID:           "session-1",
		Origin:                    domainexercise.OriginAIGenerated,
		TimeLimitMinutes:          45,
		DefenseQuestions:          5,
		CreatedAt:                 fixedNow,
		UpdatedAt:                 fixedNow,
	}
}

// expectedGeneratedRubric is the first rubric version validOutput produces:
// the base criteria then the two generated ones, with the fixture's ids
// (id-1 to id-7 for the criteria, id-8 for the version) and clock.
func expectedGeneratedRubric(t *testing.T) domainexercise.RubricVersion {
	t.Helper()
	generated, err := parseGeneration(encode(t, validOutput()), "distributed caching", interviewSpec(domainexercise.DifficultyIntermediate), nil)
	require.NoError(t, err)
	criteria := make([]domainexercise.RubricCriterion, len(generated.Criteria))
	for i, input := range generated.Criteria {
		criteria[i] = domainexercise.RubricCriterion{
			Key: input.Key, Name: input.Name, Description: input.Description,
			Weight: input.Weight, Mandatory: input.Mandatory, Levels: input.Levels,
		}
	}
	merged, err := domainexercise.MergeRubric(domainexercise.BaseRubric(), criteria)
	require.NoError(t, err)
	for i := range merged {
		merged[i].ID = fmt.Sprintf("id-%d", i+1)
	}
	return domainexercise.RubricVersion{
		ID: fmt.Sprintf("id-%d", len(merged)+1), ChallengeID: "challenge-1", Version: 1,
		BaseVersion: domainexercise.BaseRubricVersion, Criteria: merged, CreatedAt: fixedNow,
	}
}

func TestGenerationService_GenerateChallenge_practiceSavesAnUntimedChallengeWithALeanRubric(t *testing.T) {
	// Given a studied session and a model that answers a practice with two criteria
	f := newGenerationFixture(t)
	ctx := context.Background()
	spec := practiceSpec("cache aside")
	f.expectContextPieces(ctx)
	f.sessions.EXPECT().ReadSession(insideTx(), "session-1").Return(testSession, nil).Once()
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1",
		Task:      domainllm.TaskArchitectureLab,
		Messages:  buildGenerationPrompt(generationContextOf(), spec),
	}).Return(domainllm.ChatResponse{Content: encode(t, practiceOutput())}, nil).Once()
	f.renderer.EXPECT().Render([]byte(encode(t, starterStructureOutput()))).Return("<mxGraphModel>drawn</mxGraphModel>", nil).Once()
	f.runTx(ctx)
	var stored domainexercise.Challenge
	f.challenges.EXPECT().Create(insideTx(), mock.MatchedBy(func(c domainexercise.Challenge) bool { stored = c; return true })).Return(nil)
	f.challenges.EXPECT().Get(insideTx(), "challenge-1").RunAndReturn(func(context.Context, string) (domainexercise.Challenge, error) { return stored, nil })
	f.rubrics.EXPECT().Latest(insideTx(), "challenge-1").Return(domainexercise.RubricVersion{}, domainexercise.ErrRubricNotFound)
	var rubric domainexercise.RubricVersion
	f.rubrics.EXPECT().Create(insideTx(), mockRubricVersion(&rubric)).Return(nil)

	// When the practice is generated
	got, err := f.service.GenerateChallenge(ctx, "session-1", spec)

	// Then it is an untimed practice and its rubric holds only the two generated criteria
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ChallengeKindPractice, got.Kind)
	assert.Equal(t, 0, got.TimeLimitMinutes)
	assert.Equal(t, "<mxGraphModel>drawn</mxGraphModel>", got.StarterDiagram)
	assert.Equal(t, domainexercise.NoBaseRubricVersion, rubric.BaseVersion)
	assert.Len(t, rubric.Criteria, 2)
}

func TestGenerationService_GenerateChallenge_refusesAPracticeWithoutAGoalBeforeAnyModelCall(t *testing.T) {
	// Given a practice spec with a blank goal
	f := newGenerationFixture(t)

	// When generating
	_, err := f.service.GenerateChallenge(context.Background(), "session-1", practiceSpec("   "))

	// Then it is refused before reading the session or calling the model (strict mocks)
	assert.ErrorIs(t, err, domainexercise.ErrPracticeGoalRequired)
}

func TestGenerationService_GenerateChallenge_refusesAnUnknownKind(t *testing.T) {
	// Given a spec of an unknown kind
	f := newGenerationFixture(t)
	spec := domainexercise.ChallengeSpec{Difficulty: domainexercise.DifficultyBeginner, Kind: "QUIZ"}

	// When generating
	_, err := f.service.GenerateChallenge(context.Background(), "session-1", spec)

	// Then it is refused
	assert.ErrorIs(t, err, domainexercise.ErrChallengeKindInvalid)
}
