package study

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// mockNoMessageSources returns a MockMessageSourceRepository expecting
// exactly one ListByMessageIDs call for messageIDs, returning no persisted
// sources for any message.
func mockNoMessageSources(t *testing.T, messageIDs ...string) *knowledgemocks.MockMessageSourceRepository {
	t.Helper()
	if messageIDs == nil {
		messageIDs = []string{}
	}
	messageSources := knowledgemocks.NewMockMessageSourceRepository(t)
	messageSources.EXPECT().
		ListByMessageIDs(context.Background(), messageIDs).
		Return(map[string][]domainknowledge.Source{}, nil).
		Once()
	return messageSources
}

// expectLatestPage expects the cursor-less ListPage call Resume makes: one more than
// the default page size, to learn whether older history exists.
func expectLatestPage(messages *studymocks.MockMessageRepository, newestFirst []domainstudy.Message) {
	messages.EXPECT().
		ListPage(context.Background(), "session-1", (*domainstudy.MessageCursor)(nil), DefaultMessagePageSize+1).
		Return(newestFirst, nil).
		Once()
}

func TestResume_returnsSessionAndItsLatestPage(t *testing.T) {
	// Given a service with a session that has two prior messages and no
	// resolved model yet (zero-value Context)
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	history := []domainstudy.Message{{ID: "message-1", Role: domainstudy.RoleUser, Content: "Hi"}}
	expectLatestPage(messages, history)
	retriever := knowledgemocks.NewMockRetriever(t)
	messageSources := mockNoMessageSources(t, "message-1")
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil, messageSources, nil, nil)

	// When resuming the session
	got, page, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the session and its only page are returned; no model is
	// resolved (Model == "") so no catalog/tx call happens either
	require.NoError(t, err)
	require.Equal(t, session, got)
	require.Equal(t, []MessageWithSources{{Message: history[0]}}, page.Messages)
	require.False(t, page.HasMore)
	require.Empty(t, page.NextCursor)
}

func TestResume_attachesPersistedSourcesToTheMessagesThatHaveThem(t *testing.T) {
	// Given a session with two assistant messages, only one of which has
	// persisted sources
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	history := []domainstudy.Message{
		{ID: "message-1", Role: domainstudy.RoleUser, Content: "What are goroutines?"},
		{ID: "message-2", Role: domainstudy.RoleAssistant, Content: "Lightweight threads."},
		{ID: "message-3", Role: domainstudy.RoleAssistant, Content: "General knowledge answer."},
	}
	expectLatestPage(messages, []domainstudy.Message{history[2], history[1], history[0]})
	persistedSources := []domainknowledge.Source{{ChunkID: "chunk-1", Concept: "Goroutines", Score: 0.9}}
	messageSources := knowledgemocks.NewMockMessageSourceRepository(t)
	messageSources.EXPECT().
		ListByMessageIDs(context.Background(), []string{"message-1", "message-2", "message-3"}).
		Return(map[string][]domainknowledge.Source{"message-2": persistedSources}, nil).
		Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil, messageSources, nil, nil)

	// When resuming the session
	_, page, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then only the message with persisted sources carries them
	require.NoError(t, err)
	require.Equal(t, []MessageWithSources{
		{Message: history[0]},
		{Message: history[1], Sources: persistedSources},
		{Message: history[2]},
	}, page.Messages)
}

func TestResume_propagatesSessionNotFound(t *testing.T) {
	// Given a service backed by a repository with no such session
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "missing").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil, nil, nil, nil)

	// When resuming a session that does not exist
	_, _, err := service.Resume(context.Background(), "missing", nil, nil)

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}

func TestResume_propagatesMessageSourcesListError(t *testing.T) {
	// Given a message-source repository that fails to load
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, nil)
	listErr := errors.New("database unavailable")
	messageSources := knowledgemocks.NewMockMessageSourceRepository(t)
	messageSources.EXPECT().ListByMessageIDs(context.Background(), []string{}).Return(nil, listErr).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil, messageSources, nil, nil)

	// When resuming the session
	_, _, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the error propagates
	require.ErrorIs(t, err, listErr)
}

func TestResume_unresolvedContextLength_emptyModel_callsOnContextUnavailable_withoutTouchingCatalog(t *testing.T) {
	// Given a session whose last stream never resolved a model at all
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal, Model: ""}}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, nil)
	retriever := knowledgemocks.NewMockRetriever(t)
	messageSources := mockNoMessageSources(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil, messageSources, nil, nil)

	var unavailableMsg string
	// When resuming
	got, _, err := service.Resume(context.Background(), "session-1", nil, func(msg string) { unavailableMsg = msg })

	// Then the unavailable notice fires and the session comes back unchanged
	// (no catalog port was even passed — nil — proving it's never touched)
	require.NoError(t, err)
	require.Equal(t, unavailableContextMessage, unavailableMsg)
	require.Equal(t, session, got)
}

