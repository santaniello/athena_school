package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"unicode/utf8"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// maxContextChars is the hard cap, in Unicode code points, on the rendered
// JSON data block a RetrievalResult carries. File paths, headings, concepts,
// content, JSON syntax, and separators all count. Owned by this application
// layer rather than the domain: see
// specs/phases/phase-02-knowledge-engine/05-rag-integration.md.
const maxContextChars = 8000

// contextEntry is one chunk's rendered representation inside the JSON data
// block sent to the LLM. The embedding and score are deliberately excluded.
type contextEntry struct {
	SourceType string `json:"sourceType"`
	FilePath   string `json:"filePath"`
	Heading    string `json:"heading"`
	Concept    string `json:"concept"`
	Content    string `json:"content"`
}

// Retrieve implements domainknowledge.Retriever: it embeds and searches
// each of queries independently (session-attributed, scoped to that
// session's own approved local knowledge), combines the results by chunk —
// keeping each surviving chunk's best score across every query — then
// filters and caps the combined result, and resolves each surviving
// chunk's owning item's concept. study.Service calls this for every
// SourceMode.
//
// A survivor whose owning item no longer exists — e.g. a chunk orphaned in
// the VectorStore by a failed post-commit Remove (see
// specs/phases/phase-02-knowledge-engine/08-02-deleteitem-orphan-chunk-risk.md)
// — is dropped rather than failing the whole call: a partial answer beats
// none. Any other resolution error still aborts Retrieve, so a genuine
// infrastructure failure is never mistaken for "nothing relevant found".
func (s *Service) Retrieve(ctx context.Context, sessionID string, queries []string) (domainknowledge.RetrievalResult, error) {
	status := s.index.Status()
	if !status.HasSnapshot {
		return domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable
	}
	if s.store.Len() == 0 {
		return domainknowledge.RetrievalResult{}, nil
	}

	resultSets := make([][]domainknowledge.ScoredChunk, len(queries))
	for i, query := range queries {
		response, err := s.llm.Embeddings(ctx, domainllm.EmbeddingRequest{SessionID: sessionID, Input: query})
		if err != nil {
			return domainknowledge.RetrievalResult{}, fmt.Errorf("knowledge: embedding retrieval query: %w", err)
		}

		scored, err := s.store.Search(
			ctx, toFloat32(response.Embedding), domainknowledge.DefaultTopK,
			domainknowledge.SearchFilters{SessionID: sessionID},
		)
		if err != nil {
			return domainknowledge.RetrievalResult{}, fmt.Errorf("knowledge: searching local knowledge: %w", err)
		}
		resultSets[i] = scored
	}
	scored := combineByBestScore(resultSets)

	survivors := make([]domainknowledge.ScoredChunk, 0, len(scored))
	for _, sc := range scored {
		if float64(sc.Score) >= s.thresholds.MinSimilarity {
			survivors = append(survivors, sc)
		}
	}
	if len(survivors) == 0 {
		return domainknowledge.RetrievalResult{}, nil
	}

	concepts := make(map[string]string, len(survivors))
	orphaned := make(map[string]struct{})
	for _, sc := range survivors {
		if _, resolved := concepts[sc.Chunk.ItemID]; resolved {
			continue
		}
		if _, dropped := orphaned[sc.Chunk.ItemID]; dropped {
			continue
		}
		item, err := s.items.GetByID(ctx, sc.Chunk.ItemID)
		if err != nil {
			if errors.Is(err, domainknowledge.ErrItemNotFound) {
				orphaned[sc.Chunk.ItemID] = struct{}{}
				continue
			}
			return domainknowledge.RetrievalResult{}, fmt.Errorf(
				"knowledge: resolving owning item %s: %w", sc.Chunk.ItemID, err,
			)
		}
		concepts[sc.Chunk.ItemID] = item.Concept
	}
	if len(orphaned) > 0 {
		filtered := make([]domainknowledge.ScoredChunk, 0, len(survivors))
		for _, sc := range survivors {
			if _, dropped := orphaned[sc.Chunk.ItemID]; dropped {
				log.Printf("knowledge: retrieval: dropping orphaned chunk %s (item %s no longer exists)", sc.Chunk.ID, sc.Chunk.ItemID)
				continue
			}
			filtered = append(filtered, sc)
		}
		survivors = filtered
		if len(survivors) == 0 {
			return domainknowledge.RetrievalResult{}, nil
		}
	}

	// capped only ever shrinks by removing the lowest-scoring survivor
	// (the slice's last element); it starts non-empty (survivors is
	// non-empty here), so every renderContext call below sees a non-empty
	// slice — shrinking to empty returns the no-match result immediately,
	// instead of looping back to render an empty (and always in-budget)
	// JSON block that would never be used anyway.
	capped := survivors
	var renderedContext string
	for {
		rendered, err := renderContext(capped, concepts)
		if err != nil {
			return domainknowledge.RetrievalResult{}, fmt.Errorf("knowledge: rendering retrieval context: %w", err)
		}
		if utf8.RuneCountInString(rendered) <= maxContextChars {
			renderedContext = rendered
			break
		}
		capped = capped[:len(capped)-1]
		if len(capped) == 0 {
			return domainknowledge.RetrievalResult{}, nil
		}
	}

	sufficient := false
	sources := make([]domainknowledge.Source, len(capped))
	for i, sc := range capped {
		if float64(sc.Score) >= s.thresholds.Sufficiency {
			sufficient = true
		}
		sources[i] = domainknowledge.Source{
			ChunkID:    sc.Chunk.ID,
			ItemID:     sc.Chunk.ItemID,
			SourceType: sc.Chunk.Source,
			FilePath:   sc.Chunk.FilePath,
			Heading:    sc.Chunk.Heading,
			Concept:    concepts[sc.Chunk.ItemID],
			Score:      sc.Score,
			Excerpt:    sc.Chunk.Content,
		}
	}

	return domainknowledge.RetrievalResult{
		Chunks:     capped,
		Sufficient: sufficient,
		Context:    renderedContext,
		Sources:    sources,
	}, nil
}

