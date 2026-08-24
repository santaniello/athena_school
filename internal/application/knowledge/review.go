package knowledge

import (
	"context"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// CountDrafts returns how many Items currently have draft status, for the
// sidebar review badge.
func (s *Service) CountDrafts(ctx context.Context) (int, error) {
	return s.items.CountByStatus(ctx, domainknowledge.StatusDraft)
}
