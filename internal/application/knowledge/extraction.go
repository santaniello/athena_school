package knowledge

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ExtractionBatch contains transient candidates tied to server-side receipts.
type ExtractionBatch struct {
	ID    string
	Items []ExtractionCandidate
}

// ExtractionCandidate pairs one unpersisted candidate Item with the
// duplicate matches found for it. Duplicates is empty both when nothing
// matched and when SemanticCheckUnavailable is true — the two are
// distinguished only by that flag, never by an error, since a candidate
// with an incomplete check must still be usable. See
// specs/phases/phase-02-knowledge-engine/10-duplicate-detection.md.
type ExtractionCandidate struct {
	Item                     domainknowledge.Item
	Duplicates               []domainknowledge.DuplicateMatch
	SemanticCheckUnavailable bool
}

// ExtractFromSession returns unpersisted candidates extracted from a session.
func (s *Service) ExtractFromSession(ctx context.Context, sessionID string, confirmedTruncation bool) (ExtractionBatch, bool, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return ExtractionBatch{}, false, err
	}
	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return ExtractionBatch{}, false, err
	}
	if len(history) == 0 {
		return ExtractionBatch{}, false, nil
	}
	cfg, err := s.configs.Load()
	if err != nil {
		return ExtractionBatch{}, false, err
	}
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return ExtractionBatch{}, false, err
	}
	prompt, includedMessages, truncated, err := buildExtractionPrompt(history, cfg.MaxKnowledgeExtractionItems)
	if truncated && !confirmedTruncation {
		return ExtractionBatch{}, true, nil
	}
	if err != nil {
		return ExtractionBatch{}, truncated, err
	}
	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskKnowledgeExtraction,
		Messages:  []domainllm.Message{{Role: "system", Content: prompt}},
	})
	if err != nil {
		return ExtractionBatch{}, truncated, err
	}
	candidates, err := parseExtraction(response.Content, session.Topic, cfg.MaxKnowledgeExtractionItems, time.Now().UTC(), includedMessages)
	if err != nil {
		return ExtractionBatch{}, truncated, err
	}
	items := make([]domainknowledge.Item, len(candidates))
	for index, candidate := range candidates {
		items[index] = candidate.Item
	}
	if len(items) == 0 {
		return ExtractionBatch{Items: []ExtractionCandidate{}}, truncated, nil
	}
	results, err := s.attachDuplicateMatches(ctx, items)
	if err != nil {
		return ExtractionBatch{}, truncated, err
	}
	batchID := s.receipts.Create(sessionID, session.Topic, candidates)
	return ExtractionBatch{ID: batchID, Items: results}, truncated, nil
}

// attachDuplicateMatches runs duplicate detection for every candidate,
// short-circuiting the semantic stage only, for every candidate after the
// first one whose own semantic check fails in this batch — avoiding N
// further embedding calls that would very likely fail the same way,
// mirroring saveCandidates' own indexing short-circuit. The exact stage
// never short-circuits: it is a plain, inexpensive database lookup, run
// independently for every candidate regardless of this state. See
// specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
// Decision 2.
func (s *Service) attachDuplicateMatches(ctx context.Context, items []domainknowledge.Item) ([]ExtractionCandidate, error) {
	results := make([]ExtractionCandidate, len(items))
	semanticCheckUnavailable := false
	for index, item := range items {
		exactMatches, err := s.findExactDuplicates(ctx, item)
		if err != nil {
			return nil, err
		}

		result := ExtractionCandidate{Item: item}
		switch {
		case len(exactMatches) > 0:
			result.Duplicates = exactMatches
		case semanticCheckUnavailable:
			result.SemanticCheckUnavailable = true
		default:
			semanticMatches, semErr := s.findSemanticDuplicates(ctx, item, s.duplicateTopK, s.duplicateMinScore)
			if semErr != nil {
				semanticCheckUnavailable = true
				result.SemanticCheckUnavailable = true
			} else {
				result.Duplicates = semanticMatches
			}
		}
		results[index] = result
	}
	return results, nil
}

