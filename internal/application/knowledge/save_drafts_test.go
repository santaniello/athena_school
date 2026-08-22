package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSaveDrafts_revalidatesAndRegeneratesServerOwnedFields(t *testing.T) {
	// Given one valid candidate with hostile server-owned fields and one invalid candidate
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "client-id" && item.ID != "" &&
			item.Topic == "Go" && item.Concept == "Channels" && item.Definition == "Typed conduits." &&
			item.Source == domainknowledge.SourceAthena && item.Status == domainknowledge.StatusDraft &&
			!item.CreatedAt.IsZero() && item.CreatedAt.Equal(item.UpdatedAt)
	})).Return(nil).Once()
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{})
	input := []domainknowledge.Item{
		{ID: "client-id", Topic: " Go ", Concept: " Channels ", Definition: " Typed conduits. ", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved},
		{Topic: "Go", Concept: "Invalid", Definition: "   "},
	}

	// When saving the candidates
	savedIndices, err := service.SaveDrafts(ctx, input)

	// Then only the valid item is persisted and every server-owned field is replaced
	require.NoError(t, err)
	assert.Equal(t, []int{0}, savedIndices)
}

func TestSaveDrafts_stopsAtRepositoryFailureAndReturnsSavedIndices(t *testing.T) {
	// Given three valid candidates and a repository that fails on the second
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "first" })).Return(nil).Once()
	saveErr := errors.New("database locked")
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "second" })).Return(saveErr).Once()
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{})
	input := []domainknowledge.Item{
		{Topic: "Go", Concept: "first", Definition: "one"},
		{Topic: "Go", Concept: "second", Definition: "two"},
		{Topic: "Go", Concept: "third", Definition: "three"},
	}

	// When saving the candidates
	savedIndices, err := service.SaveDrafts(ctx, input)

	// Then processing aborts at the failed item and reports only the saved prefix
	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, []int{0}, savedIndices)
}

func TestSaveDrafts_returnsExactSavedIndicesWhenInvalidItemsPrecedeFailure(t *testing.T) {
	// Given an invalid item followed by one successful save and one repository failure
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "saved"
	})).Return(nil).Once()
	saveErr := errors.New("database locked")
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Concept == "failed"
	})).Return(saveErr).Once()
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, nil, domainknowledge.RetrievalThresholds{})
	input := []domainknowledge.Item{
		{Topic: "Go", Concept: "invalid", Definition: ""},
		{Topic: "Go", Concept: "saved", Definition: "persisted"},
		{Topic: "Go", Concept: "failed", Definition: "not persisted"},
	}

	// When saving the candidates
	savedIndices, err := service.SaveDrafts(ctx, input)

	// Then the result identifies the actual persisted input rather than a prefix count
	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, []int{1}, savedIndices)
}
