package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// The three explicit outcomes a conflict proposal can resolve to. See
// ResolveReconciliationConflict.
const (
	ConflictKeepExisting     = "keep_existing"
	ConflictUpdateExisting   = "update_existing"
	ConflictCreateSeparately = "create_separately"
)

// classifyCandidates runs reconciliation classification for every result,
// consuming the duplicate shortlist attachDuplicateMatches already
// attached. A candidate with no shortlist gets a deterministic create
// classification without spending an LLM call. A candidate with a
// shortlist gets exactly one LLM call comparing it against that
// shortlist's full item content; a failed or malformed response falls
// back to the same deterministic create classification, with
// ReconciliationFailed set so the caller can tell the user the comparison
// itself is what failed. results is mutated in place with each
// candidate's Reconciliation/ReconciliationFailed.
func (s *Service) classifyCandidates(ctx context.Context, results []ExtractionCandidate) []reconciliationClassification {
	classifications := make([]reconciliationClassification, len(results))
	for index := range results {
		result := &results[index]
		var classification reconciliationClassification
		if len(result.Duplicates) == 0 {
			classification = deterministicCreateClassification("no existing match found in this topic")
		} else {
			var err error
			classification, err = s.classifyCandidate(ctx, result.Item, result.Duplicates)
			if err != nil {
				result.ReconciliationFailed = true
				classification = deterministicCreateClassification("comparison against existing knowledge failed; showing as new")
			}
		}
		classifications[index] = classification
		result.Reconciliation = &ReconciliationSuggestion{
			Action: classification.Action, TargetItemID: classification.TargetItemID,
			Reason: classification.Reason, Changes: classification.Changes,
		}
	}
	return classifications
}

func deterministicCreateClassification(reason string) reconciliationClassification {
	return reconciliationClassification{Action: domainknowledge.ReconcileCreate, Reason: reason}
}

// classifyCandidate loads the full content of every shortlisted match (at
// most DefaultDuplicateTopK, mirroring the bound on what the prompt may
// ever nominate as a target), asks the LLM to compare candidate against
// them, and parses the validated result.
func (s *Service) classifyCandidate(
	ctx context.Context, candidate domainknowledge.Item, shortlist []domainknowledge.DuplicateMatch,
) (reconciliationClassification, error) {
	limit := min(len(shortlist), domainknowledge.DefaultDuplicateTopK)
	targets := make([]domainknowledge.Item, 0, limit)
	for _, match := range shortlist[:limit] {
		item, err := s.items.GetByID(ctx, match.ItemID)
		if err != nil {
			return reconciliationClassification{}, fmt.Errorf("knowledge: loading reconciliation shortlist item %s: %w", match.ItemID, err)
		}
		targets = append(targets, item)
	}

	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		Task:     domainllm.TaskKnowledgeReconciliation,
		Messages: []domainllm.Message{{Role: "system", Content: buildReconciliationPrompt(candidate, targets)}},
	})
	if err != nil {
		return reconciliationClassification{}, fmt.Errorf("knowledge: reconciliation classification call: %w", err)
	}

	return parseReconciliation(response.Content, targets)
}

// buildReconciliationPrompt asks the LLM to compare candidate against
// targets (at most DefaultDuplicateTopK existing items in the same
// topic) and propose exactly one of the five reconciliation actions.
func buildReconciliationPrompt(candidate domainknowledge.Item, targets []domainknowledge.Item) string {
	var b strings.Builder
	b.WriteString("You compare one newly extracted study concept against existing knowledge items in the same topic and decide how it relates to them.\n")
	b.WriteString("Return only valid JSON, with no markdown fences or commentary, using exactly this envelope schema:\n")
	b.WriteString(`{"action":"create|update|relate|conflict|no_change","target_item_id":"string","reason":"string","changes":{"definition":"string","properties":["string"],"trade_offs":["string"],"related_concepts":["string"]}}`)
	b.WriteString("\n\nRules:\n")
	b.WriteString("- target_item_id is required for update, relate, conflict and no_change, and must be exactly one of the existing item ids listed below. Never invent an id.\n")
	b.WriteString("- target_item_id must be omitted (or empty) for create.\n")
	b.WriteString("- changes is only meaningful for update and conflict. Include only the fields that actually change, each as its full new value — omit a field to leave it unchanged.\n")
	b.WriteString("- \"update\": the new content refines or extends an existing item without contradicting it.\n")
	b.WriteString("- \"relate\": the new content is a distinct concept worth keeping separate, but meaningfully connected to an existing item.\n")
	b.WriteString("- \"conflict\": the new content contradicts an existing item's current content. Explain the contradiction in reason, and still propose changes as if the user chooses to update the existing item.\n")
	b.WriteString("- \"no_change\": the existing item already fully captures the new content.\n")
	b.WriteString("- \"create\": none of the existing items below actually match this concept.\n\n")
	b.WriteString("New candidate:\n")
	b.WriteString(renderItemForReconciliation(candidate))
	b.WriteString("\nExisting items in this topic:\n")
	for _, target := range targets {
		fmt.Fprintf(&b, "[id:%s status:%s]\n", target.ID, target.Status)
		b.WriteString(renderItemForReconciliation(target))
	}
	return b.String()
}

