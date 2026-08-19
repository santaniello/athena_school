package knowledge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestApprove_transitionsDraftToApproved_andPersists(t *testing.T) {
	// Given a draft item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits.",
		Source: domainknowledge.SourceAthena, Status: domainknowledge.StatusDraft,
	}, nil).Once()
	repository.EXPECT().Update(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID == "item-1" && item.Status == domainknowledge.StatusApproved && !item.UpdatedAt.IsZero()
	})).Return(nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When approving it
	updated, err := service.Approve(ctx, "item-1")

	// Then it comes back approved
	require.NoError(t, err)
	assert.Equal(t, domainknowledge.StatusApproved, updated.Status)
}

func TestApprove_returnsInvalidTransition_whenItemIsAlreadyApproved(t *testing.T) {
	// Given an already-approved item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "item-1").Return(domainknowledge.Item{
		ID: "item-1", Status: domainknowledge.StatusApproved,
	}, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When approving it again
	_, err := service.Approve(ctx, "item-1")

	// Then it fails, and Update is never called
	assert.ErrorIs(t, err, domainknowledge.ErrInvalidStatusTransition)
	repository.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestApprove_propagatesNotFound_whenItemDoesNotExist(t *testing.T) {
	// Given no matching item
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().GetByID(ctx, "missing").Return(domainknowledge.Item{}, domainknowledge.ErrItemNotFound).Once()
	service := NewService(repository, nil, nil, nil, nil, nil)

	// When approving it
	_, err := service.Approve(ctx, "missing")

	// Then the not-found error propagates
	assert.ErrorIs(t, err, domainknowledge.ErrItemNotFound)
}