// combineByBestScore merges several per-query search results into one,
// keeping, for each surviving chunk, the best score it received across any
// of them — a chunk only one query actually found still survives with that
// query's own score. Re-sorts score-descending/chunk-ID-ascending
// afterward, matching VectorStore.Search's own order: everything
// downstream (MinSimilarity filtering, capping) relies on that order,
// capping in particular discarding the lowest-scoring tail first.
func combineByBestScore(resultSets [][]domainknowledge.ScoredChunk) []domainknowledge.ScoredChunk {
	best := make(map[string]domainknowledge.ScoredChunk)
	for _, set := range resultSets {
		for _, sc := range set {
			if existing, ok := best[sc.Chunk.ID]; !ok || sc.Score > existing.Score {
				best[sc.Chunk.ID] = sc
			}
		}
	}
	combined := make([]domainknowledge.ScoredChunk, 0, len(best))
	for _, sc := range best {
		combined = append(combined, sc)
	}
	sort.Slice(combined, func(i, j int) bool {
		if combined[i].Score != combined[j].Score {
			return combined[i].Score > combined[j].Score
		}
		// combined is deduplicated by Chunk.ID (it's a map's values), so
		// this comparison never sees equal IDs on both sides — i < j and
		// i <= j always agree here.
		return combined[i].Chunk.ID < combined[j].Chunk.ID
	})
	return combined
}

// renderContext serializes chunks into the deterministic JSON data block,
// in the given order, using each chunk's owning item's already-resolved
// concept.
func renderContext(chunks []domainknowledge.ScoredChunk, concepts map[string]string) (string, error) {
	entries := make([]contextEntry, len(chunks))
	for i, sc := range chunks {
		entries[i] = contextEntry{
			SourceType: sc.Chunk.Source,
			FilePath:   sc.Chunk.FilePath,
			Heading:    sc.Chunk.Heading,
			Concept:    concepts[sc.Chunk.ItemID],
			Content:    sc.Chunk.Content,
		}
	}
	rendered, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(rendered), nil
}

// toFloat32 converts an EmbeddingResponse's float64 vector to the float32
// precision VectorStore.Search expects. Kept as a small local copy rather
// than shared with internal/application/ingest's identical helper.
func toFloat32(vec []float64) []float32 {
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(v)
	}
	return out
}
