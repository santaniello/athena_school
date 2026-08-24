package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func TestCountDrafts_returnsRepositoryCountForDraftStatus(t *testing.T) {
	// Given a repository with three draft items
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().CountByStatus(ctx, domainknowledge.StatusDraft).Return(3, nil).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{})

	// When counting drafts
	count, err := service.CountDrafts(ctx)

	// Then the repository's count is returned as-is
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountDrafts_propagatesRepositoryError(t *testing.T) {
	// Given a repository that fails to count
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().CountByStatus(ctx, domainknowledge.StatusDraft).Return(0, errors.New("db unavailable")).Once()
	service := NewService(repository, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{})

	// When counting drafts
	_, err := service.CountDrafts(ctx)

	// Then the error is propagated
	require.Error(t, err)
}
