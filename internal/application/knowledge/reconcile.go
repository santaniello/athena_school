package knowledge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// The three explicit outcomes a conflict proposal can resolve to. See
// ResolveReconciliationConflict.
const (
	ConflictKeepExisting     = "keep_existing"
	ConflictUpdateExisting   = "update_existing"
	ConflictCreateSeparately = "create_separately"
)

// checkReconciliationTargetFresh reloads targetItemID and returns
// ErrReconciliationTargetStale if it was removed or no longer has
// targetUpdatedAt — the optimistic concurrency check every action with a
// target must pass before it may proceed, whether the classification is
// still in a transient receipt (see applyReconciliationMutation) or was
// reloaded from a persisted, pending ReconciliationProposal (see
// applyPendingReconciliationMutation in reconcile_pending.go).
func (s *Service) checkReconciliationTargetFresh(ctx context.Context, targetItemID string, targetUpdatedAt time.Time) (domainknowledge.Item, error) {
	target, err := s.items.GetByID(ctx, targetItemID)
	if errors.Is(err, domainknowledge.ErrItemNotFound) {
		return domainknowledge.Item{}, ErrReconciliationTargetStale
	}
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if !target.UpdatedAt.Equal(targetUpdatedAt) {
		return domainknowledge.Item{}, ErrReconciliationTargetStale
	}
	return target, nil
}

// createReconciledItem builds a brand-new Item from candidate's content —
// regenerating its ID and stamping fresh timestamps, exactly like
// saveCandidates — rechecks the exact-duplicate policy inside the same
// transaction (closing the same check-then-act race saveCandidates
// closes), and persists it as owned by sessionID — never by anything
// candidate itself claims, since a client-supplied candidate is not trusted
// for ownership.
func (s *Service) createReconciledItem(ctx context.Context, sessionID string, candidate domainknowledge.Item, status string) (domainknowledge.Item, error) {
	topic, err := domainknowledge.NormalizeTopic(candidate.Topic)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	now := time.Now().UTC()
	item := domainknowledge.Item{
		ID:              uuid.NewString(),
		SessionID:       sessionID,
		Topic:           topic,
		Concept:         truncateString(candidate.Concept, maxConceptChars),
		Definition:      truncateString(candidate.Definition, maxDefinitionChars),
		Properties:      normalizeList(candidate.Properties),
		TradeOffs:       normalizeList(candidate.TradeOffs),
		RelatedConcepts: normalizeList(candidate.RelatedConcepts),
		Source:          domainknowledge.SourceAthena,
		Status:          status,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := item.Validate(); err != nil {
		return domainknowledge.Item{}, err
	}
	exactMatches, err := s.findExactDuplicates(ctx, item)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if len(exactMatches) > 0 {
		return domainknowledge.Item{}, errExactDuplicateAtSave
	}
	if err := s.items.Save(ctx, item); err != nil {
		return domainknowledge.Item{}, err
	}
	return item, nil
}

// updateReconciledItem applies changes' set fields onto target, restamps
// UpdatedAt, and persists it — target's ID, Topic, Source, Status and
// CreatedAt are never touched.
func (s *Service) updateReconciledItem(ctx context.Context, target domainknowledge.Item, changes domainknowledge.ItemChanges) (domainknowledge.Item, error) {
	if changes.Definition != nil {
		target.Definition = truncateString(*changes.Definition, maxDefinitionChars)
	}
	if changes.Properties != nil {
		target.Properties = normalizeList(changes.Properties)
	}
	if changes.TradeOffs != nil {
		target.TradeOffs = normalizeList(changes.TradeOffs)
	}
	if changes.RelatedConcepts != nil {
		target.RelatedConcepts = normalizeList(changes.RelatedConcepts)
	}
	target.UpdatedAt = time.Now().UTC()
	if err := target.Validate(); err != nil {
		return domainknowledge.Item{}, err
	}
	if err := s.items.Update(ctx, target); err != nil {
		return domainknowledge.Item{}, err
	}
	return target, nil
}

const (
	maxConceptChars    = 120
	maxDefinitionChars = 2000
	maxListItems       = 10
	maxListEntryChars  = 200
)

func truncateString(value string, maxChars int) string {
	runes := []rune(strings.TrimSpace(value))
	return string(runes[:min(len(runes), maxChars)])
}

func normalizeList(values []string) []string {
	result := make([]string, 0, min(len(values), maxListItems))
	for _, value := range values {
		value = truncateString(value, maxListEntryChars)
		if value != "" && len(result) < maxListItems {
			result = append(result, value)
		}
	}
	return result
}