func renderItemForReconciliation(item domainknowledge.Item) string {
	var b strings.Builder
	b.WriteString("Concept: " + item.Concept + "\n")
	b.WriteString("Definition: " + item.Definition + "\n")
	if len(item.Properties) > 0 {
		b.WriteString("Properties: " + strings.Join(item.Properties, "; ") + "\n")
	}
	if len(item.TradeOffs) > 0 {
		b.WriteString("Trade-offs: " + strings.Join(item.TradeOffs, "; ") + "\n")
	}
	if len(item.RelatedConcepts) > 0 {
		b.WriteString("Related concepts: " + strings.Join(item.RelatedConcepts, "; ") + "\n")
	}
	return b.String()
}

type reconciliationEnvelope struct {
	Action       string                    `json:"action"`
	TargetItemID string                    `json:"target_item_id"`
	Reason       string                    `json:"reason"`
	Changes      *reconciliationChangesDTO `json:"changes"`
}

type reconciliationChangesDTO struct {
	Definition      *string  `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"trade_offs"`
	RelatedConcepts []string `json:"related_concepts"`
}

// parseReconciliation parses and validates the classifier's JSON response.
// It cannot nominate a target outside targets — Go, not the LLM, enforces
// that boundary — and any structural problem (malformed JSON, unknown
// action, missing/out-of-shortlist target) returns ErrMalformedReconciliation
// rather than guessing.
func parseReconciliation(raw string, targets []domainknowledge.Item) (reconciliationClassification, error) {
	object, ok := extractJSONObject(raw)
	if !ok {
		return reconciliationClassification{}, ErrMalformedReconciliation
	}
	var envelope reconciliationEnvelope
	if err := json.Unmarshal([]byte(object), &envelope); err != nil {
		return reconciliationClassification{}, ErrMalformedReconciliation
	}
	if !knownReconciliationAction(envelope.Action) {
		return reconciliationClassification{}, ErrMalformedReconciliation
	}
	reason := strings.TrimSpace(envelope.Reason)
	if reason == "" {
		return reconciliationClassification{}, ErrMalformedReconciliation
	}

	classification := reconciliationClassification{Action: envelope.Action, Reason: reason}
	if envelope.Action == domainknowledge.ReconcileCreate {
		if envelope.Changes != nil {
			classification.Changes = mapReconciliationChanges(envelope.Changes)
		}
		return classification, nil
	}

	target, found := findReconciliationTarget(targets, envelope.TargetItemID)
	if !found {
		return reconciliationClassification{}, ErrMalformedReconciliation
	}
	classification.TargetItemID = target.ID
	classification.TargetUpdatedAt = target.UpdatedAt
	if envelope.Changes != nil {
		classification.Changes = mapReconciliationChanges(envelope.Changes)
	}
	return classification, nil
}

func knownReconciliationAction(action string) bool {
	switch action {
	case domainknowledge.ReconcileCreate, domainknowledge.ReconcileUpdate, domainknowledge.ReconcileRelate,
		domainknowledge.ReconcileConflict, domainknowledge.ReconcileNoChange:
		return true
	}
	return false
}

func findReconciliationTarget(targets []domainknowledge.Item, targetItemID string) (domainknowledge.Item, bool) {
	targetItemID = strings.TrimSpace(targetItemID)
	if targetItemID == "" {
		return domainknowledge.Item{}, false
	}
	for _, target := range targets {
		if target.ID == targetItemID {
			return target, true
		}
	}
	return domainknowledge.Item{}, false
}

