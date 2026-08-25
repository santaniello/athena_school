// Package vectorstore provides the in-process, pure-Go implementation of
// knowledge.VectorStore: a flat slice scored by brute-force cosine
// similarity, kept coherent with SQLite by explicit Add/ReplaceAll/Remove
// calls from the application layer rather than owning its own persistence.
// See specs/phases/phase-02-knowledge-engine/04-vector-search.md and
// specs/decisions/ADR-004-local-vector-store.md.
package vectorstore

import (
	"context"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/santaniello/athena/internal/domain/knowledge"
)

// Store is the in-memory knowledge.VectorStore implementation. The zero
// value is not usable — construct one with New.
type Store struct {
	mu     sync.RWMutex
	chunks []knowledge.Chunk
	// ready becomes true on the first successful ReplaceAll, including an
	// empty one. Add and Remove never set it — a store that has only ever
	// seen Add/Remove calls is not the same as one that has completed an
	// initial load.
	ready bool
}

// New creates an empty, not-yet-ready Store.
func New() *Store {
	return &Store{}
}

// Add validates and normalizes the batch, then upserts it by Chunk.ID into
// the active slice: an existing ID is replaced in place, a new one is
// appended. Before that, any existing chunk that shares an incoming
// chunk's ItemID but carries a different Chunk.ID is evicted — the stale
// sibling a failed Remove left behind (see
// specs/phases/phase-02-knowledge-engine/08-01-vectorstore-orphan-chunk-recovery.md).
// Add never marks the store ready.
func (s *Store) Add(ctx context.Context, chunks []knowledge.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	prepared, err := prepareBatch(ctx, chunks)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	incomingItemIDs := make(map[string]struct{}, len(prepared))
	incomingIDs := make(map[string]struct{}, len(prepared))
	for _, c := range prepared {
		incomingItemIDs[c.ItemID] = struct{}{}
		incomingIDs[c.ID] = struct{}{}
	}
	filtered := make([]knowledge.Chunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		if _, sameItem := incomingItemIDs[c.ItemID]; sameItem {
			if _, sameID := incomingIDs[c.ID]; !sameID {
				continue
			}
		}
		filtered = append(filtered, c)
	}
	s.chunks = filtered

	index := make(map[string]int, len(s.chunks))
	for i, c := range s.chunks {
		index[c.ID] = i
	}
	for _, c := range prepared {
		if i, exists := index[c.ID]; exists {
			s.chunks[i] = c
			continue
		}
		// prepareBatch already rejected duplicate IDs within prepared, so
		// no later chunk in this same loop can look up c.ID again — no
		// need to record its new position in index.
		s.chunks = append(s.chunks, c)
	}
	return nil
}

// ReplaceAll validates and normalizes the batch, then atomically replaces
// the entire active snapshot with it and marks the store ready — even when
// the batch is empty, since that is a real "genuinely empty knowledge
// base" publication, not a no-op.
func (s *Store) ReplaceAll(ctx context.Context, chunks []knowledge.Chunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	prepared, err := prepareBatch(ctx, chunks)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.chunks = prepared
	s.ready = true
	return nil
}

// Remove evicts every given ID atomically. It validates every ID before
// mutating anything; unknown valid IDs are silently ignored.
func (s *Store) Remove(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		if strings.TrimSpace(id) == "" {
			return knowledge.ErrInvalidChunkID
		}
	}

	toRemove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		toRemove[id] = struct{}{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]knowledge.Chunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		if _, remove := toRemove[c.ID]; !remove {
			filtered = append(filtered, c)
		}
	}
	s.chunks = filtered
	return nil
}

// Len reports how many chunks are currently indexed.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.chunks)
}

// Search returns at most topK chunks matching filters, ordered by cosine
// similarity descending and then Chunk.ID ascending. See the order of
// checks documented on knowledge.VectorStore.Search.
func (s *Store) Search(
	ctx context.Context, query []float32, topK int, filters knowledge.SearchFilters,
) ([]knowledge.ScoredChunk, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if topK <= 0 {
		return nil, knowledge.ErrInvalidTopK
	}
	if err := knowledge.ValidateVector(query); err != nil {
		return nil, err
	}
	normalizedQuery := normalizeVector(query)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.ready {
		return nil, knowledge.ErrVectorStoreUnavailable
	}

	scored := make([]knowledge.ScoredChunk, 0, len(s.chunks))
	for _, c := range s.chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if filters.Topic != "" && filters.Topic != c.Topic {
			continue
		}
		if filters.Source != "" && filters.Source != c.Source {
			continue
		}
		if filters.Status != "" && filters.Status != c.Status {
			continue
		}
		if len(c.Embedding) != len(normalizedQuery) {
			continue
		}
		scored = append(scored, knowledge.ScoredChunk{
			Chunk: deepCopyChunk(c),
			Score: dotProduct(normalizedQuery, c.Embedding),
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Chunk.ID < scored[j].Chunk.ID
	})
	if len(scored) > topK {
		scored = scored[:topK]
	}
	return scored, nil
}

// prepareBatch validates the complete batch (rejecting it whole on the
// first invalid or duplicate-ID chunk) and returns deep-copied, unit-
// normalized chunks ready to publish — built away from the active slice,
// so a failed validation never touches store state. ctx is checked before
// each chunk, so a long batch (e.g. the 10k functional case) can still be
// canceled mid-validation without side effects.
func prepareBatch(ctx context.Context, chunks []knowledge.Chunk) ([]knowledge.Chunk, error) {
	seen := make(map[string]struct{}, len(chunks))
	prepared := make([]knowledge.Chunk, len(chunks))
	for i, c := range chunks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := knowledge.ValidateChunk(c); err != nil {
			return nil, err
		}
		if _, dup := seen[c.ID]; dup {
			return nil, knowledge.ErrDuplicateChunkID
		}
		seen[c.ID] = struct{}{}
		prepared[i] = normalizeChunk(c)
	}
	return prepared, nil
}

// normalizeChunk returns a copy of c with a deep-copied, unit-normalized
// Embedding — the provider's raw vector (chunk.Embedding as validated) is
// never mutated, and the caller's backing array is never aliased.
func normalizeChunk(c knowledge.Chunk) knowledge.Chunk {
	c.Embedding = normalizeVector(c.Embedding)
	return c
}

// normalizeVector returns a new slice scaled to unit Euclidean norm.
// Callers must validate vec (ValidateChunk/ValidateVector) first — this
// never checks for a zero norm.
func normalizeVector(vec []float32) []float32 {
	var sumSquares float64
	for _, v := range vec {
		sumSquares += float64(v) * float64(v)
	}
	norm := math.Sqrt(sumSquares)
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(float64(v) / norm)
	}
	return out
}

// deepCopyChunk returns a copy of c whose Embedding is backed by a new
// array, so a caller mutating a Search result cannot reach into the store.
func deepCopyChunk(c knowledge.Chunk) knowledge.Chunk {
	c.Embedding = append([]float32(nil), c.Embedding...)
	return c
}

// dotProduct assumes a and b are the same length and both unit vectors —
// callers guarantee this (dimension check, normalizeVector) — so the
// result equals cosine similarity.
func dotProduct(a, b []float32) float32 {
	var sum float32
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}
