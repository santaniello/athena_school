package knowledge

import (
	"context"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// ReindexProgress reports ReindexKnowledgeItems' progress after each item
// is attempted, indexed or not.
type ReindexProgress struct {
	ItemsProcessed int
	ItemsTotal     int
	CurrentTopic   string
}

// ReindexFailure records why one item could not be indexed during a
// reindex run.
type ReindexFailure struct {
	ItemID string
	Topic  string
	Reason string
}

// ReindexSummary is ReindexKnowledgeItems' final report.
type ReindexSummary struct {
	ItemsProcessed int
	ItemsIndexed   int
	ItemsFailed    int
	Failures       []ReindexFailure
}

// CountUnindexedItems returns how many Knowledge Items currently lack a
// current chunk under the active embedding model — the count the Knowledge
// Explorer's backfill alert shows.
func (s *Service) CountUnindexedItems(ctx context.Context) (int, error) {
	return s.items.CountUnindexed(ctx, domainllm.EmbeddingModel)
}

// ReindexKnowledgeItems processes every currently-unindexed Knowledge Item
// — the backfill recovery path for every indexing failure this package
// documents, and the "Index now" action a user explicitly consents to.
// Unlike SaveDrafts, it never stops early: every item in the backlog is
// attempted regardless of earlier failures in the same run, because this
// is an explicit request to clear a backlog rather than an incidental save
// (see Design decisions in
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md).
//
// onProgress is called once per item, indexed or not. If it returns a
// non-nil error, the run stops immediately and the Summary accumulated so
// far is returned alongside that error.
func (s *Service) ReindexKnowledgeItems(ctx context.Context, onProgress func(ReindexProgress) error) (ReindexSummary, error) {
	if err := s.index.BeginMutation(); err != nil {
		return ReindexSummary{}, err
	}
	defer s.index.EndMutation()

	items, err := s.items.ListUnindexed(ctx, domainllm.EmbeddingModel)
	if err != nil {
		return ReindexSummary{}, err
	}

	var summary ReindexSummary
	for _, item := range items {
		if indexErr := s.indexKnowledgeItem(ctx, item); indexErr != nil {
			summary.Failures = append(summary.Failures, ReindexFailure{
				ItemID: item.ID, Topic: item.Topic, Reason: indexErr.Error(),
			})
			summary.ItemsFailed++
		} else {
			summary.ItemsIndexed++
		}
		summary.ItemsProcessed++

		if onProgress == nil {
			continue
		}
		if err := onProgress(ReindexProgress{
			ItemsProcessed: summary.ItemsProcessed, ItemsTotal: len(items), CurrentTopic: item.Topic,
		}); err != nil {
			return summary, err
		}
	}
	return summary, nil
}
