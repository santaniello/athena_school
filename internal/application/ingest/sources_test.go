package ingest

import (
	"bytes"
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
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

func TestListSources_returnsTheRepositorysSessionSources(t *testing.T) {
	// Given a session with two imported documents
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	want := []domainknowledge.SessionSource{
		{ItemID: "item-1", Title: "Distributed Systems", Path: "notes/ds.md", ChunkCount: 12, IngestedAt: time.Unix(1, 0)},
		{ItemID: "item-2", Title: "CAP theorem", Path: "cap.md", ChunkCount: 4, IngestedAt: time.Unix(2, 0)},
	}
	ingestedFiles.EXPECT().ListSourcesBySession(ctx, testSessionID).Return(want, nil).Once()
	service := newTestService(nil, ingestedFiles, nil, nil, nil, nil, nil)

	// When listing the session's sources
	got, err := service.ListSources(ctx, testSessionID)

	// Then it returns exactly what the repository reported
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestListSources_returnsTheRepositoryError(t *testing.T) {
	// Given a repository that fails to list
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	boom := errors.New("disk full")
	ingestedFiles.EXPECT().ListSourcesBySession(ctx, testSessionID).Return(nil, boom).Once()
	service := newTestService(nil, ingestedFiles, nil, nil, nil, nil, nil)

	// When listing the session's sources
	_, err := service.ListSources(ctx, testSessionID)

	// Then the failure is reported
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestRemoveSource_deletesChunksItemAndIngestedFile_thenEvictsTheIndex(t *testing.T) {
	// Given a document owned by the session
	ctx := context.Background()
	var order []string
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "delete-chunks")
	}).Return([]string{"chunk-1", "chunk-2"}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	items.EXPECT().Delete(ctx, "item-1").Run(func(context.Context, string) {
		order = append(order, "delete-item")
	}).Return(nil).Once()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().DeleteByItemID(ctx, testSessionID, "item-1").Run(func(context.Context, string, string) {
		order = append(order, "delete-ingested-file")
	}).Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1", "chunk-2"}).Run(func(context.Context, []string) {
		order = append(order, "evict")
	}).Return(nil).Once()
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newTestService(chunks, ingestedFiles, items, nil, tx, store, passingIndexGuard(t))

	// When removing it
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then every deletion happens inside the transaction, in order, and
	// the index is evicted only after it commits
	require.NoError(t, err)
	assert.Equal(t, []string{"delete-chunks", "delete-item", "delete-ingested-file", "evict"}, order)
}

func TestRemoveSource_itemDoesNotExist_returnsErrSourceNotFound_withoutMutating(t *testing.T) {
	// Given an item id nothing owns
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := newTestService(nil, nil, items, nil, nil, nil, passingIndexGuard(t))

	// When removing it
	err := service.RemoveSource(ctx, testSessionID, "missing")

	// Then it reports ErrSourceNotFound and never reaches the transaction
	// (no Transactor mock was even set up, so any call to it would panic)
	require.ErrorIs(t, err, ErrSourceNotFound)
}

func TestRemoveSource_itemBelongsToAnotherSession_returnsErrSourceNotFound_withoutMutating(t *testing.T) {
	// Given an item owned by a different session
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: "other-session"}, nil).Once()
	service := newTestService(nil, nil, items, nil, nil, nil, passingIndexGuard(t))

	// When session-1 tries to remove it
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then it is rejected the same way as a non-existent item, so one
	// session can never probe or delete another's document
	require.ErrorIs(t, err, ErrSourceNotFound)
}

func TestRemoveSource_indexBusy_returnsTheGuardError_withoutLookingUpTheItem(t *testing.T) {
	// Given an index reload in flight
	ctx := context.Background()
	guard := ingestmocks.NewMockIndexGuard(t)
	boom := errors.New("index is retrying")
	guard.EXPECT().BeginMutation().Return(boom).Once()
	service := newTestService(nil, nil, nil, nil, nil, nil, guard)

	// When removing a source
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then the guard's error is returned and nothing else is touched (no
	// items/chunks/tx mock was set up, so any call would panic)
	assert.ErrorIs(t, err, boom)
}

func TestRemoveSource_transactionFails_returnsTheErrorWithoutEvictingTheIndex(t *testing.T) {
	// Given a transaction that fails midway
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	boom := errors.New("disk full")
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, boom).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newTestService(chunks, nil, items, nil, tx, nil, passingIndexGuard(t))

	// When removing it (no VectorStore mock was set up, so an eviction
	// attempt would panic)
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then the failure is reported
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestRemoveSource_noChunks_skipsTheIndexEviction(t *testing.T) {
	// Given a document with no chunks (e.g. an empty file)
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return(nil, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	items.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().DeleteByItemID(ctx, testSessionID, "item-1").Return(nil).Once()
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newTestService(chunks, ingestedFiles, items, nil, tx, nil, passingIndexGuard(t))

	// When removing it (no VectorStore mock was set up, so Remove would panic)
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then it succeeds without touching the index
	require.NoError(t, err)
}

func TestRemoveSource_evictionFails_isLoggedNotReturned(t *testing.T) {
	// Given a commit that succeeds but whose post-commit eviction fails
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	chunks.EXPECT().DeleteByItemID(ctx, "item-1").Return([]string{"chunk-1"}, nil).Once()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1", SessionID: testSessionID}, nil).Once()
	items.EXPECT().Delete(ctx, "item-1").Return(nil).Once()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().DeleteByItemID(ctx, testSessionID, "item-1").Return(nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().Remove(mock.Anything, []string{"chunk-1"}).Return(errors.New("index unavailable")).Once()
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := newTestService(chunks, ingestedFiles, items, nil, tx, store, passingIndexGuard(t))
	logged := captureLog(t)

	// When removing it
	err := service.RemoveSource(ctx, testSessionID, "item-1")

	// Then the durable delete is reported as a success, and the eviction
	// failure only reaches the log
	require.NoError(t, err)
	assert.Contains(t, logged.String(), "item-1")
}
