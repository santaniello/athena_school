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

	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func noopChunkHandler(string) error { return nil }

func TestStart_returnsTopicRequired_whenTopicIsBlank(t *testing.T) {
	// Given a service and a blank topic
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	service := NewService(sessions, messages, llm, profiles)

	// When starting a session with a whitespace-only topic
	_, err := service.Start(context.Background(), "   ", noopChunkHandler)

	// Then it fails with ErrTopicRequired; no port received any call since
	// none of the mocks above have a .EXPECT() set (mockery fails the test
	// via t.Cleanup if an unexpected call happens).
	require.ErrorIs(t, err, ErrTopicRequired)
}

func TestStart_createsSessionAndStreamsAndPersistsAssistantReply(t *testing.T) {
	// Given a service whose ports all succeed and an LLM that streams two chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)

	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	sessions.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(session domainstudy.Session) bool {
			return session.ID != "" && session.Topic == "Distributed systems" &&
				session.Mode == domainstudy.ModeStudy && !session.StartedAt.IsZero()
		})).
		Return(nil).
		Once()
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
			return message.Role == domainstudy.RoleAssistant && message.Content == "Hello there!" && message.SessionID != ""
		})).
		Return(nil).
		Once()

	var received []string
	service := NewService(sessions, messages, llm, profiles)

	// When starting a session for a topic
	session, err := service.Start(context.Background(), "Distributed systems", func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

	// Then it succeeds, returns the created session, and forwarded every chunk
	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.Equal(t, "Distributed systems", session.Topic)
	require.Equal(t, []string{"Hello ", "there!"}, received)
}

func TestStart_propagatesStreamError_withoutPersistingAssistantMessage(t *testing.T) {
	// Given a service whose LLM call fails mid-stream
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)

	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	sessions.EXPECT().Create(context.Background(), mock.AnythingOfType("study.Session")).Return(nil).Once()
	streamErr := errors.New("upstream failure")
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(streamErr).
		Once()

	service := NewService(sessions, messages, llm, profiles)

	// When starting a session
	session, err := service.Start(context.Background(), "Distributed systems", noopChunkHandler)

	// Then the error propagates, the session that was already created is
	// still returned, and no assistant message was ever appended (messages
	// mock has no .EXPECT() for Append, so an unexpected call would fail)
	require.ErrorIs(t, err, streamErr)
	require.NotEmpty(t, session.ID)
}
