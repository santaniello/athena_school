package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestUpdateItem_overwritesEditableFields_andRestampsUpdatedAt(t *testing.T) {
	// Given an existing approved item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	originalCreatedAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusApproved,
		CreatedAt: originalCreatedAt, UpdatedAt: originalCreatedAt,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Concept == "New" && item.Definition == "New def." &&
			item.Status == domainknowledge.StatusApproved && item.Source == domainknowledge.SourceAthena &&
			item.CreatedAt.Equal(originalCreatedAt) && item.UpdatedAt.After(originalCreatedAt)
	})).Return(nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When updating its editable fields
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "Go", Concept: "New", Definition: "New def.",
		Properties: []string{"p1"}, TradeOffs: []string{"t1"}, RelatedConcepts: []string{"r1"},
	})

	// Then the change persists and Status/Source/CreatedAt are untouched
	require.NoError(t, err)
	assert.Equal(t, "New", updated.Concept)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
	assert.Equal(t, domainknowledge.SourceAthena, updated.Source)
	assert.True(t, updated.CreatedAt.Equal(originalCreatedAt))
}

func TestUpdateItem_returnsValidationError_whenConceptIsCleared_andNeverCallsUpdate(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When updating with a blank concept
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "", Definition: "Old def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrConceptRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateItem_propagatesNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given no matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When updating it
	_, err := service.UpdateItem(ctx, "missing", ItemFields{Topic: "Go", Concept: "X", Definition: "Y"})

	// Then the not-found error propagates
	assert.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}
