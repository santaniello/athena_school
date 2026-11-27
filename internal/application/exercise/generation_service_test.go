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
	t          *testing.T
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
		t:          t,
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
	f.service = NewGenerationService(f.sessions, f.summaries, f.profiles, f.notes, f.challenges, f.llm, f.renderer, challengeService, rubricService, f.tx)
	return f
}

// expectContextPiecesFor sets up the session, summary, profile, notes and
// existing-title reads of a challenge of the given kind whose session already
// has the given titles.
func (f *generationFixture) expectContextPiecesFor(ctx context.Context, kind domainexercise.ChallengeKind, titles []string) {
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, nil)
	f.challenges.EXPECT().ListTitlesBySourceSession(ctx, "session-1", kind, 10).Return(titles, nil)
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

func TestGenerationService_GenerateChallenge_failsForAnUnknownSession(t *testing.T) {
	// Given a session that does not exist
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{}, ErrSourceSessionNotFound)

	// When a challenge is generated for it
	_, err := f.service.GenerateChallenge(ctx, "session-1", practiceSpec("cache aside"))

	// Then it fails before any model call
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_GenerateChallenge_refusesASessionOfAFolderWithTheLabDisabled(t *testing.T) {
	// Given a session whose folder has the Architecture Lab off
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "Streams", ArchLabEnabled: false}, nil)

	// When a challenge is generated for it
	_, err := f.service.GenerateChallenge(ctx, "session-1", practiceSpec("cache aside"))

	// Then it fails with ErrArchLabNotEnabled before any context read, model call or write
	require.ErrorIs(t, err, domainexercise.ErrArchLabNotEnabled)
}

func TestGenerationService_GenerateChallenge_failsForABlankSessionID(t *testing.T) {
	// Given no session id
	f := newGenerationFixture(t)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(context.Background(), "  ", practiceSpec("cache aside"))

	// Then it fails before reading anything
	require.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_GenerateChallenge_failsForAnInvalidDifficulty(t *testing.T) {
	// Given a difficulty that does not exist
	f := newGenerationFixture(t)

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(context.Background(), "session-1", domainexercise.ChallengeSpec{Difficulty: "EXPERT", Kind: domainexercise.ChallengeKindPractice, PracticeGoal: "cache aside"})

	// Then it fails before reading anything
	require.ErrorIs(t, err, domainexercise.ErrDifficultyInvalid)
}

func TestGenerationService_GenerateChallenge_failsWhenContextCannotBeBuilt(t *testing.T) {
	// Given a profile that cannot be read
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil)
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{}, errors.New("no profile"))

	// When a challenge is generated
	_, err := f.service.GenerateChallenge(ctx, "session-1", practiceSpec("cache aside"))

	// Then it fails before any model call
	require.Error(t, err)
}

func TestGenerationService_GenerateChallenge_practiceSavesAnUntimedChallengeWithALeanRubric(t *testing.T) {
	// Given a studied session and a model that answers a practice with two criteria
	f := newGenerationFixture(t)
	ctx := context.Background()
	spec := practiceSpec("cache aside")
	f.expectContextPiecesFor(ctx, domainexercise.ChallengeKindPractice, []string{"Cache-Aside basics"})
	f.sessions.EXPECT().ReadSession(insideTx(), "session-1").Return(testSession, nil).Once()
	gc := generationContextOf()
	gc.ExistingTitles = []string{"Cache-Aside basics"}
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1",
		Task:      domainllm.TaskArchitectureLab,
		Messages:  buildGenerationPrompt(gc, spec),
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

func TestGenerationService_GenerateChallenge_refusesAnInterviewBeforeAnyRead(t *testing.T) {
	// Given a request for an interview from a session
	f := newGenerationFixture(t)

	// When generating
	_, err := f.service.GenerateChallenge(context.Background(), "session-1", interviewSpec(domainexercise.DifficultyIntermediate))

	// Then it is refused before reading the session or calling the model (strict mocks)
	assert.ErrorIs(t, err, ErrInterviewNotGeneratedFromSession)
}
