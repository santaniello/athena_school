package desktop

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

const testEmbeddingModel = "openai/text-embedding-3-small"

func testIndexChunk(id string) domainknowledge.Chunk {
	return domainknowledge.Chunk{
		ID: id, Source: domainknowledge.SourceImportedDoc, Topic: "Go",
		Status: domainknowledge.StatusApproved, ItemID: "item-" + id,
		Embedding: []float32{1, 0},
	}
}

func TestApp_GetKnowledgeIndexStatus_startsLoading_beforeTheInitialLoadRuns(t *testing.T) {
	// Given an App wired with a freshly constructed index loader that has
	// never been started
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	loader := applicationknowledge.NewIndexLoader(chunks, store, testEmbeddingModel)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, loader)
	app.Startup(context.Background())

	// When querying its status
	status := app.GetKnowledgeIndexStatus()

	// Then it already reports loading with no snapshot
	assert.Equal(t, "loading", status.State)
	assert.False(t, status.HasSnapshot)
}

func TestApp_StartKnowledgeIndex_loadsThenEmitsStatus(t *testing.T) {
	// Given an App wired with a real index loader whose repository/store
	// both succeed
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	valid := []domainknowledge.Chunk{testIndexChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: valid}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().ReplaceAll(ctx, valid).Return(nil).Once()
	loader := applicationknowledge.NewIndexLoader(chunks, store, testEmbeddingModel)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, loader)
	app.Startup(ctx)

	var emitted []IndexStatusResult
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName == eventKnowledgeIndexStatus {
			emitted = append(emitted, data[0].(IndexStatusResult))
		}
	}

	// When the initial load runs (as Wails' OnDomReady would trigger)
	app.StartKnowledgeIndex(ctx)

	// Then the status becomes ready and the same outcome is emitted
	status := app.GetKnowledgeIndexStatus()
	assert.Equal(t, "ready", status.State)
	assert.True(t, status.HasSnapshot)
	require.Len(t, emitted, 1)
	assert.Equal(t, "ready", emitted[0].State)
}

func TestApp_GetKnowledgeIndexStatus_reportsIssues_afterAPartiallyValidLoad(t *testing.T) {
	// Given a repository reporting one valid chunk and one isolated issue
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	valid := []domainknowledge.Chunk{testIndexChunk("c1")}
	issue := domainknowledge.ChunkLoadIssue{
		ChunkID: "c2", ItemID: "item-c2", Source: domainknowledge.SourceImportedDoc,
		FilePath: "notes/c2.md", Reason: domainknowledge.ChunkIssueMissingItem,
	}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).
		Return(domainknowledge.ChunkLoadResult{Chunks: valid, Issues: []domainknowledge.ChunkLoadIssue{issue}}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().ReplaceAll(ctx, valid).Return(nil).Once()
	loader := applicationknowledge.NewIndexLoader(chunks, store, testEmbeddingModel)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, loader)
	app.Startup(ctx)
	app.emit = func(context.Context, string, ...interface{}) {}

	// When the initial load runs and the status is queried
	app.StartKnowledgeIndex(ctx)
	status := app.GetKnowledgeIndexStatus()

	// Then the isolated issue survives translation to the desktop DTO with
	// only its safe fields
	assert.Equal(t, "ready_with_warnings", status.State)
	require.Len(t, status.Issues, 1)
	assert.Equal(t, "c2", status.Issues[0].ChunkID)
	assert.Equal(t, "notes/c2.md", status.Issues[0].FilePath)
	assert.Equal(t, domainknowledge.ChunkIssueMissingItem, status.Issues[0].Reason)
}

func TestApp_RetryKnowledgeIndex_publishesANewSnapshot_andEmitsStatus(t *testing.T) {
	// Given an App whose index already completed a successful initial load
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	first := []domainknowledge.Chunk{testIndexChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := applicationknowledge.NewIndexLoader(chunks, store, testEmbeddingModel)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, loader)
	app.Startup(ctx)
	app.emit = func(context.Context, string, ...interface{}) {}
	app.StartKnowledgeIndex(ctx)

	var emitted []IndexStatusResult
	app.emit = func(_ context.Context, eventName string, data ...interface{}) {
		if eventName == eventKnowledgeIndexStatus {
			emitted = append(emitted, data[0].(IndexStatusResult))
		}
	}
	second := []domainknowledge.Chunk{testIndexChunk("c2")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: second}, nil).Once()
	store.EXPECT().ReplaceAll(ctx, second).Return(nil).Once()

	// When retrying through the desktop adapter
	result := app.RetryKnowledgeIndex()

	// Then it publishes the new snapshot, returns it, and emits the same outcome
	assert.Equal(t, "ready", result.State)
	require.Len(t, emitted, 1)
	assert.Equal(t, "ready", emitted[0].State)
}

func TestApp_RetryKnowledgeIndex_keepsThePreviousSnapshot_whenTheRetryFails(t *testing.T) {
	// Given an App whose index already completed a successful initial load
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	first := []domainknowledge.Chunk{testIndexChunk("c1")}
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{Chunks: first}, nil).Once()
	store := knowledgemocks.NewMockVectorStore(t)
	store.EXPECT().ReplaceAll(ctx, first).Return(nil).Once()
	loader := applicationknowledge.NewIndexLoader(chunks, store, testEmbeddingModel)
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, loader)
	app.Startup(ctx)
	app.emit = func(context.Context, string, ...interface{}) {}
	app.StartKnowledgeIndex(ctx)

	boom := errors.New("disk full")
	chunks.EXPECT().ListCurrent(ctx, testEmbeddingModel).Return(domainknowledge.ChunkLoadResult{}, boom).Once()

	// When retrying and the reload fails
	result := app.RetryKnowledgeIndex()

	// Then the previous ready state and snapshot are preserved
	assert.Equal(t, "ready", result.State)
	assert.True(t, result.HasSnapshot)
	assert.NotEmpty(t, result.LastError)
}
