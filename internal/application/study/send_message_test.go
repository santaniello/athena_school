package study

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// mockNormalSession sets up sessions.GetByID to return a fresh,
// unmeasured (ContextStateNormal) session, and sessions.UpdateContext to
// accept any write — the shape every SendMessage test needs once it's past
// source-mode/content validation, since the initial transaction always
// reads and then updates the session's ContextUsage. Returns the tx double
// SendMessage's own Transactor.WithinTx calls route through.
func mockNormalSession(t *testing.T, sessions *studymocks.MockSessionRepository, sessionID string) *applicationstudymocks.MockTransactor {
	t.Helper()
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)
	sessions.EXPECT().GetByID(context.Background(), sessionID).Return(normalSession(sessionID), nil).Once()
	sessions.EXPECT().UpdateContext(context.Background(), sessionID, mock.Anything).Return(nil)
	return tx
}

func TestSendMessage_returnsErrInvalidSourceMode_forUnknownMode(t *testing.T) {
	// Given a service and an unknown source mode; no mock has any .EXPECT()
	// set, so any provider/store/repository call would fail the test
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When sending a message with an unrecognized source mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", "bogus-mode", noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then it fails with ErrInvalidSourceMode before persisting or making
	// any provider/store call
	require.ErrorIs(t, err, domainknowledge.ErrInvalidSourceMode)
}

func TestSendMessage_returnsErrInvalidSourceMode_beforeBlankContentCheck(t *testing.T) {
	// Given a service, an unknown source mode, and blank content
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When sending a message with both problems
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "   ", "bogus-mode", noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then the invalid mode wins over the blank-content check
	require.ErrorIs(t, err, domainknowledge.ErrInvalidSourceMode)
	require.NotErrorIs(t, err, ErrMessageRequired)
}

func TestSendMessage_returnsMessageRequired_whenContentIsBlank(t *testing.T) {
	// Given a service and blank message content in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When sending a whitespace-only message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "   ", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then it fails with ErrMessageRequired; no port received any call
	require.ErrorIs(t, err, ErrMessageRequired)
}

func TestSendMessage_blockedSession_returnsErrSessionContextLimitReached_withoutPersistingOrCallingAnyPort(t *testing.T) {
	// Given a session already at the blocked context state
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)
	sessions.EXPECT().
		GetByID(context.Background(), "session-1").
		Return(domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateBlocked}}, nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then it's rejected before appending the user message, updating
	// context, or touching retrieval/the LLM (no .EXPECT() set on any of
	// those beyond the read above)
	require.ErrorIs(t, err, domainstudy.ErrSessionContextLimitReached)
}

func TestSendMessage_concurrentCallsForSameSession_secondReturnsErrStudyTurnInProgress(t *testing.T) {
	// Given a session whose stream is currently blocked mid-flight
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil)
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil)
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

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)
	}()
	<-started

	// When a second message is sent for the same session while the first is
	// still in flight
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "Another question", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then it's rejected immediately with ErrStudyTurnInProgress
	require.ErrorIs(t, err, ErrStudyTurnInProgress)

	close(release)
	require.Error(t, <-errCh)
}

func TestSendMessage_persistsUserMessageBeforeCallingLLM(t *testing.T) {
	// Given a service tracking the order ports are called in, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

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
		Return(domainllm.StreamResponse{}, nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
			return m.Role == domainstudy.RoleAssistant
		})).
		Run(func(context.Context, domainstudy.Message) { callOrder = append(callOrder, "append-assistant") }).
		Return(nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then the user message is persisted before the LLM is ever called, and
	// the assistant reply is persisted only after the stream completes;
	// retriever has no .EXPECT() set — web never touches it
	require.NoError(t, err)
	require.Equal(t, []string{"append-user", "list-history", "chat-stream", "append-assistant"}, callOrder)
}

