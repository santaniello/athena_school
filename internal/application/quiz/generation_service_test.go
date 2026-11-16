package quiz

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	quizmocks "github.com/santaniello/athena/internal/application/quiz/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
	domainmocks "github.com/santaniello/athena/internal/domain/quiz/mocks"
)

var (
	fixedNow    = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	testSession = domainquiz.SourceSession{Topic: "Distributed Caching", Goal: "Understand eviction"}
	testProfile = domainquiz.GenerationProfile{Area: "backend", ExperienceLevel: "intermediate", Language: "pt"}
	testSpec    = domainquiz.Spec{Count: 3, Difficulty: domainquiz.DifficultyMedium}
)

type generationFixture struct {
	service   *GenerationService
	sessions  *quizmocks.MockSessionReader
	summaries *quizmocks.MockSummaryReader
	profiles  *quizmocks.MockProfileReader
	notes     *quizmocks.MockNotesSearcher
	llm       *llmmocks.MockProvider
	repo      *domainmocks.MockRepository
}

// reverse is a deterministic shuffle: it reverses the options.
func reverse(n int) []int {
	perm := make([]int, n)
	for i := range perm {
		perm[i] = n - 1 - i
	}
	return perm
}

func newGenerationFixture(t *testing.T) *generationFixture {
	t.Helper()
	f := &generationFixture{
		sessions:  quizmocks.NewMockSessionReader(t),
		summaries: quizmocks.NewMockSummaryReader(t),
		profiles:  quizmocks.NewMockProfileReader(t),
		notes:     quizmocks.NewMockNotesSearcher(t),
		llm:       llmmocks.NewMockProvider(t),
		repo:      domainmocks.NewMockRepository(t),
	}
	f.service = NewGenerationService(f.sessions, f.summaries, f.profiles, f.notes, f.llm, f.repo)
	f.service.now = func() time.Time { return fixedNow }
	ids := 0
	f.service.newID = func() string { ids++; return fmt.Sprintf("id-%d", ids) }
	f.service.shuffle = reverse
	return f
}

func (f *generationFixture) expectContext(ctx context.Context) {
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return([]domainquiz.NoteExcerpt{{Source: "cache.md", Content: "LRU."}}, nil).Once()
}

func (f *generationFixture) expectChat(ctx context.Context, spec domainquiz.Spec, content string, err error) {
	gc := generationContext{
		Topic: testSession.Topic, Goal: testSession.Goal, Summary: "We covered eviction.", Profile: testProfile,
		Notes: []domainquiz.NoteExcerpt{{Source: "cache.md", Content: "LRU."}},
	}
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{
		SessionID: "session-1", Task: domainllm.TaskQuiz, Messages: buildPrompt(gc, spec),
	}).Return(domainllm.ChatResponse{Content: content}, err).Once()
}

// expectedQuiz is what threeQuestions becomes once it is shuffled (by reverse)
// and given IDs, for the session in testSession.
func expectedQuiz() domainquiz.Quiz {
	return domainquiz.Quiz{
		ID: "id-1", SessionID: "session-1", SessionTitle: "Distributed Caching", Difficulty: domainquiz.DifficultyMedium, CreatedAt: fixedNow,
		Questions: []domainquiz.Question{
			// the multiple-choice options are reversed and the correct index follows FIFO
			{ID: "id-2", Position: 1, Kind: domainquiz.KindMultipleChoice, Prompt: "Which policy evicts the oldest?", Options: []string{"FIFO", "LRU"}, CorrectIndex: 0, Explanation: "FIFO uses insertion order."},
			// true/false keeps its order
			{ID: "id-3", Position: 2, Kind: domainquiz.KindTrueFalse, Prompt: "A cache is always faster.", Options: []string{"True", "False"}, CorrectIndex: 1, Explanation: "Cold caches add a hop."},
			{ID: "id-4", Position: 3, Kind: domainquiz.KindMultipleChoice, Prompt: "What is a cache hit?", Options: []string{"C", "B", "A"}, CorrectIndex: 2, Explanation: "The key was found."},
		},
	}
}

