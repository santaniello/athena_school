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
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSendMessage_returnsMessageRequired_whenContentIsBlank(t *testing.T) {
	// Given a service and blank message content
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	service := NewService(sessions, messages, llm, profiles, folders)

	// When sending a whitespace-only message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "   ", noopChunkHandler)

	// Then it fails with ErrMessageRequired; no port received any call
	require.ErrorIs(t, err, ErrMessageRequired)
}

func TestSendMessage_persistsUserMessageBeforeCallingLLM(t *testing.T) {
	// Given a service tracking the order ports are called in
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	var callOrder []string
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
			return m.Role == domainstudy.RoleUser && m.Content == "What is CAP theorem?"
		})).
		Run(func(context.Context, domainstudy.Message) { callOrder = append(callOrder, "append-user") }).
		Return(nil).
		Once()
	messages.EXPECT().
		ListBySession(context.Background(), "session-1").
		Run(func(context.Context, string) { callOrder = append(callOrder, "list-history") }).
		Return([]domainstudy.Message{{Role: domainstudy.RoleUser, Content: "What is CAP theorem?"}}, nil).
		Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(context.Context, domainllm.ChatRequest, func(string) error) {
			callOrder = append(callOrder, "chat-stream")
		}).
		Return(nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
			return m.Role == domainstudy.RoleAssistant
		})).
		Run(func(context.Context, domainstudy.Message) { callOrder = append(callOrder, "append-assistant") }).
		Return(nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", noopChunkHandler)

	// Then the user message is persisted before the LLM is ever called, and
	// the assistant reply is persisted only after the stream completes
	require.NoError(t, err)
	require.Equal(t, []string{"append-user", "list-history", "chat-stream", "append-assistant"}, callOrder)
}

func TestSendMessage_sendsHistoryAndFreshSystemPromptToLLM(t *testing.T) {
	// Given a service with two prior turns in history
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Once()
	messages.EXPECT().
		ListBySession(context.Background(), "session-1").
		Return([]domainstudy.Message{
			{Role: domainstudy.RoleUser, Content: "Hi"},
			{Role: domainstudy.RoleAssistant, Content: "Hello!"},
			{Role: domainstudy.RoleUser, Content: "What is CAP theorem?"},
		}, nil).
		Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			if len(req.Messages) != 4 || req.Messages[0].Role != "system" {
				return false
			}
			return req.Messages[1].Content == "Hi" &&
				req.Messages[2].Content == "Hello!" &&
				req.Messages[3].Content == "What is CAP theorem?"
		}), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("It stands for..."))
		}).
		Return(nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
			return m.Role == domainstudy.RoleAssistant && m.Content == "It stands for..."
		})).
		Return(nil).
		Once()

	var received []string
	service := NewService(sessions, messages, llm, profiles, folders)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then it succeeds and forwarded the streamed chunk
	require.NoError(t, err)
	require.Equal(t, []string{"It stands for..."}, received)
}

func TestSendMessage_propagatesStreamError_withoutPersistingAssistantMessage(t *testing.T) {
	// Given a service whose LLM call fails mid-stream, after the user
	// message has already been persisted
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	streamErr := errors.New("upstream failure")
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(streamErr).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", noopChunkHandler)

	// Then the error propagates; the "assistant" Append above has no
	// .EXPECT(), so an unexpected call would fail the test
	require.ErrorIs(t, err, streamErr)
}