func mapReconciliationChanges(dto *reconciliationChangesDTO) domainknowledge.ItemChanges {
	changes := domainknowledge.ItemChanges{}
	if dto.Definition != nil {
		truncated := truncateString(*dto.Definition, maxDefinitionChars)
		changes.Definition = &truncated
	}
	if dto.Properties != nil {
		changes.Properties = normalizeList(dto.Properties)
	}
	if dto.TradeOffs != nil {
		changes.TradeOffs = normalizeList(dto.TradeOffs)
	}
	if dto.RelatedConcepts != nil {
		changes.RelatedConcepts = normalizeList(dto.RelatedConcepts)
	}
	return changes
}

// ApplyReconciliationCreate persists candidateID's classified candidate as
// a brand-new Knowledge Item at status (draft or approved), ignoring every
// candidate/client id — the same server-side identity regeneration
// saveCandidates already applies to a plain extraction save.
func (s *Service) ApplyReconciliationCreate(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item, status string,
) (domainknowledge.Item, error) {
	return s.applyReconciliationMutation(ctx, batchID, candidateID, candidate, "",
		func(ctx context.Context, _ candidateReceipt, _ domainknowledge.Item) (domainknowledge.Item, error) {
			return s.createReconciledItem(ctx, candidate, status)
		},
	)
}

// ApplyReconciliationUpdate applies the classified ItemChanges to the
// classified target, preserving its identity, lifecycle and creation time.
func (s *Service) ApplyReconciliationUpdate(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item,
) (domainknowledge.Item, error) {
	return s.applyReconciliationMutation(ctx, batchID, candidateID, candidate, "",
		func(ctx context.Context, receipt candidateReceipt, target domainknowledge.Item) (domainknowledge.Item, error) {
			return s.updateReconciledItem(ctx, target, receipt.Reconciliation.Changes)
		},
	)
}

// ApplyReconciliationRelate creates candidateID's candidate as a new
// Knowledge Item (always as a draft — a related item is never an exact
// match, so it is never auto-approved) and links it to the classified
// target via a canonical `related` relation.
func (s *Service) ApplyReconciliationRelate(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item,
) (domainknowledge.Item, error) {
	return s.applyReconciliationMutation(ctx, batchID, candidateID, candidate, "",
		func(ctx context.Context, _ candidateReceipt, target domainknowledge.Item) (domainknowledge.Item, error) {
			item, err := s.createReconciledItem(ctx, candidate, domainknowledge.StatusDraft)
			if err != nil {
				return domainknowledge.Item{}, err
			}
			relation := domainknowledge.CanonicalRelation(item.ID, target.ID, domainknowledge.RelationRelated, time.Now().UTC())
			if err := s.relations.Save(ctx, relation); err != nil {
				return domainknowledge.Item{}, err
			}
			return item, nil
		},
	)
}

// ResolveReconciliationConflict applies one of the three explicit conflict
// outcomes — the only way a conflict proposal may resolve. keep_existing
// performs no item mutation; update_existing has the same item effect as
// ApplyReconciliationUpdate; create_separately has the same item effect as
// ApplyReconciliationCreate, always as a draft. Every outcome keeps the
// proposal's Action as `conflict` — the resolution itself is recorded in
// its Reason — so the audit record still shows the LLM actually detected
// a conflict here.
func (s *Service) ResolveReconciliationConflict(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item, resolution string,
) (domainknowledge.Item, error) {
	switch resolution {
	case ConflictKeepExisting:
		err := s.resolveReconciliationNoMutation(ctx, batchID, candidateID, candidate, domainknowledge.ProposalApplied, true, "resolved: kept existing item")
		return domainknowledge.Item{}, err
	case ConflictUpdateExisting:
		return s.applyReconciliationMutation(ctx, batchID, candidateID, candidate, "resolved: updated existing item",
			func(ctx context.Context, receipt candidateReceipt, target domainknowledge.Item) (domainknowledge.Item, error) {
				return s.updateReconciledItem(ctx, target, receipt.Reconciliation.Changes)
			},
		)
	case ConflictCreateSeparately:
		return s.applyReconciliationMutation(ctx, batchID, candidateID, candidate, "resolved: created separately",
			func(ctx context.Context, _ candidateReceipt, _ domainknowledge.Item) (domainknowledge.Item, error) {
				return s.createReconciledItem(ctx, candidate, domainknowledge.StatusDraft)
			},
		)
	default:
		return domainknowledge.Item{}, ErrReconciliationResolutionInvalid
	}
}