func TestSendMessage_provisionalIncrement_reachingBlocked_emitsContextImmediately_andStillContinuesTheTurn(t *testing.T) {
	// Given a session already at 940/1000 tokens (normal is only true
	// because the model/length hasn't been evaluated against it before —
	// here we start from a state that crosses the 95% boundary the moment
	// the provisional increment is added)
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)

	almostFull := domainstudy.Session{
		ID:      "session-1",
		Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal, Model: "m1", UsedTokens: 940, ContextLength: 1000},
	}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(almostFull, nil).Once()

	var updatedContexts []domainstudy.ContextUsage
	sessions.EXPECT().
		UpdateContext(context.Background(), "session-1", mock.MatchedBy(func(domainstudy.ContextUsage) bool { return true })).
		Run(func(_ context.Context, _ string, u domainstudy.ContextUsage) {
			updatedContexts = append(updatedContexts, u)
		}).
		Return(nil)

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, nil).
		Once()

	var contextEvents []ContextEvent
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message whose provisional estimate alone pushes the
	// session's context usage past the blocked (95%) boundary
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler,
		func(e ContextEvent) { contextEvents = append(contextEvents, e) }, nil)

	// Then the turn is still accepted and completes (blocked only prevents
	// *later* turns), and the first UpdateContext call already reached
	// ContextStateBlocked
	require.NoError(t, err)
	require.NotEmpty(t, updatedContexts)
	require.Equal(t, domainstudy.ContextStateBlocked, updatedContexts[0].State)
	require.NotEmpty(t, contextEvents)
	require.Equal(t, domainstudy.ContextStateBlocked, contextEvents[0].State)
}

func TestSendMessage_sendsHistoryAndFreshSystemPromptToLLM(t *testing.T) {
	// Given a service with two prior turns in history, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

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
		Return(domainllm.StreamResponse{}, nil).
		Once()
	messages.EXPECT().
		Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
			return m.Role == domainstudy.RoleAssistant && m.Content == "It stands for..."
		})).
		Return(nil).
		Once()

	var received []string
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, func(chunk string) error {
		received = append(received, chunk)
		return nil
	}, nil, nil)

	// Then it succeeds and forwarded the streamed chunk
	require.NoError(t, err)
	require.Equal(t, []string{"It stands for..."}, received)
}

func TestSendMessage_propagatesStreamError_withoutPersistingAssistantMessage(t *testing.T) {
	// Given a service whose LLM call fails mid-stream, after the user
	// message has already been persisted, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	streamErr := errors.New("upstream failure")
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, streamErr).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then the error propagates; the "assistant" Append above has no
	// .EXPECT(), so an unexpected call would fail the test
	require.ErrorIs(t, err, streamErr)
}

func TestSendMessage_web_callsOnSourcesOnceWithEmptySlice_beforeAnyChunk(t *testing.T) {
	// Given a service in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)

	var callOrder []string
	var receivedSources []domainknowledge.Source
	llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			callOrder = append(callOrder, "chat-stream")
			require.NoError(t, handler("chunk"))
		}).
		Return(domainllm.StreamResponse{}, nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb,
		func(sources []domainknowledge.Source) error {
			callOrder = append(callOrder, "sources")
			receivedSources = sources
			return nil
		},
		func(string) error {
			callOrder = append(callOrder, "chunk")
			return nil
		},
		nil, nil,
	)

	// Then onSources fired exactly once, with an empty (non-nil) slice,
	// before the chat call and before any chunk
	require.NoError(t, err)
	require.Equal(t, []string{"sources", "chat-stream", "chunk"}, callOrder)
	require.Equal(t, []domainknowledge.Source{}, receivedSources)
}

func TestSendMessage_notes_returnsErrVectorStoreUnavailable_preservingUserMessage_noEmbeddingOrChatCall(t *testing.T) {
	// Given a notes-mode retrieval that fails because the index has no
	// valid snapshot
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable).Once()

	sourcesCalled := false
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message in notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeNotes,
		func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler, nil, nil)

	// Then the error propagates; the user message stayed persisted (its
	// .EXPECT() above was satisfied), no assistant message was appended (no
	// .EXPECT() for it), llm/profiles/folders had no .EXPECT() set, and
	// onSources was never called
	require.ErrorIs(t, err, domainknowledge.ErrVectorStoreUnavailable)
	require.False(t, sourcesCalled)
}

