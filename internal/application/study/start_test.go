package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
	_, err := service.Start(context.Background(), "   ")

	// Then it fails with ErrTopicRequired; no port received any call since
	// none of the mocks above have a .EXPECT() set (mockery fails the test
	// via t.Cleanup if an unexpected call happens).
	require.ErrorIs(t, err, ErrTopicRequired)
}

func TestStart_createsAndPersistsSession(t *testing.T) {
	// Given a service whose repository accepts a new session. Start no
	// longer calls the LLM at all — it only creates the session, so the
	// caller (the desktop binding) can switch the UI to the chat view
	// immediately, before requesting the opening turn separately via
	// RequestOpeningTurn. Splitting these two steps is what makes the
	// opening turn's streaming actually visible to the user instead of the
	// whole response appearing at once after a long wait.
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	sessions.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(session domainstudy.Session) bool {
			return session.ID != "" && session.Topic == "Distributed systems" &&
				session.Mode == domainstudy.ModeStudy && !session.StartedAt.IsZero()
		})).
		Return(nil).
		Once()
	service := NewService(sessions, messages, llm, profiles)

	// When starting a session for a topic
	session, err := service.Start(context.Background(), "Distributed systems")

	// Then it succeeds and returns the created session; profiles/llm/messages
	// were never touched (no .EXPECT() set on those mocks)
	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.Equal(t, "Distributed systems", session.Topic)
}
