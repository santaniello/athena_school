package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sort"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// FindDuplicates detects whether candidate's concept is already represented
// in its topic. Exact normalized matches (draft, approved, and deprecated
// alike) are checked first and never spend an embedding call; only when
// none exist does it fall back to a semantic search over SourceAthena
// chunks in the same topic, across every status. See
// specs/phases/phase-02-knowledge-engine/10-duplicate-detection.md.
//
// A failure in the exact stage is a genuine error. A failure in the
// semantic stage (the embedding call or the VectorStore search) instead
// returns a nil result wrapping ErrSemanticDuplicateCheckUnavailable — the
// caller must still treat the candidate as usable, only the semantic check
// as incomplete.
func (s *Service) FindDuplicates(
	ctx context.Context, candidate domainknowledge.Item, topK int, minScore float64,
) ([]domainknowledge.DuplicateMatch, error) {
	exactMatches, err := s.findExactDuplicates(ctx, candidate)
	if err != nil {
		return nil, err
	}
	if len(exactMatches) > 0 {
		return exactMatches, nil
	}
	return s.findSemanticDuplicates(ctx, candidate, topK, minScore)
}

// findExactDuplicates is FindDuplicates' first stage, factored out so
// ExtractFromSession's batch loop can run it independently of the semantic
// stage (see attachDuplicateMatches). A failure here is a genuine error —
// unlike the semantic stage, it is not wrapped as a soft, presentable
// warning.
func (s *Service) findExactDuplicates(ctx context.Context, candidate domainknowledge.Item) ([]domainknowledge.DuplicateMatch, error) {
	normalizedConcept := domainknowledge.NormalizeConcept(candidate.Concept)
	exactItems, err := s.items.FindByNormalizedConcept(ctx, candidate.Topic, normalizedConcept)
	if err != nil {
		return nil, fmt.Errorf("knowledge: finding exact duplicate matches: %w", err)
	}
	return exactDuplicateMatches(exactItems), nil
}

// findSemanticDuplicates is FindDuplicates' second stage, factored out so
// ExtractFromSession's batch loop can skip it independently of the exact
// stage once an earlier candidate's own embedding call has already failed
// in the same batch (see attachDuplicateMatches).
func (s *Service) findSemanticDuplicates(
	ctx context.Context, candidate domainknowledge.Item, topK int, minScore float64,
) ([]domainknowledge.DuplicateMatch, error) {
	if s.store.Len() == 0 {
		return nil, nil
	}

	response, err := s.llm.Embeddings(ctx, domainllm.EmbeddingRequest{Input: renderCandidateForDuplicateCheck(candidate)})
	if err != nil {
		return nil, fmt.Errorf("knowledge: embedding duplicate candidate: %w: %w", domainknowledge.ErrSemanticDuplicateCheckUnavailable, err)
	}

	scored, err := s.store.Search(
		ctx, toFloat32(response.Embedding), topK,
		domainknowledge.SearchFilters{Topic: candidate.Topic, Source: domainknowledge.SourceAthena},
	)
	if err != nil {
		return nil, fmt.Errorf("knowledge: searching for semantic duplicates: %w: %w", domainknowledge.ErrSemanticDuplicateCheckUnavailable, err)
	}

	return s.semanticDuplicateMatches(ctx, scored, minScore)
}

// renderCandidateForDuplicateCheck is the text embedded for semantic
// duplicate comparison: concept and definition only. Unlike
// renderItemContent (used for retrieval indexing), Properties/TradeOffs are
// deliberately excluded — 10-duplicate-detection.md scopes the comparison
// to the concept and its definition.
func renderCandidateForDuplicateCheck(candidate domainknowledge.Item) string {
	return candidate.Concept + "\n\n" + candidate.Definition
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

// semanticDuplicateMatches filters scored by minScore, keeps only the
// highest-scoring chunk per owning item (Search already orders scored by
// score descending, so the first chunk seen per ItemID is that item's
// best), resolves each survivor's current Concept/Status, and orders the
// result by score descending then item ID ascending. A chunk whose owning
// item no longer exists is dropped rather than failing the whole call,
// mirroring Retrieve's orphaned-chunk handling.
func (s *Service) semanticDuplicateMatches(ctx context.Context, scored []domainknowledge.ScoredChunk, minScore float64) ([]domainknowledge.DuplicateMatch, error) {
	seen := make(map[string]struct{}, len(scored))
	matches := make([]domainknowledge.DuplicateMatch, 0, len(scored))
	for _, sc := range scored {
		if float64(sc.Score) < minScore {
			continue
		}
		if _, alreadySeen := seen[sc.Chunk.ItemID]; alreadySeen {
			continue
		}
		seen[sc.Chunk.ItemID] = struct{}{}

		item, err := s.items.GetByID(ctx, sc.Chunk.ItemID)
		if err != nil {
			if errors.Is(err, domainknowledge.ErrItemNotFound) {
				continue
			}
			return nil, fmt.Errorf(
				"knowledge: resolving semantic duplicate match owner %s: %w: %w",
				sc.Chunk.ItemID, domainknowledge.ErrSemanticDuplicateCheckUnavailable, err,
			)
		}
		matches = append(matches, domainknowledge.DuplicateMatch{
			ItemID: item.ID, Concept: item.Concept, Status: item.Status,
			MatchType: domainknowledge.MatchSemantic, Score: float64(sc.Score),
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		return matches[i].ItemID < matches[j].ItemID
	})
	return matches, nil
}
