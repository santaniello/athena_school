package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/folder"
	"github.com/santaniello/athena/internal/application/study"
	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// capturedEvents records every event emitted through App.emit during a test,
// so assertions can inspect them without touching the real Wails runtime
// (which os.Exit's when a.ctx wasn't produced by wails.Run).
type capturedEvents struct {
	chunks []string
	done   bool
	errors []string
}

func newTestStudyApp(t *testing.T, sessions domainstudy.SessionRepository, messages domainstudy.MessageRepository, llm domainllm.Provider, profiles domainprofile.Store, folders domainfolder.Repository) (*App, *capturedEvents) {
	t.Helper()
	studyService := study.NewService(sessions, messages, llm, profiles, folders)
	folderService := folder.NewService(folders, sessions)
	app := NewApp(nil, nil, nil, nil, nil, studyService, folderService, nil)
	app.Startup(context.Background())

	captured := &capturedEvents{}
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		switch eventName {
		case eventStudyChunk:
			captured.chunks = append(captured.chunks, data[0].(string))
		case eventStudyDone:
			captured.done = true
		case eventStudyError:
			captured.errors = append(captured.errors, data[0].(string))
		}
	}
	return app, captured
}

func TestApp_StartStudySession_createsAndReturnsSession(t *testing.T) {
	// Given an App backed by a study service that accepts a new session.
	// StartStudySession no longer calls the LLM at all — that happens in a
	// separate RequestOpeningTurn call the frontend makes once the chat view
	// is already showing, so the opening turn's streaming is actually
	// visible instead of the whole response appearing after a long wait.
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	folders.EXPECT().GetByID(mock.Anything, "default").Return(domainfolder.Folder{ID: "default", IsDefault: true}, nil).Once()
	sessions.EXPECT().Create(mock.Anything, mock.AnythingOfType("study.Session")).Return(nil).Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When starting a study session
	result, err := app.StartStudySession("Distributed systems", "")

	// Then it returns the created session without emitting any event (no
	// streaming happened) and without touching the LLM/profile/messages
	// ports (no .EXPECT() set on those mocks)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Distributed systems", result.Topic)
	assert.NotEmpty(t, result.StartedAt)
	assert.Empty(t, captured.chunks)
	assert.False(t, captured.done)
	assert.Empty(t, captured.errors)
}

func TestApp_StartStudySession_propagatesTopicRequiredError(t *testing.T) {
	// Given an App backed by a study service whose topic validation fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When starting a session with a blank topic
	_, err := app.StartStudySession("   ", "")

	// Then the error propagates directly (the frontend catches the rejected
	// promise for this call, not an event), with no event emitted at all
	require.ErrorIs(t, err, study.ErrTopicRequired)
	assert.Empty(t, captured.chunks)
	assert.False(t, captured.done)
	assert.Empty(t, captured.errors)
}

func TestApp_RequestOpeningTurn_streamsChunksAndSignalsDone(t *testing.T) {
	// Given an App backed by a study service that streams two chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("Hello "))
			require.NoError(t, handler("there!"))
		}).
		Return(nil).
		Once()
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When requesting the opening turn for an already-created session
	err := app.RequestOpeningTurn("session-1", "Distributed systems")

	// Then it streamed both chunks, ending with a "done" event
	require.NoError(t, err)
	assert.Equal(t, []string{"Hello ", "there!"}, captured.chunks)
	assert.True(t, captured.done)
	assert.Empty(t, captured.errors)
}

func TestApp_RequestOpeningTurn_emitsErrorEvent_onFailure(t *testing.T) {
	// Given an App backed by a study service whose profile load fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, errors.New("profile not found"))
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When requesting the opening turn
	err := app.RequestOpeningTurn("session-1", "Distributed systems")

	// Then the error propagates and an error event was emitted, with no
	// chunk or done event
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Empty(t, captured.chunks)
	assert.False(t, captured.done)
}

func TestApp_SendStudyMessage_streamsReplyAndSignalsDone(t *testing.T) {
	// Given an App backed by a study service that streams a reply
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("It stands for..."))
		}).
		Return(nil).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?")

	// Then it succeeds, forwarded the chunk and signaled done
	require.NoError(t, err)
	assert.Equal(t, []string{"It stands for..."}, captured.chunks)
	assert.True(t, captured.done)
}

func TestApp_SendStudyMessage_emitsErrorEvent_onFailure(t *testing.T) {
	// Given an App backed by a study service whose LLM call fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Once()
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(errors.New("upstream failure")).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message and the LLM call fails
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?")

	// Then the error propagates and an error event was emitted, with no
	// done event
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.False(t, captured.done)
}

func TestApp_EndStudySession_endsTheSession(t *testing.T) {
	// Given an App backed by a study service that accepts ending the session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().End(mock.Anything, "session-1", mock.AnythingOfType("time.Time")).Return(nil).Once()
	app, _ := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When ending the session
	err := app.EndStudySession("session-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_DeleteStudySession_deletesTheSession(t *testing.T) {
	// Given an App backed by a study service that accepts deleting the session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().DeleteBySession(mock.Anything, "session-1").Return(nil).Once()
	sessions.EXPECT().Delete(mock.Anything, "session-1").Return(nil).Once()
	app, _ := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When deleting the session
	err := app.DeleteStudySession("session-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_ResumeStudySession_reopensAndReturnsHistory(t *testing.T) {
	// Given an App backed by a study service with an ended session that has
	// one prior message
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	ended := domainstudy.Session{ID: "session-1", Topic: "Distributed systems", FolderID: "default", EndedAt: time.Now().UTC()}
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(ended, nil).Once()
	sessions.EXPECT().Reopen(mock.Anything, "session-1").Return(nil).Once()
	messages.EXPECT().
		ListBySession(mock.Anything, "session-1").
		Return([]domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Hi"}}, nil).
		Once()
	app, _ := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When resuming the session
	result, err := app.ResumeStudySession("session-1")

	// Then it is reopened (no EndedAt) and its history is returned
	require.NoError(t, err)
	assert.Equal(t, "session-1", result.Session.ID)
	assert.Empty(t, result.Session.EndedAt)
	require.Len(t, result.Messages, 1)
	assert.Equal(t, "Hi", result.Messages[0].Content)
}

func TestApp_MoveStudySession_movesTheSession(t *testing.T) {
	// Given an App backed by a study service that accepts the move
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().MoveToFolder(mock.Anything, "session-1", "folder-b").Return(nil).Once()
	app, _ := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When moving the session to another folder
	err := app.MoveStudySession("session-1", "folder-b")

	// Then it succeeds
	require.NoError(t, err)
}

func TestApp_ListStudySessionsByFolder_returnsEverySessionInThatFolder(t *testing.T) {
	// Given an App backed by a study service with two sessions in folder-a
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().ListByFolder(mock.Anything, "folder-a").Return([]domainstudy.Session{
		{ID: "s-1", Topic: "Cache invalidation", FolderID: "folder-a"},
		{ID: "s-2", Topic: "Concurrency patterns", FolderID: "folder-a"},
	}, nil).Once()
	app, _ := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When listing sessions in folder-a
	results, err := app.ListStudySessionsByFolder("folder-a")

	// Then both sessions are returned as DTOs
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "Cache invalidation", results[0].Topic)
}