func TestGenerationService_Generate_storesTheShuffledQuiz(t *testing.T) {
	// Given a studied session and a model that answers with three valid questions
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, threeQuestions, nil)
	want := expectedQuiz()
	f.repo.EXPECT().CreateQuiz(ctx, want).Return(nil).Once()

	// When the quiz is generated
	got, err := f.service.Generate(ctx, " session-1 ", testSpec)

	// Then the shuffled quiz is stored and returned, titled with the session topic
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGenerationService_GenerateForMessage_storesTheQuizLinkedToTheMessage(t *testing.T) {
	// Given a studied session and a model that answers with three valid questions
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, threeQuestions, nil)
	want := expectedQuiz()
	want.MessageID = "message-1"
	f.repo.EXPECT().CreateQuiz(ctx, want).Return(nil).Once()

	// When the quiz is generated for the chat message of an offer
	got, err := f.service.GenerateForMessage(ctx, "session-1", "message-1", testSpec)

	// Then the stored and returned quiz carries the message id
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestGenerationService_Generate_leavesTheMessageIDEmpty(t *testing.T) {
	// Given a quiz asked for from the menu
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, threeQuestions, nil)
	f.repo.EXPECT().CreateQuiz(ctx, expectedQuiz()).Return(nil).Once()

	// When it is generated
	got, err := f.service.Generate(ctx, "session-1", testSpec)

	// Then it has no message
	require.NoError(t, err)
	assert.Empty(t, got.MessageID)
}

func TestGenerationService_Generate_refusesAnInvalidSpecBeforeAnything(t *testing.T) {
	// Given a spec with too few questions (the strict mocks fail on any call)
	f := newGenerationFixture(t)

	// When generating
	_, err := f.service.Generate(context.Background(), "session-1", domainquiz.Spec{Count: 2, Difficulty: domainquiz.DifficultyEasy})

	// Then it is refused
	assert.ErrorIs(t, err, domainquiz.ErrQuestionCountInvalid)
}

func TestGenerationService_Generate_normalizesTheSpec(t *testing.T) {
	// Given an empty spec, which means 10 medium questions
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, domainquiz.Spec{Count: 10, Difficulty: domainquiz.DifficultyMedium}, "", errors.New("stop here"))

	// When generating
	_, err := f.service.Generate(ctx, "session-1", domainquiz.Spec{})

	// Then the prompt was built from the defaults (the expectation above proves it)
	assert.Error(t, err)
}

func TestGenerationService_Generate_blankSessionIsNotFound(t *testing.T) {
	f := newGenerationFixture(t)
	_, err := f.service.Generate(context.Background(), "  ", testSpec)
	assert.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_Generate_unknownSession(t *testing.T) {
	// Given a session the reader does not know
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainquiz.SourceSession{}, ErrSourceSessionNotFound).Once()

	// When generating
	_, err := f.service.Generate(ctx, "session-1", testSpec)

	// Then it is reported as not found
	assert.ErrorIs(t, err, ErrSourceSessionNotFound)
}

func TestGenerationService_Generate_contextFailuresStopTheCall(t *testing.T) {
	boom := errors.New("boom")
	tests := map[string]func(f *generationFixture, ctx context.Context){
		"summary": func(f *generationFixture, ctx context.Context) {
			f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
			f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", boom).Once()
		},
		"profile": func(f *generationFixture, ctx context.Context) {
			f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
			f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
			f.profiles.EXPECT().ReadProfile(ctx).Return(domainquiz.GenerationProfile{}, boom).Once()
		},
		"session": func(f *generationFixture, ctx context.Context) {
			f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainquiz.SourceSession{}, boom).Once()
		},
	}
	for name, arrange := range tests {
		t.Run(name, func(t *testing.T) {
			// Given a context read that fails
			f := newGenerationFixture(t)
			ctx := context.Background()
			arrange(f, ctx)

			// When generating
			_, err := f.service.Generate(ctx, "session-1", testSpec)

			// Then the failure is returned and the model is never called
			assert.ErrorIs(t, err, boom)
		})
	}
}

func TestGenerationService_Generate_aNotesFailureDoesNotStopGeneration(t *testing.T) {
	// Given a notes index that is not ready
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(testSession, nil).Once()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, errors.New("index not ready")).Once()
	gc := generationContext{Topic: testSession.Topic, Goal: testSession.Goal, Summary: "We covered eviction.", Profile: testProfile}
	f.llm.EXPECT().Chat(ctx, domainllm.ChatRequest{SessionID: "session-1", Task: domainllm.TaskQuiz, Messages: buildPrompt(gc, testSpec)}).
		Return(domainllm.ChatResponse{Content: threeQuestions}, nil).Once()
	f.repo.EXPECT().CreateQuiz(ctx, expectedQuiz()).Return(nil).Once()

	// When generating
	_, err := f.service.Generate(ctx, "session-1", testSpec)

	// Then the prompt had no notes and the quiz was still made
	assert.NoError(t, err)
}

