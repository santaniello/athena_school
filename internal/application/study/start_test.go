package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func noopChunkHandler(string) error { return nil }

func noopSourcesHandler([]domainknowledge.Source) error { return nil }

func TestStart_returnsTopicRequired_whenTopicIsBlank(t *testing.T) {
	// Given a service and a blank topic
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When starting a session with a whitespace-only topic
	_, err := service.Start(context.Background(), "   ", "")

	// Then it fails with ErrTopicRequired; no port received any call since
	// none of the mocks above have a .EXPECT() set (mockery fails the test
	// via t.Cleanup if an unexpected call happens).
	require.ErrorIs(t, err, ErrTopicRequired)
}

func TestStart_createsAndPersistsSession(t *testing.T) {
	// Given a service whose repository accepts a new session in the chosen
	// folder. Start no longer calls the LLM at all — it only creates the
	// session, so the caller (the desktop binding) can switch the UI to the
	// chat view immediately, before requesting the opening turn separately
	// via RequestOpeningTurn.
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().GetByID(context.Background(), "folder-1").Return(domainfolder.Folder{ID: "folder-1"}, nil).Once()
	sessions.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(session domainstudy.Session) bool {
			return session.ID != "" && session.Topic == "Distributed systems" &&
				session.Mode == domainstudy.ModeStudy && session.FolderID == "folder-1" && !session.StartedAt.IsZero() &&
				session.Context == (domainstudy.ContextUsage{State: domainstudy.ContextStateNormal})
		})).
		Return(nil).
		Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When starting a session for a topic in a folder
	session, err := service.Start(context.Background(), "Distributed systems", "folder-1")

	// Then it succeeds and returns the created session; profiles/llm/messages
	// were never touched (no .EXPECT() set on those mocks)
	require.NoError(t, err)
	require.NotEmpty(t, session.ID)
	require.Equal(t, "Distributed systems", session.Topic)
	require.Equal(t, "folder-1", session.FolderID)
}

func TestStart_fallsBackToDefaultFolder_whenFolderIDIsBlank(t *testing.T) {
	// Given a service and no folder chosen
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().
		GetByID(context.Background(), domainfolder.DefaultFolderID).
		Return(domainfolder.Folder{ID: domainfolder.DefaultFolderID, IsDefault: true}, nil).
		Once()
	sessions.EXPECT().
		Create(context.Background(), mock.MatchedBy(func(session domainstudy.Session) bool {
			return session.FolderID == domainfolder.DefaultFolderID
		})).
		Return(nil).
		Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When starting a session without specifying a folder
	session, err := service.Start(context.Background(), "Distributed systems", "")

	// Then it lands in the default folder
	require.NoError(t, err)
	require.Equal(t, domainfolder.DefaultFolderID, session.FolderID)
}

func TestStart_propagatesFolderNotFound_whenChosenFolderDoesNotExist(t *testing.T) {
	// Given a service whose folder repository has no matching folder
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().GetByID(context.Background(), "missing").Return(domainfolder.Folder{}, domainfolder.ErrFolderNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When starting a session in a folder that does not exist
	_, err := service.Start(context.Background(), "Distributed systems", "missing")

	// Then the error propagates; sessions.Create is never called (no
	// .EXPECT() set)
	require.ErrorIs(t, err, domainfolder.ErrFolderNotFound)
}
