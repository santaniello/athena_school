package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/folder"
	"github.com/santaniello/athena/internal/application/study"
	domainfolder "github.com/santaniello/athena/internal/domain/folder"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// normalSession is the session GetByID returns for tests that just need the
// context-limit guard to pass through: unmeasured, not blocked.
var normalSession = domainstudy.Session{
	ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal},
}

// passthroughTransactor is a study.Transactor that runs fn directly against
// the given ctx, with no real transaction — desktop tests exercise event
// wiring against mocked repositories, not SQLite atomicity (see
// internal/infrastructure/sqlite's own transaction tests for that).
type passthroughTransactor struct{}

func (passthroughTransactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// capturedEvents records every event emitted through App.emit during a test,
// so assertions can inspect them without touching the real Wails runtime
// (which os.Exit's when a.ctx wasn't produced by wails.Run).
type capturedEvents struct {
	chunks              []StudyChunkEvent
	dones               []StudyDoneEvent
	errors              []StudyErrorEvent
	sources             []StudySourcesEvent
	contextNormal       []StudyContextEvent
	contextWarning      []StudyContextEvent
	contextLimitReached []StudyContextEvent
	contextUnavailable  []StudyContextUnavailableEvent
	// done/gone bools kept for tests that only check whether it fired.
	done bool
}

func newTestStudyApp(t *testing.T, sessions domainstudy.SessionRepository, messages domainstudy.MessageRepository, llm domainllm.Provider, profiles domainprofile.Store, folders domainfolder.Repository) (*App, *capturedEvents) {
	t.Helper()
	retriever := knowledgemocks.NewMockRetriever(t)
	catalog := llmmocks.NewMockModelContextResolver(t)
	studyService := study.NewService(sessions, messages, llm, profiles, folders, retriever, passthroughTransactor{}, catalog)
	folderService := folder.NewService(folders, sessions)
	app := NewApp(nil, nil, nil, nil, nil, studyService, folderService, nil, nil, nil, nil)
	app.Startup(context.Background())

	captured := &capturedEvents{}
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		switch eventName {
		case eventStudyChunk:
			captured.chunks = append(captured.chunks, data[0].(StudyChunkEvent))
		case eventStudyDone:
			captured.done = true
			captured.dones = append(captured.dones, data[0].(StudyDoneEvent))
		case eventStudyError:
			captured.errors = append(captured.errors, data[0].(StudyErrorEvent))
		case eventStudySources:
			captured.sources = append(captured.sources, data[0].(StudySourcesEvent))
		case eventStudyContextNormal:
			captured.contextNormal = append(captured.contextNormal, data[0].(StudyContextEvent))
		case eventStudyContextWarning:
			captured.contextWarning = append(captured.contextWarning, data[0].(StudyContextEvent))
		case eventStudyContextLimitReached:
			captured.contextLimitReached = append(captured.contextLimitReached, data[0].(StudyContextEvent))
		case eventStudyContextLimitUnavailable:
			captured.contextUnavailable = append(captured.contextUnavailable, data[0].(StudyContextUnavailableEvent))
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
	// ports (no .EXPECT() set on those mocks), and its context usage starts
	// normal/unmeasured
	require.NoError(t, err)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Distributed systems", result.Topic)
	assert.NotEmpty(t, result.StartedAt)
	assert.Equal(t, string(domainstudy.ContextStateNormal), result.Context.State)
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
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(normalSession, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("Hello "))
			require.NoError(t, handler("there!"))
		}).
		Return(domainllm.StreamResponse{}, nil).
		Once()
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Once()
	sessions.EXPECT().UpdateContext(mock.Anything, "session-1", mock.Anything).Return(nil).Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When requesting the opening turn for an already-created session
	err := app.RequestOpeningTurn("session-1", "Distributed systems")

	// Then it streamed both chunks, ending with a "done" event, all
	// session-scoped, and no sources event fired (no retrieval on the
	// opening turn)
	require.NoError(t, err)
	require.Len(t, captured.chunks, 2)
	assert.Equal(t, "session-1", captured.chunks[0].SessionID)
	assert.Equal(t, []string{"Hello ", "there!"}, []string{captured.chunks[0].Content, captured.chunks[1].Content})
	require.True(t, captured.done)
	require.Len(t, captured.dones, 1)
	assert.Equal(t, "session-1", captured.dones[0].SessionID)
	assert.Empty(t, captured.errors)
	assert.Empty(t, captured.sources)
}

