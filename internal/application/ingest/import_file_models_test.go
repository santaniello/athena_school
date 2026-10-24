package ingest

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	ingestmocks "github.com/santaniello/athena/internal/application/ingest/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
)

const largeEmbeddingModel = "openai/text-embedding-3-large"

func resolverWithEmbedding(model string) *domainllm.OverridableResolver {
	resolver := domainllm.NewOverridableResolver()
	resolver.SetOverrides(nil, model)
	return resolver
}

func TestImportFile_stampsChunksAndIngestedFileWithTheConfiguredEmbeddingModel(t *testing.T) {
	// Given a service whose resolver picks a non-default embedding model
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: largeEmbeddingModel}, nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.MatchedBy(func(cs []domainknowledge.Chunk) bool {
		return len(cs) == 1 && cs[0].EmbeddingModel == largeEmbeddingModel
	})).Return(nil).Once()
	items.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.EmbeddingModel == largeEmbeddingModel
	})).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))
	service.models = resolverWithEmbedding(largeEmbeddingModel)

	// When importing a file
	summary, err := service.ImportFile(ctx, testSessionID, root, testSourceRoot, "go.md", noopProgress)

	// Then both the chunk and the ingested-file record carry the chosen model
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
}

func TestImportFile_fileIndexedWithAnotherEmbeddingModel_isReEmbeddedEvenWhenUnchanged(t *testing.T) {
	// Given a file recorded with the default model and the same mtime, while
	// the user has since switched to another embedding model
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	llm := llmmocks.NewMockProvider(t)
	tx := ingestmocks.NewMockTransactor(t)
	runWithinTx(tx)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: domainllm.EmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: largeEmbeddingModel}, nil).Once()
	chunks.EXPECT().DeleteBySourcePath(ctx, testSessionID, srcPath("go.md")).Return(nil, nil).Once()
	chunks.EXPECT().SaveAll(ctx, mock.Anything).Return(nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	items.EXPECT().Update(ctx, mock.Anything).Return(nil).Once()
	ingestedFiles.EXPECT().Upsert(ctx, mock.MatchedBy(func(f domainknowledge.IngestedFile) bool {
		return f.EmbeddingModel == largeEmbeddingModel
	})).Return(nil).Once()
	service := newTestService(chunks, ingestedFiles, items, noOpDocuments(t), llm, tx, noOpReconciliationStore(t), passingIndexGuard(t))
	service.models = resolverWithEmbedding(largeEmbeddingModel)

	// When re-importing it
	summary, err := service.ImportFile(ctx, testSessionID, root, testSourceRoot, "go.md", noopProgress)

	// Then it is re-embedded with the new model instead of skipped
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesIngested)
	assert.Equal(t, 0, summary.FilesSkipped)
}

func TestImportFile_fileIndexedWithTheConfiguredModel_isSkippedWhenUnchanged(t *testing.T) {
	// Given a file recorded with the non-default model currently configured
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go."), ModTime: fixedModTime}}
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	items := knowledgemocks.NewMockRepository(t)
	documents := knowledgemocks.NewMockDocumentRepository(t)
	llm := llmmocks.NewMockProvider(t)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SourcePath: srcPath("go.md"), Path: "go.md", MTimeUnixNano: fixedModTime.UnixNano(),
			EmbeddingModel: largeEmbeddingModel, ChunkCount: 1, ItemID: "item-1",
		},
	}, nil).Once()
	items.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{ID: "item-1"}, nil).Once()
	documents.EXPECT().Get(ctx, testSessionID, "item-1").Return("# Go\nBasics of Go.", nil).Once()
	service := newTestService(knowledgemocks.NewMockChunkRepository(t), ingestedFiles, items, documents, llm,
		ingestmocks.NewMockTransactor(t), nil, passingIndexGuard(t))
	service.models = resolverWithEmbedding(largeEmbeddingModel)

	// When importing it again
	summary, err := service.ImportFile(ctx, testSessionID, root, testSourceRoot, "go.md", noopProgress)

	// Then the unchanged file is skipped, with no embedding call
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesSkipped)
}

func TestImportFile_embeddingModelChangingMidImport_failsTheFileInsteadOfMixingModels(t *testing.T) {
	// Given a provider that answers with a different model than the one this
	// import started with (the user changed the setting meanwhile)
	root := fstest.MapFS{"go.md": {Data: []byte("# Go\nBasics of Go.")}}
	ctx := context.Background()
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	llm := llmmocks.NewMockProvider(t)
	ingestedFiles.EXPECT().ListBySession(ctx, testSessionID).Return(map[string]domainknowledge.IngestedFile{}, nil).Once()
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "Go\n\n# Go\nBasics of Go."}).
		Return(domainllm.EmbeddingResponse{Embedding: []float64{0.1}, Model: largeEmbeddingModel}, nil).Once()
	service := newTestService(knowledgemocks.NewMockChunkRepository(t), ingestedFiles, knowledgemocks.NewMockRepository(t),
		knowledgemocks.NewMockDocumentRepository(t), llm, ingestmocks.NewMockTransactor(t), nil, passingIndexGuard(t))

	// When importing under the default model
	summary, err := service.ImportFile(ctx, testSessionID, root, testSourceRoot, "go.md", noopProgress)

	// Then the file is reported failed and nothing is persisted
	require.NoError(t, err)
	assert.Equal(t, 1, summary.FilesFailed)
	require.Len(t, summary.Failures, 1)
	assert.Contains(t, summary.Failures[0].Reason, "embedding model changed")
}

func TestPreviewImport_fileIndexedWithAnotherEmbeddingModel_isNotSkipped(t *testing.T) {
	// Given a file recorded with the default model, with the user now on
	// another one; the formatter is reached because the file is not skipped
	const content = "# Go\nBasics of Go."
	ingestedFiles := knowledgemocks.NewMockIngestedFileRepository(t)
	ingestedFiles.EXPECT().ListBySession(mock.Anything, testSessionID).Return(map[string]domainknowledge.IngestedFile{
		srcPath("go.md"): {
			SessionID: testSessionID, SourcePath: srcPath("go.md"), Path: "go.md",
			MTimeUnixNano: fixedModTime.UnixNano(), EmbeddingModel: domainllm.EmbeddingModel, ItemID: "item-1",
		},
	}, nil).Once()
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Chat(mock.Anything, mock.MatchedBy(func(r domainllm.ChatRequest) bool {
		return r.Task == domainllm.TaskNoteFormatting
	})).Return(domainllm.ChatResponse{Content: content}, nil)
	svc := &Service{
		index: passingPreviewIndexGuard(t), ingestedFiles: ingestedFiles, llm: llm,
		models: resolverWithEmbedding(largeEmbeddingModel),
	}
	root := fstest.MapFS{"go.md": {Data: []byte(content), ModTime: fixedModTime}}

	// When previewing it
	preview, err := svc.PreviewImport(context.Background(), testSessionID, root, testSourceRoot, "go.md", nil)

	// Then it is not reported as unchanged
	require.NoError(t, err)
	assert.False(t, preview.Skipped)
}