func TestSendMessage_strictNotes_returnsErrVectorStoreUnavailable_sameGuarantees(t *testing.T) {
	// Given a strict-notes retrieval that fails because the index has no
	// valid snapshot
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable).Once()

	sourcesCalled := false
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message in strict-notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeStrictNotes,
		func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler, nil, nil)

	// Then the same guarantees hold as for notes mode
	require.ErrorIs(t, err, domainknowledge.ErrVectorStoreUnavailable)
	require.False(t, sourcesCalled)
}

func TestSendMessage_buildsQueryFromTopicAndTrimmedMessage_excludingHistory_carryingSessionID(t *testing.T) {
	// Given a session with prior history
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.Message{
		{Role: domainstudy.RoleUser, Content: "Hi"},
		{Role: domainstudy.RoleAssistant, Content: "Hello!"},
	}, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	retriever.EXPECT().
		Retrieve(context.Background(), "session-1", "Topic: Distributed systems\n\nMessage: What is CAP theorem?").
		Return(domainknowledge.RetrievalResult{}, nil).
		Once()
	llm.EXPECT().ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).Return(domainllm.StreamResponse{}, nil).Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message with leading/trailing whitespace
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "  What is CAP theorem?  ", domainknowledge.SourceModeNotes, noopSourcesHandler, noopChunkHandler, nil, nil)

	// Then the query carried only the topic and the trimmed current
	// message, with no history, and the session ID (see retriever.EXPECT()
	// above's exact-matched arguments)
	require.NoError(t, err)
}

func TestSendMessage_notes_fallsThroughToPlainChat_onValidEmptyOrMissRetrieval(t *testing.T) {
	// Given a successful retrieval with no surviving chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, nil).Once()

	var receivedSources []domainknowledge.Source
	llm.EXPECT().
		ChatStream(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
			// Same shape as web: just the system prompt, no second system message
			return len(req.Messages) == 1 && req.Messages[0].Role == "system"
		}), mock.AnythingOfType("func(string) error")).
		Return(domainllm.StreamResponse{}, nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message in notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeNotes,
		func(sources []domainknowledge.Source) error { receivedSources = sources; return nil }, noopChunkHandler, nil, nil)

	// Then it falls through to a plain chat call, and onSources received an
	// empty (non-nil) slice
	require.NoError(t, err)
	require.Equal(t, []domainknowledge.Source{}, receivedSources)
}

func TestSendMessage_strictNotes_persistsFixedMessage_onValidEmptyOrMissRetrieval_noChatCompletionCall(t *testing.T) {
	// Given a successful strict-notes retrieval with no surviving chunks
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	tx := mockNormalSession(t, sessions, "session-1")

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleAssistant && m.Content == domainknowledge.NoLocalKnowledgeMessage
	})).Return(nil).Once()
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, nil).Once()

	var callOrder []string
	var receivedSources []domainknowledge.Source
	var receivedChunks []string
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

	// When sending a message in strict-notes mode; llm/profiles/folders have
	// no .EXPECT() set, so no chat/completion call and no history/profile
	// load happen on this branch
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeStrictNotes,
		func(sources []domainknowledge.Source) error {
			callOrder = append(callOrder, "sources")
			receivedSources = sources
			return nil
		},
		func(chunk string) error {
			callOrder = append(callOrder, "chunk")
			receivedChunks = append(receivedChunks, chunk)
			return nil
		},
		nil, nil,
	)

	// Then it succeeds, delivering the fixed message through the normal
	// chunk callback after emitting empty sources
	require.NoError(t, err)
	require.Equal(t, []string{"sources", "chunk"}, callOrder)
	require.Equal(t, []domainknowledge.Source{}, receivedSources)
	require.Equal(t, []string{domainknowledge.NoLocalKnowledgeMessage}, receivedChunks)
}

