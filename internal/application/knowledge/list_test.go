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
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

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
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When listing topics
	topics, err := service.ListTopics(ctx)

	// Then they are returned as-is
	require.NoError(t, err)
	assert.Equal(t, []string{"Go", "Kubernetes"}, topics)
}

func TestListItemEvidence_returnsRepositorySnapshotsForTheItem(t *testing.T) {
	// Given an item with two persisted Evidence snapshots
	ctx := context.Background()
	expected := []domainknowledge.Evidence{
		{ID: "evidence-1", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-1", SourceLabel: "Go", Excerpt: "first"},
		{ID: "evidence-2", OriginType: domainknowledge.OriginSessionMessage, OriginID: "message-2", SourceLabel: "Go", Excerpt: "second"},
	}
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().ListByItem(ctx, "item-1").Return(expected, nil).Once()
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, evidenceRepo, nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)

	// When listing that item's evidence
	evidence, err := service.ListItemEvidence(ctx, "item-1")

	// Then the repository's result is returned as-is, in its given order
	require.NoError(t, err)
	assert.Equal(t, expected, evidence)
}
