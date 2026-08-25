package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSaveAndApprove_revalidatesAndPersistsDirectlyAsApproved(t *testing.T) {
	// Given one valid candidate with hostile server-owned fields and one invalid candidate —
	// see specs/Athena.md §12 ("Salvar como conhecimento" skips the draft stage)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "client-id" && item.ID != "" &&
			item.Topic == "Go" && item.Concept == "Channels" && item.Definition == "Typed conduits." &&
			item.Source == domainknowledge.SourceAthena && item.Status == domainknowledge.StatusApproved &&
			!item.CreatedAt.IsZero() && item.CreatedAt.Equal(item.UpdatedAt)
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})
	input := []domainknowledge.Item{
		{ID: "client-id", Topic: " Go ", Concept: " Channels ", Definition: " Typed conduits. ", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusDraft},
		{Topic: "Go", Concept: "Invalid", Definition: "   "},
	}

	// When saving and approving the candidates
	savedIndices, err := service.SaveAndApprove(ctx, input)

	// Then only the valid item is persisted and indexed, directly as approved
	require.NoError(t, err)
	assert.Equal(t, []int{0}, savedIndices)
}

func TestSaveAndApprove_stopsAtRepositoryFailureAndReturnsSavedIndices(t *testing.T) {
	// Given three valid candidates and a repository that fails on the second
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "first" })).Return(nil).Once()
	saveErr := errors.New("database locked")
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "second" })).Return(saveErr).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{})
	input := []domainknowledge.Item{
		{Topic: "Go", Concept: "first", Definition: "one"},
		{Topic: "Go", Concept: "second", Definition: "two"},
		{Topic: "Go", Concept: "third", Definition: "three"},
	}

	// When saving and approving the candidates
	savedIndices, err := service.SaveAndApprove(ctx, input)

	// Then processing aborts at the failed item and reports only the saved prefix
	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, []int{0}, savedIndices)
}
