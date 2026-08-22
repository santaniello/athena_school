package study

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
)

// normalSession is an unmeasured session in ContextStateNormal, the
// zero-usage state every fresh session and most opening-turn/no-history
// tests start from.
func normalSession(id string) domainstudy.Session {
	return domainstudy.Session{ID: id, Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal}}
}

// runWithinTx makes the mocked Transactor behave like the real one: it just
// invokes fn immediately against ctx, so the repository mocks set up
// underneath faithfully observe every call made inside the transaction.
func runWithinTx(tx *applicationstudymocks.MockTransactor) {
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		})
}

func TestRequestOpeningTurn_streamsAndPersistsAssistantReply(t *testing.T) {
	// Given a service whose ports all succeed and an LLM that streams two chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)

	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Task == domainllm.TaskStudy && len(req.Messages) == 1 && req.Messages[0].Role == "system"
		}), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("Hello "))
			require.NoError(t, handler("there!"))
		}).
		Return(domainllm.StreamResponse{}, nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(message domainstudy.Message) bool {
			return message.Role == domainstudy.RoleAssistant && message.Content == "Hello there!" && message.SessionID == "session-1"
		})).
		Return(nil).
		Once()
	sessions.EXPECT().UpdateContext(context.Background(), "session-1", mock.Anything).Return(nil).Once()

	var received []string
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When requesting the opening turn for an already-created session
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", func(chunk string) error {
		received = append(received, chunk)
		return nil
	}, nil, nil)

	// Then it succeeds and forwarded every chunk
	require.NoError(t, err)
	require.Equal(t, []string{"Hello ", "there!"}, received)
}

func TestRequestOpeningTurn_propagatesProfileLoadError(t *testing.T) {
	// Given a profile store that fails to load
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	loadErr := errors.New("profile not found")
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, loadErr)

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler, nil, nil)

	// Then the error propagates; the LLM is never called (no .EXPECT() set)
	require.ErrorIs(t, err, loadErr)
}

func TestRequestOpeningTurn_propagatesStreamError_withoutPersistingAssistantMessage(t *testing.T) {
	// Given a service whose LLM call fails mid-stream
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	streamErr := errors.New("upstream failure")
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, streamErr).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler, nil, nil)

	// Then the error propagates and no assistant message was ever appended
	// (messages mock has no .EXPECT() for Append, so an unexpected call
	// would fail the test)
	require.ErrorIs(t, err, streamErr)
}

func TestRequestOpeningTurn_replaysExistingMessageInsteadOfGeneratingASecondOne(t *testing.T) {
	// Given a session that already has a persisted opening turn — e.g. a
	// duplicate request racing the first one
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().
		ListBySession(context.Background(), "session-1").
		Return([]domainstudy.Message{{Role: domainstudy.RoleAssistant, Content: "Already said hello."}}, nil).
		Once()

	var received []string
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When requesting the opening turn again
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", func(chunk string) error {
		received = append(received, chunk)
		return nil
	}, nil, nil)

	// Then it replays the existing message and never calls the LLM, profile
	// store, or session repository (no .EXPECT() set for any of them) nor
	// persists a new message
	require.NoError(t, err)
	require.Equal(t, []string{"Already said hello."}, received)
}

func TestRequestOpeningTurn_blockedSession_returnsErrSessionContextLimitReached_withoutCallingLLM(t *testing.T) {
	// Given a session that somehow reached blocked before any opening turn
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().
		GetByID(context.Background(), "session-1").
		Return(domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateBlocked}}, nil).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler, nil, nil)

	// Then it's rejected before touching the profile store or the LLM (no
	// .EXPECT() set on either)
	require.ErrorIs(t, err, domainstudy.ErrSessionContextLimitReached)
}

func TestRequestOpeningTurn_concurrentCallsForSameSession_secondReturnsErrStudyTurnInProgress(t *testing.T) {
	// Given a session with no history yet, and an LLM call that blocks until
	// released, simulating an opening turn still streaming
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)

	release := make(chan struct{})
	started := make(chan struct{})
	llm.EXPECT().
		ChatStream(context.Background(), mock.Anything, mock.Anything).
		Run(func(context.Context, domainllm.ChatRequest, func(string) error) {
			close(started)
			<-release
		}).
		Return(domainllm.StreamResponse{}, errors.New("stream never completes in this test")).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler, nil, nil)
	}()
	<-started

	// When a second opening turn is requested for the same session while the
	// first is still in flight
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler, nil, nil)

	// Then it's rejected immediately with ErrStudyTurnInProgress, without
	// waiting for the first call or touching any repository/provider port
	// beyond what the first call already reserved (messages/sessions/llm
	// mocks above have no room for a second concurrent set of calls beyond
	// what's already configured).
	require.ErrorIs(t, err, ErrStudyTurnInProgress)

	close(release)
	require.Error(t, <-errCh)
}
