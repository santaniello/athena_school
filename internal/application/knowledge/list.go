package knowledge

import (
	"context"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// ListItems returns every Item matching topic/status. An empty topic or
// status means no constraint on that field.
func (s *Service) ListItems(ctx context.Context, topic, status string) ([]domainknowledge.Item, error) {
	return s.items.List(ctx, domainknowledge.Filter{Topic: topic, Status: status})
}

// ListTopics returns every distinct topic across all Items, alphabetically.
func (s *Service) ListTopics(ctx context.Context) ([]string, error) {
	return s.items.ListTopics(ctx)
}

// ListItemEvidence returns itemID's persisted Evidence snapshots in
// deterministic order — empty for a legacy or shadow Item that never went
// through evidence-bearing extraction.
func (s *Service) ListItemEvidence(ctx context.Context, itemID string) ([]domainknowledge.Evidence, error) {
	return s.evidence.ListByItem(ctx, itemID)
}