func TestApp_RequestOpeningTurn_emitsErrorEvent_onFailure(t *testing.T) {
	// Given an App backed by a study service whose profile load fails
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(normalSession, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, errors.New("profile not found"))
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When requesting the opening turn
	err := app.RequestOpeningTurn("session-1", "Distributed systems")

	// Then the error propagates and a session-scoped error event was
	// emitted, with no chunk or done event, and no stable code since this
	// isn't one of the pre-persistence errors
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Equal(t, "session-1", captured.errors[0].SessionID)
	assert.Empty(t, captured.errors[0].Code)
	assert.Empty(t, captured.chunks)
	assert.False(t, captured.done)
}

func TestApp_RequestOpeningTurn_blockedSession_emitsContextLimitReachedCode(t *testing.T) {
	// Given a session that has already reached its context limit
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(mock.Anything, "session-1").
		Return(domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateBlocked}}, nil).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When requesting the opening turn; llm/profiles have no .EXPECT() set,
	// so no provider call happens
	err := app.RequestOpeningTurn("session-1", "Distributed systems")

	// Then it fails with the domain sentinel, surfaced with the stable
	// "context_limit_reached" code
	require.ErrorIs(t, err, domainstudy.ErrSessionContextLimitReached)
	require.Len(t, captured.errors, 1)
	assert.Equal(t, errorCodeContextLimitReached, captured.errors[0].Code)
}

func TestApp_SendStudyMessage_streamsReplyAndSignalsDone(t *testing.T) {
	// Given an App backed by a study service that streams a reply in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(normalSession, nil).Once()
	sessions.EXPECT().UpdateContext(mock.Anything, "session-1", mock.Anything).Return(nil).Twice()
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			require.NoError(t, handler("It stands for..."))
		}).
		Return(domainllm.StreamResponse{}, nil).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message in web mode
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb)

	// Then it succeeds, forwarded the chunk and signaled done, both
	// session-scoped, and emitted an empty sources event first
	require.NoError(t, err)
	require.Len(t, captured.chunks, 1)
	assert.Equal(t, "session-1", captured.chunks[0].SessionID)
	assert.Equal(t, "It stands for...", captured.chunks[0].Content)
	assert.True(t, captured.done)
	require.Len(t, captured.sources, 1)
	assert.Equal(t, "session-1", captured.sources[0].SessionID)
	assert.Equal(t, []StudySourceResult{}, captured.sources[0].Sources)
}

func TestApp_SendStudyMessage_invalidSourceMode_emitsErrorEventWithoutPersistingOrCallingAnyPort(t *testing.T) {
	// Given an App and an unknown source mode; no port mock has any
	// .EXPECT() set, so any call would fail the test
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message with an unrecognized source mode
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?", "bogus-mode")

	// Then the error propagates and a session-scoped error event was
	// emitted (same failure-handling path as any other SendStudyMessage
	// error), with no chunk, done, or sources event, and no port was ever
	// called (no .EXPECT() set on any mock above)
	require.ErrorIs(t, err, domainknowledge.ErrInvalidSourceMode)
	assert.Empty(t, captured.chunks)
	assert.False(t, captured.done)
	require.Len(t, captured.errors, 1)
	assert.Equal(t, "session-1", captured.errors[0].SessionID)
	assert.Empty(t, captured.errors[0].Code)
	assert.Empty(t, captured.sources)
}

func TestApp_SendStudyMessage_blockedSession_emitsContextLimitReachedCode_withoutPersistingOrCallingAnyPort(t *testing.T) {
	// Given a session that has already reached its context limit; no
	// messages/retriever/llm mock has any .EXPECT() set
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(mock.Anything, "session-1").
		Return(domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateBlocked}}, nil).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb)

	// Then it fails with the domain sentinel before persisting the message,
	// surfaced with the stable "context_limit_reached" code
	require.ErrorIs(t, err, domainstudy.ErrSessionContextLimitReached)
	require.Len(t, captured.errors, 1)
	assert.Equal(t, errorCodeContextLimitReached, captured.errors[0].Code)
}