// SaveDrafts revalidates and persists confirmed candidates sequentially, as drafts.
func (s *Service) SaveDrafts(ctx context.Context, batchID string, items []domainknowledge.Item) ([]int, error) {
	return s.saveCandidates(ctx, batchID, items, domainknowledge.StatusDraft)
}

// SaveAndApprove revalidates and persists confirmed candidates sequentially,
// directly as approved — skipping the draft review stage. See
// specs/Athena.md §12 ("Save as knowledge"), the third option alongside
// SaveDrafts ("Save as drafts") and discarding the candidates entirely
// ("Dismiss").
func (s *Service) SaveAndApprove(ctx context.Context, batchID string, items []domainknowledge.Item) ([]int, error) {
	return s.saveCandidates(ctx, batchID, items, domainknowledge.StatusApproved)
}

// saveCandidates revalidates and persists confirmed candidates sequentially,
// regenerating every server-owned field and stamping status. The frontend is
// never trusted for provenance: each input's ID is used only as an opaque
// key into that candidate's backend receipt (see receipt_store.go), which
// carries the source Study Session and the EvidenceRefs validated at
// extraction time. Save reloads that session's Messages and repeats the
// ownership and verbatim-quote checks against their current content, so an
// edited-around-the-quote Message stays valid while a deleted Message or a
// Message that no longer contains the quote makes that candidate unsavable.
// A candidate left without any valid Evidence is skipped without touching
// its saved siblings.
//
// Each Item, together with its immutable Evidence snapshots and links, is
// saved in its own SQLite transaction — a failure leaves neither behind. A
// candidate's receipt is claimed before that transaction (so a concurrent
// call cannot also claim and save it) and restored if the candidate does
// not end up persisted — invalid evidence, a failed transaction — keeping
// it available for retry; siblings not yet attempted keep theirs too.
//
// Each saved item is indexed right after its own persistence — but once one
// item's indexing fails, the rest of the batch is still saved (and its
// receipt still consumed), just no longer indexed: every unindexed item
// (attempted or not) is recovered uniformly by the backfill flow, so there
// is no value in paying for N further failing embedding calls in one
// request (e.g. a missing OpenRouter key). See Design decisions in
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md.
func (s *Service) saveCandidates(ctx context.Context, batchID string, items []domainknowledge.Item, status string) ([]int, error) {
	if err := s.index.BeginMutation(); err != nil {
		return nil, err
	}
	defer s.index.EndMutation()

	savedIndices := make([]int, 0, len(items))
	// Every candidate in one batch shares its extraction's SessionID (see
	// receiptStore.Create), so this cache loads each cited session's
	// Messages at most once per save, instead of once per candidate.
	messagesBySession := make(map[string]map[string]domainstudy.Message)
	var indexingErr error
	for index, input := range items {
		topic, topicErr := domainknowledge.NormalizeTopic(input.Topic)
		if topicErr != nil {
			continue
		}
		receipt, claimed := s.receipts.Claim(batchID, input.ID)
		if !claimed {
			continue
		}
		messagesByID, cached := messagesBySession[receipt.SessionID]
		if !cached {
			var err error
			messagesByID, err = s.loadMessagesByID(ctx, receipt.SessionID)
			if err != nil {
				s.receipts.Restore(batchID, input.ID, receipt)
				return savedIndices, err
			}
			messagesBySession[receipt.SessionID] = messagesByID
		}
		validRefs := revalidateEvidenceRefs(receipt, messagesByID)
		if len(validRefs) == 0 {
			s.receipts.Restore(batchID, input.ID, receipt)
			continue
		}

		now := time.Now().UTC()
		item := domainknowledge.Item{
			ID:              uuid.NewString(),
			Topic:           topic,
			Concept:         truncateString(input.Concept, maxConceptChars),
			Definition:      truncateString(input.Definition, maxDefinitionChars),
			Properties:      normalizeList(input.Properties),
			TradeOffs:       normalizeList(input.TradeOffs),
			RelatedConcepts: normalizeList(input.RelatedConcepts),
			Source:          domainknowledge.SourceAthena,
			Status:          status,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if item.Validate() != nil {
			s.receipts.Restore(batchID, input.ID, receipt)
			continue
		}

		err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
			// The exact-match recheck runs inside this same transaction,
			// not before it: db.go's single-connection pool means the
			// connection this transaction holds from here to Commit is the
			// only one in the process, so no concurrent save can observe
			// "no match" and insert between this check and this Save —
			// mirroring the read-then-write pattern Transactor's own doc
			// comment already establishes for Approve/Deprecate/UpdateItem/
			// DeleteItem. A crafted or stale frontend still cannot bypass
			// the save policy this way either.
			exactMatches, dupErr := s.findExactDuplicates(ctx, item)
			if dupErr != nil {
				return dupErr
			}
			if len(exactMatches) > 0 {
				// Skipped silently, the same as invalid evidence above —
				// see specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
				// Decision 3.
				return errExactDuplicateAtSave
			}

			if err := s.items.Save(ctx, item); err != nil {
				return err
			}
			for _, ref := range validRefs {
				evidence, err := s.evidence.GetOrCreate(ctx, domainknowledge.Evidence{
					ID:          uuid.NewString(),
					OriginType:  domainknowledge.OriginSessionMessage,
					OriginID:    ref.MessageID,
					SourceLabel: receipt.SourceLabel,
					Excerpt:     ref.Quote,
					CreatedAt:   now,
				})
				if err != nil {
					return err
				}
				if err := s.evidence.LinkToItem(ctx, domainknowledge.ItemEvidence{ItemID: item.ID, EvidenceID: evidence.ID}); err != nil {
					return err
				}
			}
			return nil
		})
		if errors.Is(err, errExactDuplicateAtSave) {
			s.receipts.Restore(batchID, input.ID, receipt)
			continue
		}
		if err != nil {
			s.receipts.Restore(batchID, input.ID, receipt)
			return savedIndices, err
		}
		savedIndices = append(savedIndices, index)

		if indexingErr == nil {
			indexingErr = s.indexKnowledgeItem(ctx, item)
		}
	}
	return savedIndices, indexingErr
}

