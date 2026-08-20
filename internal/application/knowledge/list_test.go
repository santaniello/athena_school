package knowledge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestListItems_forwardsTopicAndStatusFilter(t *testing.T) {
	// Given a repository that returns items for a topic/status filter
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	expected := []domainknowledge.Item{{ID: "item-1", Topic: "Go", Status: domainknowledge.StatusApproved}}
	repository.EXPECT().List(ctx, domainknowledge.Filter{Topic: "Go", Status: domainknowledge.StatusApproved}).
		Return(expected, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil)

	// When listing items by topic and status
	items, err := service.ListItems(ctx, "Go", domainknowledge.StatusApproved)

	// Then the repository's result is returned as-is
	require.NoError(t, err)
	assert.Equal(t, expected, items)
}

func TestListTopics_returnsRepositoryTopics(t *testing.T) {
	// Given a repository with three distinct topics
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().ListTopics(ctx).Return([]string{"Go", "Kubernetes"}, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil)

	// When listing topics
	topics, err := service.ListTopics(ctx)

	// Then they are returned as-is
	require.NoError(t, err)
	assert.Equal(t, []string{"Go", "Kubernetes"}, topics)
}
