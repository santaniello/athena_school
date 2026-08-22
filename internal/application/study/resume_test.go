package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestResume_returnsSessionAndFullHistory(t *testing.T) {
	// Given a service with a session that has two prior messages and no
	// resolved model yet (zero-value Context)
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	folders := foldermocks.NewMockRepository(t)
	session := domainstudy.Session{ID: "session-1", Topic: "Topic", FolderID: "default"}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(session, nil).Once()
	history := []domainstudy.Message{{Role: domainstudy.RoleUser, Content: "Hi"}}
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When resuming the session
	got, msgs, err := service.Resume(context.Background(), "session-1", nil, nil)

	// Then the session and its full history are returned; no model is
	// resolved (Model == "") so no catalog/tx call happens either
	require.NoError(t, err)
	require.Equal(t, session, got)
	require.Equal(t, history, msgs)
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
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

	// When resuming a session that does not exist
	_, _, err := service.Resume(context.Background(), "missing", nil, nil)

	// Then the error propagates
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
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
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, nil)

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
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	catalog.EXPECT().CachedContextLength("model-a").Return(1000, true).Once()
	sessions.EXPECT().
		UpdateContext(context.Background(), "session-1", mock.MatchedBy(func(usage domainstudy.ContextUsage) bool {
			return usage.ContextLength == 1000 && usage.State == domainstudy.ContextStateBlocked
		})).
		Return(nil).
		Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, tx, catalog)

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
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	catalog.EXPECT().CachedContextLength("model-a").Return(0, false).Once()
	// Returning (0, nil) — still unresolved after the refresh — routes the
	// background goroutine into the unavailable-notice branch, which we use
	// as this test's "the background call finished" signal below; it takes
	// neither the tx nor the emitContextTransition path.
	catalog.EXPECT().RefreshContextLength(mock.Anything, "model-a").Return(0, nil).Once()

	retriever := knowledgemocks.NewMockRetriever(t)
	service := NewService(sessions, messages, llm, profiles, folders, retriever, nil, catalog)

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