func TestApp_SendStudyMessage_emitsPostCapSourcesEvent_notes(t *testing.T) {
	// Given a study service (via a real Service backed by mocks) whose
	// retriever finds one surviving chunk in notes mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	catalog := llmmocks.NewMockModelContextResolver(t)

	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(normalSession, nil).Once()
	sessions.EXPECT().UpdateContext(mock.Anything, "session-1", mock.Anything).Return(nil).Twice()
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	result := domainknowledge.RetrievalResult{
		Chunks:     []domainknowledge.ScoredChunk{{Chunk: domainknowledge.Chunk{ID: "chunk-1"}, Score: 0.9}},
		Sufficient: true,
		Context:    `[{"heading":"H"}]`,
		Sources: []domainknowledge.Source{
			{ChunkID: "chunk-1", ItemID: "item-1", SourceType: domainknowledge.SourceImportedDoc, FilePath: "notes/a.md", Heading: "H", Concept: "Channels", Score: 0.9, Excerpt: "..."},
		},
	}
	retriever.EXPECT().Retrieve(mock.Anything, "session-1", mock.AnythingOfType("string")).Return(result, nil).Once()
	llm.EXPECT().ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, nil).Once()

	studyService := study.NewService(sessions, messages, llm, profiles, folders, retriever, passthroughTransactor{}, catalog)
	folderService := folder.NewService(folders, sessions)
	app := NewApp(nil, nil, nil, nil, nil, studyService, folderService, nil, nil, nil, nil)
	app.Startup(context.Background())
	captured := &capturedEvents{}
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		switch eventName {
		case eventStudyChunk:
			captured.chunks = append(captured.chunks, data[0].(StudyChunkEvent))
		case eventStudyDone:
			captured.done = true
		case eventStudySources:
			captured.sources = append(captured.sources, data[0].(StudySourcesEvent))
		}
	}

	// When sending a message in notes mode
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeNotes)

	// Then the emitted sources event carries exactly the DTO fields, in
	// order, derived from the post-cap domain Source (no internal ID or
	// excerpt field exists on the DTO type at all)
	require.NoError(t, err)
	require.Len(t, captured.sources, 1)
	assert.Equal(t, "session-1", captured.sources[0].SessionID)
	require.Len(t, captured.sources[0].Sources, 1)
	assert.Equal(t, StudySourceResult{
		SourceType: domainknowledge.SourceImportedDoc, FilePath: "notes/a.md", Heading: "H", Concept: "Channels", Score: 0.9,
	}, captured.sources[0].Sources[0])
}

func TestApp_SendStudyMessage_emitsErrorEvent_onFailure(t *testing.T) {
	// Given an App backed by a study service whose LLM call fails, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(normalSession, nil).Once()
	sessions.EXPECT().UpdateContext(mock.Anything, "session-1", mock.Anything).Return(nil).Once()
	messages.EXPECT().Append(mock.Anything, mock.AnythingOfType("study.Message")).Return(nil).Once()
	messages.EXPECT().ListBySession(mock.Anything, "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(mock.Anything, mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, errors.New("upstream failure")).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When sending a message and the LLM call fails
	err := app.SendStudyMessage("session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb)

	// Then the error propagates and a session-scoped error event was
	// emitted, with no done event and no stable code (this happens after
	// the user message was already persisted)
	require.Error(t, err)
	require.Len(t, captured.errors, 1)
	assert.Equal(t, "session-1", captured.errors[0].SessionID)
	assert.Empty(t, captured.errors[0].Code)
	assert.False(t, captured.done)
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

func TestApp_ResumeStudySession_returnsSessionAndHistory(t *testing.T) {
	// Given an App backed by a study service with a session that has one
	// prior message and an unresolved (never-measured) context
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Distributed systems", FolderID: "default", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal}}
	sessions.EXPECT().GetByID(mock.Anything, "session-1").Return(session, nil).Once()
	messages.EXPECT().
		ListBySession(mock.Anything, "session-1").
		Return([]domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Hi"}}, nil).
		Once()
	app, captured := newTestStudyApp(t, sessions, messages, llm, profiles, folders)

	// When resuming the session
	result, err := app.ResumeStudySession("session-1")

	// Then its session and history are returned, with the model unresolved
	// (blank model, zero context length) surfacing the transient
	// unavailable notice rather than attempting a background refresh
	require.NoError(t, err)
	assert.Equal(t, "session-1", result.Session.ID)
	assert.Equal(t, string(domainstudy.ContextStateNormal), result.Session.Context.State)
	require.Len(t, result.Messages, 1)
	assert.Equal(t, "Hi", result.Messages[0].Content)
	require.Len(t, captured.contextUnavailable, 1)
	assert.Equal(t, "session-1", captured.contextUnavailable[0].SessionID)
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
