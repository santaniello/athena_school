package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
)

func duplicateCandidate(topic, concept, definition string) domainknowledge.Item {
	return domainknowledge.Item{SessionID: "session-1", Topic: topic, Concept: concept, Definition: definition}
}

func TestFindExactDuplicates_returnsEveryNormalizedMatch_orderedByItemID(t *testing.T) {
	// Given items already saved with the same normalized concept in the
	// same session and topic, across every lifecycle status
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().
		FindByNormalizedConcept(ctx, "session-1", "System Design", "cache aside pattern").
		Return([]domainknowledge.Item{
			{ID: "item-draft", Concept: "cache aside pattern", Status: domainknowledge.StatusDraft},
			{ID: "item-approved", Concept: "Cache-Aside Pattern", Status: domainknowledge.StatusApproved},
		}, nil)
	service := NewService(items, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", " Cache-Aside  Pattern ", "A caching strategy.")

	// When finding exact duplicates
	matches, err := service.findExactDuplicates(ctx, candidate)

	// Then both matches are returned as exact, score 1, ordered by item ID ascending
	require.NoError(t, err)
	require.Equal(t, []domainknowledge.DuplicateMatch{
		{ItemID: "item-approved", Concept: "Cache-Aside Pattern", Status: domainknowledge.StatusApproved, MatchType: domainknowledge.MatchExact, Score: 1},
		{ItemID: "item-draft", Concept: "cache aside pattern", Status: domainknowledge.StatusDraft, MatchType: domainknowledge.MatchExact, Score: 1},
	}, matches)
}

func TestFindExactDuplicates_returnsError_whenTheLookupFails(t *testing.T) {
	// Given a repository failure on the exact-match lookup
	ctx := context.Background()
	items := knowledgemocks.NewMockRepository(t)
	items.EXPECT().FindByNormalizedConcept(ctx, "session-1", "System Design", "circuit breaker").
		Return(nil, errors.New("sqlite: database is locked"))
	service := NewService(items, nil, nil, nil, nil, nil, nil, nil, nil, domainknowledge.RetrievalThresholds{}, nil,
		nil, nil, domainknowledge.DefaultDuplicateTopK, domainknowledge.DefaultDuplicateSimilarity)
	candidate := duplicateCandidate("System Design", "Circuit Breaker", "A resiliency pattern.")

	// When finding exact duplicates
	matches, err := service.findExactDuplicates(ctx, candidate)

	// Then it fails with the repository error
	require.ErrorContains(t, err, "database is locked")
	require.Empty(t, matches)
}
