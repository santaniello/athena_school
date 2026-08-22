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

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSendMessage_returnsErrInvalidSourceMode_forUnknownMode(t *testing.T) {
	// Given a service and an unknown source mode; no mock has any .EXPECT()
	// set, so any provider/store/repository call would fail the test
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message with an unrecognized source mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", "bogus-mode", noopSourcesHandler, noopChunkHandler)

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
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message with both problems
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "   ", "bogus-mode", noopSourcesHandler, noopChunkHandler)

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
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a whitespace-only message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "   ", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler)

	// Then it fails with ErrMessageRequired; no port received any call
	require.ErrorIs(t, err, ErrMessageRequired)
}

func TestSendMessage_persistsUserMessageBeforeCallingLLM(t *testing.T) {
	// Given a service tracking the order ports are called in, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)

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

	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler)

	// Then the user message is persisted before the LLM is ever called, and
	// the assistant reply is persisted only after the stream completes;
	// retriever has no .EXPECT() set — web never touches it
	require.NoError(t, err)
	require.Equal(t, []string{"append-user", "list-history", "chat-stream", "append-assistant"}, callOrder)
}

func TestSendMessage_sendsHistoryAndFreshSystemPromptToLLM(t *testing.T) {
	// Given a service with two prior turns in history, in web mode
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	retriever := knowledgemocks.NewMockRetriever(t)

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
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, func(chunk string) error {
		received = append(received, chunk)
		return nil
	})

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

	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeWeb, noopSourcesHandler, noopChunkHandler)

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
		Return(nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever)

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

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable).Once()

	sourcesCalled := false
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message in notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeNotes,
		func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler)

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

	messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
		return m.Role == domainstudy.RoleUser
	})).Return(nil).Once()
	retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
		Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable).Once()

	sourcesCalled := false
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message in strict-notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeStrictNotes,
		func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler)

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
	llm.EXPECT().ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).Return(nil).Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message with leading/trailing whitespace
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "  What is CAP theorem?  ", domainknowledge.SourceModeNotes, noopSourcesHandler, noopChunkHandler)

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
		Return(nil).
		Once()

	service := NewService(sessions, messages, llm, profiles, folders, retriever)

	// When sending a message in notes mode
	err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", domainknowledge.SourceModeNotes,
		func(sources []domainknowledge.Source) error { receivedSources = sources; return nil }, noopChunkHandler)

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
	service := NewService(sessions, messages, llm, profiles, folders, retriever)

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

			messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool {
				return m.Role == domainstudy.RoleUser
			})).Return(nil).Once()
			retrievalErr := errors.New("embedding provider unavailable")
			retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("string")).
				Return(domainknowledge.RetrievalResult{}, retrievalErr).Once()

			sourcesCalled := false
			service := NewService(sessions, messages, llm, profiles, folders, retriever)

			// When sending a message
			err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", mode,
				func([]domainknowledge.Source) error { sourcesCalled = true; return nil }, noopChunkHandler)

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
				Return(nil).
				Once()

			var receivedSources []domainknowledge.Source
			service := NewService(sessions, messages, llm, profiles, folders, retriever)

			// When sending a message
			err := service.SendMessage(context.Background(), "session-1", "Distributed systems", "What is CAP theorem?", c.mode,
				func(s []domainknowledge.Source) error { receivedSources = s; return nil }, noopChunkHandler)

			// Then the LLM was called with the second system message built
			// from the exact same result/mode, and onSources received the
			// exact post-cap sources
			require.NoError(t, err)
			require.Equal(t, sources, receivedSources)
		})
	}
}
