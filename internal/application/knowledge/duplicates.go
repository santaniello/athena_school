package knowledge

import (
	"context"
	"fmt"
	"sort"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// findExactDuplicates returns the existing items in candidate's session and
// topic whose normalized concept equals the candidate's.
func (s *Service) findExactDuplicates(ctx context.Context, candidate domainknowledge.Item) ([]domainknowledge.DuplicateMatch, error) {
	normalizedConcept := domainknowledge.NormalizeConcept(candidate.Concept)
	exactItems, err := s.items.FindByNormalizedConcept(ctx, candidate.SessionID, candidate.Topic, normalizedConcept)
	if err != nil {
		return nil, fmt.Errorf("knowledge: finding exact duplicate matches: %w", err)
	}
	return exactDuplicateMatches(exactItems), nil
}

// exactDuplicateMatches converts every exact match to score 1, ordered by
// item ID ascending — normalization means every entry ties on score.
func exactDuplicateMatches(items []domainknowledge.Item) []domainknowledge.DuplicateMatch {
	matches := make([]domainknowledge.DuplicateMatch, len(items))
	for i, item := range items {
		matches[i] = domainknowledge.DuplicateMatch{
			ItemID: item.ID, Concept: item.Concept, Status: item.Status,
			MatchType: domainknowledge.MatchExact, Score: 1,
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ItemID < matches[j].ItemID })
	return matches
}