func TestSendMessage_local_propagatesGenericRetrievalError_persistingOnlyUserMessage(t *testing.T) {
	for _, mode := range []string{domainknowledge.SourceModeNotes, domainknowledge.SourceModeStrictNotes} {
		t.Run(mode, func(t *testing.T) {
			// Given a retrieval call that fails with a generic technical error
			sessions := studymocks.NewMockSessionRepository(t)
			messages := studymocks.NewMockMessageRepository(t)
			llm := llmmocks.NewMockProvider(t)
			profiles := profilemocks.NewMockStore(t)
			folders := foldermocks.NewMockRepository(t)
			retriever := knowledgemocks.NewMockRetriever(t)
			tx := mockNormalSession(t, sessions, "session-1")

			messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
				return m.Role == domainstudy.RoleUser
			})).Return(nil).Once()
			retrievalErr := errors.New("embedding provider unavailable")
			retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
				Return(domainknowledge.RetrievalResult{}, retrievalErr).Once()

			sourcesCalled := false
			service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

			// When sending a message
			err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", mode,
				func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler, nil, nil)

			// Then the error propagates; no assistant message, no onSources
			// call, no llm call (none of their .EXPECT()s were set beyond the
			// user Append above)
			require.ErrorIs(t, err, retrievalErr)
			require.False(t, sourcesCalled)
		})
	}
}

func TestSendMessage_localMode_withSurvivingChunks_sendsKnowledgeContextAsSecondSystemMessage(t *testing.T) {
	cases := []struct {
		name       string
		mode       string
		sufficient bool
	}{
		{"notes_sufficient", domainknowledge.SourceModeNotes, true},
		{"notes_insufficientButNonEmpty", domainknowledge.SourceModeNotes, false},
		{"strictNotes_sufficient", domainknowledge.SourceModeStrictNotes, true},
		{"strictNotes_insufficientButNonEmpty", domainknowledge.SourceModeStrictNotes, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given a retrieval result with surviving chunks
			sessions := studymocks.NewMockSessionRepository(t)
			messages := studymocks.NewMockMessageRepository(t)
			llm := llmmocks.NewMockProvider(t)
			profiles := profilemocks.NewMockStore(t)
			folders := foldermocks.NewMockRepository(t)
			retriever := knowledgemocks.NewMockRetriever(t)
			tx := mockNormalSession(t, sessions, "session-1")

			sources := []domainknowledge.Source{{ChunkID: "chunk-1", SourceType: domainknowledge.SourceImportedDoc, Concept: "Channels", Score: 0.9}}
			result := domainknowledge.RetrievalResult{
				Chunks:     []domainknowledge.ScoredChunk{{Chunk: domainknowledge.Chunk{ID: "chunk-1"}, Score: 0.9}},
				Sufficient: c.sufficient,
				Context:    `[{"heading":"H"}]`,
				Sources:    sources,
			}
			expectedKnowledgeMessage := buildKnowledgeContext(result, c.mode)

			messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
			messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
			profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
			retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).Return(result, nil).Once()

			llm.EXPECT().
				ChatStream(context.Background(), mock.MatchedBy(func(req domainllm.ChatRequest) bool {
					return len(req.Messages) == 2 && req.Messages[0].Role == "system" && req.Messages[1] == expectedKnowledgeMessage
				}), mock.AnythingOfType("func(string) error")).
				Return(domainllm.StreamResponse{}, nil).
				Once()

			var receivedSources []domainknowledge.Source
			service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, nil)

			// When sending a message
			err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", c.mode,
				func(s []domainknowledge.Source) error { receivedSources = s; return nil }, noopChunkHandler, nil, nil)

			// Then the LLM was called with the second system message built
			// from the exact same result/mode, and onSources received the
			// exact post-cap sources
			require.NoError(t, err)
			require.Equal(t, sources, receivedSources)
		})
	}
}