func TestResume_unresolvedContextLength_cacheHit_recomputesAndPersistsBeforeReturning(t *testing.T) {
	// Given a session with a known model but an unresolved (zero) context
	// length, and a catalog that has that model cached
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	runWithinTx(tx)
	catalog := llmmocks.NewMockModelContextResolver(t)

	session := domainstudy.Session{
		ID: "session-1",
		Context: domainstudy.ContextUsage{
			State: domainstudy.ContextStateNormal, Model: "model-a", UsedTokens: 950, ContextLength: 0,
		},
	}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, nil)
	catalog.EXPECT().CachedContextLength("model-a").Return(1000, true).Once()
	sessions.EXPECT().
		UpdateContext(context.Background(), "session-1", mock.MatchedBy(func(usage domainstudy.ContextUsage) bool {
			return usage.ContextLength == 1000 && usage.State == domainstudy.ContextStateBlocked
		})).
		Return(nil).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	messageSources := mockNoMessageSources(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, catalog, messageSources, nil, nil)

	// When resuming
	got, _, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the DTO already reflects the newly resolved state — no event is
	// needed since the caller reads it straight off the returned session
	require.NoError(t, err)
	require.Equal(t, 1000, got.Context.ContextLength)
	require.Equal(t, domainstudy.ContextStateBlocked, got.Context.State)
}

func TestResume_unresolvedContextLength_cacheMiss_startsBackgroundRefresh_withoutBlockingReturn(t *testing.T) {
	// Given a session with a known model whose length isn't cached yet
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	catalog := llmmocks.NewMockModelContextResolver(t)

	session := domainstudy.Session{
		ID:      "session-1",
		Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal, Model: "model-a", ContextLength: 0},
	}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, nil)
	catalog.EXPECT().CachedContextLength("model-a").Return(0, false).Once()
	// Returning (0, nil) — still unresolved after the refresh — routes the
	// background goroutine into the unavailable-notice branch, which we use
	// as this test's "the background call finished" signal below; it takes
	// neither the tx nor the emitContextTransition path.
	catalog.EXPECT().RefreshContextLength(mock.Anything, "model-a").Return(0, nil).Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	messageSources := mockNoMessageSources(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, catalog, messageSources, nil, nil)

	done := make(chan struct{})
	// When resuming
	got, _, err := service.Resume(context.Background(), "session-1", nil, func(string) { close(done) })

	// Then Resume returns immediately with the preserved (still-unresolved)
	// state — it never waits on the background refresh
	require.NoError(t, err)
	require.Equal(t, session, got)

	// Wait for the background refresh to actually finish before the test
	// (and mockery's t.Cleanup expectation check) exits, so the goroutine's
	// mock call can never race with it.
	<-done
}

func TestResume_returnsTheSummaryCursorWhenTheSessionHasASummary(t *testing.T) {
	// Given a session that was compacted up to message-3
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	session := domainstudy.Session{ID: "session-1"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	expectLatestPage(messages, nil)
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{SessionID: "session-1", Content: "## Progress", CoversUntilMessageID: "message-3", UpdatedAt: time.Now()}, nil).
		Once()
	service := NewService(sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t),
		knowledgemocks.NewMockRetriever(t), nil, nil, mockNoMessageSources(t), nil, nil, WithSummaries(summaries))

	// When resuming the session
	_, history, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the history carries the cursor of the summary
	require.NoError(t, err)
	require.Equal(t, "message-3", history.SummaryCoversUntilMessageID)
}

func TestResume_hasNoSummaryCursorWhenTheSessionHasNoSummary(t *testing.T) {
	// Given a session that was never compacted
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1"}, nil).Once()
	expectLatestPage(messages, nil)
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	service := NewService(sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t),
		knowledgemocks.NewMockRetriever(t), nil, nil, mockNoMessageSources(t), nil, nil, WithSummaries(summaries))

	// When resuming the session
	_, history, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then resume succeeds without a cursor
	require.NoError(t, err)
	require.Empty(t, history.SummaryCoversUntilMessageID)
}

func TestResume_propagatesSummaryReadError(t *testing.T) {
	// Given a summary repository that fails to read
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1"}, nil).Once()
	expectLatestPage(messages, nil)
	readErr := errors.New("database unavailable")
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, readErr).Once()
	service := NewService(sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t),
		knowledgemocks.NewMockRetriever(t), nil, nil, mockNoMessageSources(t), nil, nil, WithSummaries(summaries))

	// When resuming the session
	_, _, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the error propagates
	require.ErrorIs(t, err, readErr)
}
