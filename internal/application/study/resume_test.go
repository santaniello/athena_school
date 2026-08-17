package study

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestResume_reopensAnEndedSessionAndLoadsHistory(t *testing.T) {
	// Given a service with an ended session that has two prior messages
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	ended := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default", EndedAt: time.Now().UTC()}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(ended, nil).Once()
	sessions.EXPECT().Reopen(context.Background(), "session-1").Return(nil).Once()
	history := []domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Hi"}}
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	service := NewService(sessions, messages, llm, profiles, folders)

	// When resuming the session
	session, msgs, err := service.Resume(context.Background(), "session-1")

	// Then it is reopened (EndedAt cleared) and the full history is returned
	require.NoError(t, err)
	require.True(t, session.IsOpen())
	require.Equal(t, history, msgs)
}

func TestResume_doesNotReopenAnAlreadyOpenSession(t *testing.T) {
	// Given a service with a session that is still open
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	open := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(open, nil).Once()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	service := NewService(sessions, messages, llm, profiles, folders)

	// When resuming the session
	_, _, err := service.Resume(context.Background(), "session-1")

	// Then it succeeds without calling Reopen (no .EXPECT() set for it, so
	// an unexpected call would fail the test)
	require.NoError(t, err)
}

func TestResume_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "missing").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound).Once()
	service := NewService(sessions, messages, llm, profiles, folders)

	// When resuming a session that does not exist
	_, _, err := service.Resume(context.Background(), "missing")

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}

func TestResume_propagatesReopenError(t *testing.T) {
	// Given a service whose Reopen call fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	ended := domainstudy.Session{ID: "session-1", EndedAt: time.Now().UTC()}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(ended, nil).Once()
	reopenErr := errors.New("boom")
	sessions.EXPECT().Reopen(context.Background(), "session-1").Return(reopenErr).Once()
	service := NewService(sessions, messages, llm, profiles, folders)

	// When resuming the session
	_, _, err := service.Resume(context.Background(), "session-1")

	// Then the error propagates; history is never loaded (no .EXPECT() set)
	require.ErrorIs(t, err, reopenErr)
}
