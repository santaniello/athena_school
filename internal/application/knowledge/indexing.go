package knowledge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// renderItemContent builds the text embedded for item: Concept, Definition,
// Properties and TradeOffs only. Topic is deliberately excluded — it is a
// metadata filter, not indexed content, so a Topic-only edit never needs a
// new embedding (see ChunkRepository.UpdateMetadataByItemID). RelatedConcepts
// is excluded for the same reason.
func renderItemContent(item domainknowledge.Item) string {
	var b strings.Builder
	b.WriteString(item.Concept)
	b.WriteString("\n\n")
	b.WriteString(item.Definition)
	if len(item.Properties) > 0 {
		b.WriteString("\n\nProperties:")
		for _, property := range item.Properties {
			b.WriteString("\n- " + property)
		}
	}
	if len(item.TradeOffs) > 0 {
		b.WriteString("\n\nTrade-offs:")
		for _, tradeOff := range item.TradeOffs {
			b.WriteString("\n- " + tradeOff)
		}
	}
	return b.String()
}

// indexingFailure wraps cause as an error satisfying errors.Is(err,
// ErrIndexingFailed), identifying which item and step failed.
func indexingFailure(itemID, step string, cause error) error {
	return fmt.Errorf("%s knowledge item %s: %w: %w", step, itemID, ErrIndexingFailed, cause)
}

// indexKnowledgeItem makes item searchable as a single chunk: any chunk(s)
// it already owns are deleted and the freshly embedded one inserted in one
// SQLite transaction, then the VectorStore is reconciled to match (the old
// IDs evicted, the new chunk added). It never mutates anything beyond that
// delete-then-insert — the item itself is assumed already persisted by the
// caller.
//
// Every failure — embedding, SQLite, or VectorStore — returns an error
// wrapping ErrIndexingFailed rather than touching what's already durable
// for item; see the Failure policy section of
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md.
func (s *Service) indexKnowledgeItem(ctx context.Context, item domainknowledge.Item) error {
	response, err := s.llm.Embeddings(ctx, domainllm.EmbeddingRequest{Input: renderItemContent(item)})
	if err != nil {
		return indexingFailure(item.ID, "embedding", err)
	}

	chunk := domainknowledge.Chunk{
		ID:             uuid.NewString(),
		Source:         domainknowledge.SourceAthena,
		Topic:          item.Topic,
		Status:         item.Status,
		ItemID:         item.ID,
		Content:        renderItemContent(item),
		Embedding:      toFloat32Embedding(response.Embedding),
		EmbeddingModel: domainllm.EmbeddingModel,
		ItemUpdatedAt:  item.UpdatedAt,
		CreatedAt:      time.Now().UTC(),
	}

	var removedChunkIDs []string
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		removedChunkIDs, err = s.chunks.DeleteByItemID(ctx, item.ID)
		if err != nil {
			return err
		}
		return s.chunks.SaveAll(ctx, []domainknowledge.Chunk{chunk})
	})
	if err != nil {
		return indexingFailure(item.ID, "persisting chunk for", err)
	}

	reconcileCtx, cancel := reconcileContext()
	defer cancel()
	if err := s.store.Remove(reconcileCtx, removedChunkIDs); err != nil {
		return indexingFailure(item.ID, "evicting stale chunk for", err)
	}
	if err := s.store.Add(reconcileCtx, []domainknowledge.Chunk{chunk}); err != nil {
		return indexingFailure(item.ID, "indexing", err)
	}
	return nil
}

// toFloat32Embedding converts an EmbeddingResponse's float64 vector to the
// float32 precision knowledge.Chunk.Embedding stores (see
// internal/infrastructure/sqlite/embedding.go).
func toFloat32Embedding(vec []float64) []float32 {
	out := make([]float32, len(vec))
	for i, v := range vec {
		out[i] = float32(v)
	}
	return out
}
