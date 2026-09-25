package knowledge

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

// captureLog redirects the standard logger into the returned buffer for the
// duration of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buffer bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&buffer)
	t.Cleanup(func() { log.SetOutput(previous) })
	return &buffer
}

func newSessionDeleteService(
	chunks *knowledgemocks.MockChunkRepository, evidence *knowledgemocks.MockEvidenceRepository,
	tx *txmocks.MockTransactor, store *knowledgemocks.MockVectorStore, guard IndexGuard,
) *Service {
	return NewService(nil, nil, chunks, tx, store, guard, domainknowledge.RetrievalThresholds{}, evidence)
}

func TestDeleteSessionsWithKnowledge_collectsChunkIDsBeforeTheDelete_thenCleansEvidence_thenEvictsTheIndex(t *testing.T) {
	// Given two sessions that own chunks
	ctx := context.Background()
	var order []string
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Run(func(context.Context, string) {
		order = append(order, "ids-a")
	}).Return([]string{"chunk-a1", "chunk-a2"}, nil).Once()
	chunks.EXPECT().ListIDsBySession(ctx, "session-b").Run(func(context.Context, string) {
		order = append(order, "ids-b")
	}).Return([]string{"chunk-b1"}, nil).Once()
	evidence := knowledgemocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().DeleteUnreferenced(ctx).Run(func(context.Context) {
		order = append(order, "evidence")
	}).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-a1", "chunk-a2", "chunk-b1"}).Run(func(context.Context, []string) {
		order = append(order, "evict")
	}).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newSessionDeleteService(chunks, evidence, tx, store, passingIndexGuard(t))
	logged := captureLog(t)

	// When deleting them together with their knowledge
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a", "session-b"}, func(context.Context) error {
		order = append(order, "delete")
		return nil
	})

	// Then the chunk IDs are read before the sessions (and, through the
	// foreign-key cascade, their chunks) disappear, orphaned evidence is
	// cleaned in the same transaction, and the index is evicted last
	require.NoError(t, err)
	assert.Equal(t, []string{"ids-a", "ids-b", "delete", "evidence", "evict"}, order)
	assert.Empty(t, logged.String(), "a clean eviction logs nothing")
}

func TestDeleteSessionsWithKnowledge_returnsTheDeleteError_withoutCleaningEvidenceOrEvictingTheIndex(t *testing.T) {
	// Given a session delete that fails
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Return([]string{"chunk-a1"}, nil).Once()
	boom := errors.New("disk full")
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newSessionDeleteService(chunks, knowledgemocks.NewMockEvidenceRepository(t), tx, knowledgemocks.NewMockVectorStore(t), passingIndexGuard(t))

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error { return boom })

	// Then the error propagates; evidence cleanup and index eviction never
	// run (the mocks would fail on any unexpected call), so nothing is
	// half-cleaned after the rollback
	assert.ErrorIs(t, err, boom)
}

func TestDeleteSessionsWithKnowledge_returnsTheListingError_withoutRunningTheDelete(t *testing.T) {
	// Given a chunk repository that cannot list a session's chunks
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	boom := errors.New("disk full")
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Return(nil, boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newSessionDeleteService(chunks, nil, tx, nil, passingIndexGuard(t))
	deleted := false

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error {
		deleted = true
		return nil
	})

	// Then the error propagates and the sessions are left alone, since their
	// chunks could not be accounted for
	assert.ErrorIs(t, err, boom)
	assert.False(t, deleted)
}

func TestDeleteSessionsWithKnowledge_returnsTheEvidenceCleanupError(t *testing.T) {
	// Given a delete that succeeds but an evidence cleanup that fails
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Return(nil, nil).Once()
	boom := errors.New("disk full")
	evidence := knowledgemocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().DeleteUnreferenced(ctx).Return(boom).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newSessionDeleteService(chunks, evidence, tx, knowledgemocks.NewMockVectorStore(t), passingIndexGuard(t))

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error { return nil })

	// Then the error propagates so the enclosing transaction rolls the delete back
	assert.ErrorIs(t, err, boom)
}

func TestDeleteSessionsWithKnowledge_skipsTheIndexEviction_whenNoChunkWasOwned(t *testing.T) {
	// Given sessions that own no chunks at all
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Return([]string{}, nil).Once()
	evidence := knowledgemocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().DeleteUnreferenced(ctx).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	// no Remove expectation: the store mock fails the test on any call
	service := newSessionDeleteService(chunks, evidence, tx, knowledgemocks.NewMockVectorStore(t), passingIndexGuard(t))

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error { return nil })

	// Then it succeeds without touching the index
	require.NoError(t, err)
}

func TestDeleteSessionsWithKnowledge_succeeds_whenTheIndexEvictionFails_afterTheDurableDelete(t *testing.T) {
	// Given a durable delete followed by a failing index eviction
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().ListIDsBySession(ctx, "session-a").Return([]string{"chunk-a1"}, nil).Once()
	evidence := knowledgemocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().DeleteUnreferenced(ctx).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-a1"}).Return(errors.New("store exploded")).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newSessionDeleteService(chunks, evidence, tx, store, passingIndexGuard(t))
	logged := captureLog(t)

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error { return nil })

	// Then the delete still reports success — the sessions are gone and their
	// leftover in-memory chunks are unreachable, since retrieval is scoped by
	// session — and the eviction failure is logged, not swallowed silently
	require.NoError(t, err)
	assert.Contains(t, logged.String(), "store exploded")
}

func TestDeleteSessionsWithKnowledge_returnsErrIndexLoading_whenIndexIsLoading_andNeverDeletes(t *testing.T) {
	// Given a loading index
	ctx := context.Background()
	guard := txmocks.NewMockIndexGuard(t)
	guard.EXPECT().BeginMutation().Return(ErrIndexLoading).Once()
	service := newSessionDeleteService(nil, nil, nil, nil, guard)
	deleted := false

	// When deleting
	err := service.DeleteSessionsWithKnowledge(ctx, []string{"session-a"}, func(context.Context) error {
		deleted = true
		return nil
	})

	// Then the mutation is rejected before anything runs
	assert.ErrorIs(t, err, ErrIndexLoading)
	assert.False(t, deleted)
}
