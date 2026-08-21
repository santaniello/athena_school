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
)

func TestRequestOpeningTurn_streamsAndPersistsAssistantReply(t *testing.T) {
	// Given a service whose ports all succeed and an LLM that streams two chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			return req.Task == domainllm.TaskStudy && len(req.Messages) == 1 && req.Messages[0].Role == "system"
		}), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("Hello "))
			require.NoError(t, handler("there!"))
		}).
		Return(nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(message domainstudy.Message) bool {
			return message.Role == domainstudy.RoleAssistant && message.Content == "Hello there!" && message.SessionID == "session-1"
		})).
		Return(nil).
		Once()

	var received []string
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When requesting the opening turn for an already-created session
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

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
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, loadErr)

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler)

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
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	streamErr := errors.New("upstream failure")
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(streamErr).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When requesting the opening turn
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", noopChunkHandler)

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
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When requesting the opening turn again
	err := service.RequestOpeningTurn(context.Background(), "session-1", "Distributed systems", func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then it replays the existing message and never calls the LLM or
	// profile store (no .EXPECT() set for either) nor persists a new message
	require.NoError(t, err)
	require.Equal(t, []string{"Already said hello."}, received)
}