// AcknowledgeReconciliationNoChange marks the classified no_change
// proposal resolved without creating or changing any Item, preserving it
// as the audit record.
func (s *Service) AcknowledgeReconciliationNoChange(ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item) error {
	return s.resolveReconciliationNoMutation(ctx, batchID, candidateID, candidate, domainknowledge.ProposalApplied, true, "")
}

// SaveReconciliationForReview persists the classified proposal as
// pending, changing neither the candidate nor its target — Knowledge
// Review is where it is later decided. No staleness check runs here: a
// pending proposal is allowed to go stale while it waits: the review flow
// that eventually acts on it is what must refuse a stale application.
func (s *Service) SaveReconciliationForReview(ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item) error {
	return s.resolveReconciliationNoMutation(ctx, batchID, candidateID, candidate, domainknowledge.ProposalPending, false, "")
}

// applyReconciliationMutation is the shared engine behind every
// reconciliation resolution that creates or changes a Knowledge Item. It
// claims candidateID's receipt, revalidates its evidence against the
// source session's current Messages — exactly as saveCandidates already
// does for plain extraction — and, inside one transaction, re-reads the
// classified target (when the receipt has one) to enforce the
// TargetUpdatedAt staleness check before mutate runs. mutate performs the
// actual item effect; persistReconciliationDecision then writes the
// resulting ReconciliationProposal and its evidence links as the
// permanent audit record. Indexing runs post-commit, mirroring
// saveCandidates/UpdateItem: a failure there never rolls back the durable
// write.
//
// A claim that does not end up resolved — an invalid receipt, invalid
// evidence, a stale target, or a failed transaction — restores the
// receipt for retry, mirroring saveCandidates' own Claim/Restore contract.
func (s *Service) applyReconciliationMutation(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item, resolutionNote string,
	mutate func(ctx context.Context, receipt candidateReceipt, target domainknowledge.Item) (domainknowledge.Item, error),
) (domainknowledge.Item, error) {
	if err := s.index.BeginMutation(); err != nil {
		return domainknowledge.Item{}, err
	}
	defer s.index.EndMutation()

	receipt, claimed := s.receipts.Claim(batchID, candidateID)
	if !claimed {
		return domainknowledge.Item{}, ErrReconciliationCandidateNotFound
	}
	messagesByID, err := s.loadMessagesByID(ctx, receipt.SessionID)
	if err != nil {
		s.receipts.Restore(batchID, candidateID, receipt)
		return domainknowledge.Item{}, err
	}
	validRefs := revalidateEvidenceRefs(receipt, messagesByID)
	if len(validRefs) == 0 {
		s.receipts.Restore(batchID, candidateID, receipt)
		return domainknowledge.Item{}, ErrReconciliationEvidenceInvalid
	}

	var resultItem domainknowledge.Item
	txErr := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var target domainknowledge.Item
		if receipt.Reconciliation.TargetItemID != "" {
			var err error
			target, err = s.checkReconciliationTargetFresh(ctx, receipt.Reconciliation)
			if err != nil {
				return err
			}
		}
		item, mutateErr := mutate(ctx, receipt, target)
		if mutateErr != nil {
			return mutateErr
		}
		resultItem = item
		return s.persistReconciliationDecision(ctx, candidate, receipt, domainknowledge.ProposalApplied, validRefs, resolutionNote)
	})
	if txErr != nil {
		s.receipts.Restore(batchID, candidateID, receipt)
		return domainknowledge.Item{}, txErr
	}

	if err := s.indexKnowledgeItem(ctx, resultItem); err != nil {
		return resultItem, err
	}
	return resultItem, nil
}

