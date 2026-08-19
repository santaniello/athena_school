package knowledge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestDeprecate_transitionsApprovedToDeprecated_andPersists(t *testing.T) {
	// Given an approved item, including one that arrived there as an
	// imported note (approved from creation, never having been a draft)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Status == domainknowledge.StatusDeprecated
	})).Return(nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When deprecating it
	updated, err := service.Deprecate(ctx, "item-1")

	// Then it comes back deprecated
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusDeprecated, updated.Status)
}

func TestDeprecate_returnsInvalidTransition_whenItemIsStillADraft(t *testing.T) {
	// Given a draft item (not yet approved)
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Status: domainknowledge.StatusDraft,
	}, nil).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, nil, nil, nil, nil, nil, tx)

	// When deprecating it
	_, err := service.Deprecate(ctx, "item-1")

	// Then it fails, and Update is never called
	assert.ErrorIs(t, err, domainknowledge.ErrInvalidStatusTransition)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}