func TestGenerationService_Generate_modelFailureStoresNothing(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	boom := errors.New("provider down")
	f.expectChat(ctx, testSpec, "", boom)

	_, err := f.service.Generate(ctx, "session-1", testSpec)

	assert.ErrorIs(t, err, boom)
}

func TestGenerationService_Generate_invalidAnswersStoreNothing(t *testing.T) {
	duplicateOptions := `{"questions":[
 {"kind":"MULTIPLE_CHOICE","prompt":"p","options":["A","a"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"q","options":["True","False"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"r","options":["True","False"],"correctIndex":0,"explanation":"e"}]}`
	badIndex := `{"questions":[
 {"kind":"TRUE_FALSE","prompt":"p","options":["True","False"],"correctIndex":5,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"q","options":["True","False"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"r","options":["True","False"],"correctIndex":0,"explanation":"e"}]}`
	for name, content := range map[string]string{"not json": "sorry", "duplicate options": duplicateOptions, "correct index out of range": badIndex} {
		t.Run(name, func(t *testing.T) {
			// Given a model answer that breaks a rule (the strict repo mock fails on a write)
			f := newGenerationFixture(t)
			ctx := context.Background()
			f.expectContext(ctx)
			f.expectChat(ctx, testSpec, content, nil)

			// When generating
			_, err := f.service.Generate(ctx, "session-1", testSpec)

			// Then it is invalid generation
			assert.ErrorIs(t, err, ErrInvalidGeneration)
		})
	}
}

func TestGenerationService_Generate_domainErrorIsKeptInTheChain(t *testing.T) {
	// Given an answer whose only flaw is a duplicated option
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, `{"questions":[
 {"kind":"MULTIPLE_CHOICE","prompt":"p","options":["A","a"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"q","options":["True","False"],"correctIndex":0,"explanation":"e"},
 {"kind":"TRUE_FALSE","prompt":"r","options":["True","False"],"correctIndex":0,"explanation":"e"}]}`, nil)

	// When generating
	_, err := f.service.Generate(ctx, "session-1", testSpec)

	// Then the caller can tell both that it is invalid and which rule broke
	assert.ErrorIs(t, err, ErrInvalidGeneration)
	assert.ErrorIs(t, err, domainquiz.ErrQuestionOptionDuplicate)
}

func TestGenerationService_Generate_repositoryFailureIsReturned(t *testing.T) {
	f := newGenerationFixture(t)
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, threeQuestions, nil)
	boom := errors.New("disk full")
	f.repo.EXPECT().CreateQuiz(ctx, expectedQuiz()).Return(boom).Once()

	_, err := f.service.Generate(ctx, "session-1", testSpec)

	assert.ErrorIs(t, err, boom)
}

func TestGenerationService_Generate_aBrokenShuffleStoresNothing(t *testing.T) {
	// Given a shuffle that does not return a permutation of the options
	f := newGenerationFixture(t)
	f.service.shuffle = func(n int) []int { return make([]int, n+1) }
	ctx := context.Background()
	f.expectContext(ctx)
	f.expectChat(ctx, testSpec, threeQuestions, nil)

	// When generating
	_, err := f.service.Generate(ctx, "session-1", testSpec)

	// Then the failure is reported, naming the first question, and nothing is stored
	assert.ErrorIs(t, err, domainquiz.ErrPermutationInvalid)
	assert.ErrorContains(t, err, "shuffling question 1:")
}

func TestNewGenerationService_defaultsAreRealImplementations(t *testing.T) {
	// When a service is built without test overrides
	service := NewGenerationService(nil, nil, nil, nil, nil, nil)

	// Then it stamps the current time, makes unique IDs and shuffles for real
	assert.WithinDuration(t, time.Now(), service.now(), time.Minute)
	assert.NotEqual(t, service.newID(), service.newID())
	assert.ElementsMatch(t, []int{0, 1, 2, 3, 4, 5}, service.shuffle(6))
}