// resolveReconciliationNoMutation is applyReconciliationMutation's
// counterpart for a decision that never creates or changes an Item:
// acknowledging no_change, keeping the existing item on a conflict, or
// saving any classified action for review. checkStale mirrors
// applyReconciliationMutation's own staleness check — skipped for
// SaveReconciliationForReview, since a proposal is allowed to go stale
// while pending.
func (s *Service) resolveReconciliationNoMutation(
	ctx context.Context, batchID, candidateID string, candidate domainknowledge.Item,
	finalStatus string, checkStale bool, resolutionNote string,
) error {
	receipt, claimed := s.receipts.Claim(batchID, candidateID)
	if !claimed {
		return ErrReconciliationCandidateNotFound
	}
	messagesByID, err := s.loadMessagesByID(ctx, receipt.SessionID)
	if err != nil {
		s.receipts.Restore(batchID, candidateID, receipt)
		return err
	}
	validRefs := revalidateEvidenceRefs(receipt, messagesByID)
	if len(validRefs) == 0 {
		s.receipts.Restore(batchID, candidateID, receipt)
		return ErrReconciliationEvidenceInvalid
	}

	txErr := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if checkStale && receipt.Reconciliation.TargetItemID != "" {
			if _, err := s.checkReconciliationTargetFresh(ctx, receipt.Reconciliation); err != nil {
				return err
			}
		}
		return s.persistReconciliationDecision(ctx, candidate, receipt, finalStatus, validRefs, resolutionNote)
	})
	if txErr != nil {
		s.receipts.Restore(batchID, candidateID, receipt)
		return txErr
	}
	return nil
}

// checkReconciliationTargetFresh reloads classification's target and
// returns ErrReconciliationTargetStale if it was removed or edited since
// classification — the optimistic concurrency check every action with a
// target must pass before it may proceed.
func (s *Service) checkReconciliationTargetFresh(ctx context.Context, classification reconciliationClassification) (domainknowledge.Item, error) {
	target, err := s.items.GetByID(ctx, classification.TargetItemID)
	if errors.Is(err, domainknowledge.ErrItemNotFound) {
		return domainknowledge.Item{}, ErrReconciliationTargetStale
	}
	if err != nil {
		return domainknowledge.Item{}, err
	}
	if !target.UpdatedAt.Equal(classification.TargetUpdatedAt) {
		return domainknowledge.Item{}, ErrReconciliationTargetStale
	}
	return target, nil
}

// createReconciledItem builds a brand-new Item from candidate's content —
// regenerating its ID and stamping fresh timestamps, exactly like
// saveCandidates — rechecks the exact-duplicate policy inside the same
// transaction (closing the same check-then-act race saveCandidates
// closes), and persists it.
func (s *Service) createReconciledItem(ctx context.Context, candidate domainknowledge.Item, status string) (domainknowledge.Item, error) {
	topic, err := domainknowledge.NormalizeTopic(candidate.Topic)
	if err != nil {
		return domainknowledge.Item{}, err
	}
	now := time.Now().UTC()
	item := domainknowledge.Item{
		ID:              uuid.NewString(),
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

// persistReconciliationDecision builds a ReconciliationProposal from
// candidate and receipt's classified fields plus the resolution actually
// applied, validates it, and persists its header row and evidence links
// — the permanent audit record for every reconciliation decision, applied
// immediately or saved for review alike.
func (s *Service) persistReconciliationDecision(
	ctx context.Context, candidate domainknowledge.Item, receipt candidateReceipt,
	status string, validRefs []domainknowledge.EvidenceRef, resolutionNote string,
) error {
	now := time.Now().UTC()
	reason := receipt.Reconciliation.Reason
	if resolutionNote != "" {
		reason = reason + "; " + resolutionNote
	}
	proposal := domainknowledge.ReconciliationProposal{
		ID: uuid.NewString(), Action: receipt.Reconciliation.Action, Status: status,
		Candidate: candidate, TargetItemID: receipt.Reconciliation.TargetItemID,
		TargetUpdatedAt: receipt.Reconciliation.TargetUpdatedAt,
		Reason:          reason, Changes: receipt.Reconciliation.Changes, CreatedAt: now,
	}
	if err := proposal.Validate(); err != nil {
		return err
	}
	if err := s.reconciliations.Save(ctx, proposal); err != nil {
		return err
	}
	for _, ref := range validRefs {
		evidence, err := s.evidence.GetOrCreate(ctx, domainknowledge.Evidence{
			ID: uuid.NewString(), OriginType: domainknowledge.OriginSessionMessage,
			OriginID: ref.MessageID, SourceLabel: receipt.SourceLabel, Excerpt: ref.Quote, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := s.reconciliations.LinkEvidence(ctx, proposal.ID, evidence.ID); err != nil {
			return err
		}
	}
	return nil
}