// DiscardExtraction drops every pending receipt in batchID — the backend
// counterpart of the user dismissing an extraction batch without saving it.
// It never runs after a partial save error: pending receipts for
// unattempted or failed candidates must remain available for retry.
func (s *Service) DiscardExtraction(batchID string) {
	s.receipts.Discard(batchID)
}

// loadMessagesByID loads sessionID's Study Session Messages, keyed by ID.
func (s *Service) loadMessagesByID(ctx context.Context, sessionID string) (map[string]domainstudy.Message, error) {
	messages, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	messagesByID := make(map[string]domainstudy.Message, len(messages))
	for _, message := range messages {
		messagesByID[message.ID] = message
	}
	return messagesByID, nil
}

// revalidateEvidenceRefs keeps only the EvidenceRefs whose Message still
// exists in messagesByID and still supports the exact quote — repeating, at
// save time, the ownership and verbatim checks already performed once at
// extraction time.
func revalidateEvidenceRefs(receipt candidateReceipt, messagesByID map[string]domainstudy.Message) []domainknowledge.EvidenceRef {
	valid := make([]domainknowledge.EvidenceRef, 0, len(receipt.EvidenceRefs))
	for _, ref := range receipt.EvidenceRefs {
		message, exists := messagesByID[ref.MessageID]
		if !exists || !ref.IsSupportedBy(message.Content) {
			continue
		}
		valid = append(valid, ref)
	}
	return valid
}
