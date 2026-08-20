package knowledge

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
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
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

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
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When updating with a blank concept
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "Go", Concept: "", Definition: "Old def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrConceptRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateItem_trimsTopicWhitespace_beforePersisting(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.Topic == "Distributed systems"
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When updating with a topic padded with whitespace
	updated, err := service.UpdateItem(ctx, "item-1", ItemFields{
		Topic: "  Distributed systems  ", Concept: "New", Definition: "New def.",
	})

	// Then the persisted and returned topic is trimmed
	require.NoError(t, err)
	assert.Equal(t, "Distributed systems", updated.Topic)
}

func TestUpdateItem_returnsTopicRequired_whenTopicIsBlank_andNeverCallsUpdate(t *testing.T) {
	// Given an existing item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Old", Definition: "Old def.",
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When updating with a whitespace-only topic
	_, err := service.UpdateItem(ctx, "item-1", ItemFields{Topic: "   ", Concept: "New", Definition: "New def."})

	// Then it fails validation and never reaches the repository's Update
	assert.ErrorIs(t, err, domainknowledge.ErrTopicRequired)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestUpdateItem_propagatesNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given no matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When updating it
	_, err := service.UpdateItem(ctx, "missing", ItemFields{Topic: "Go", Concept: "X", Definition: "Y"})

	// Then the not-found error propagates
	assert.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}
