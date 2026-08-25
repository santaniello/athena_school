package knowledge

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// ExtractFromSession returns unpersisted candidates extracted from a session.
func (s *Service) ExtractFromSession(ctx context.Context, sessionID string, confirmedTruncation bool) ([]domainknowledge.Item, bool, error) {
	session, err := s.sessions.GetByID(ctx, sessionID)
	if err != nil {
		return nil, false, err
	}
	history, err := s.messages.ListBySession(ctx, sessionID)
	if err != nil {
		return nil, false, err
	}
	if len(history) == 0 {
		return nil, false, nil
	}
	cfg, err := s.configs.Load()
	if err != nil {
		return nil, false, err
	}
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, false, err
	}
	prompt, truncated, err := buildExtractionPrompt(history, cfg.MaxKnowledgeExtractionItems)
	if truncated && !confirmedTruncation {
		return nil, true, nil
	}
	if err != nil {
		return nil, truncated, err
	}
	response, err := s.llm.Chat(ctx, domainllm.ChatRequest{
		SessionID: sessionID,
		Task:      domainllm.TaskKnowledgeExtraction,
		Messages:  []domainllm.Message{{Role: "system", Content: prompt}},
	})
	if err != nil {
		return nil, truncated, err
	}
	items, err := parseExtraction(response.Content, session.Topic, cfg.MaxKnowledgeExtractionItems, time.Now().UTC())
	return items, truncated, err
}

// SaveDrafts revalidates and persists confirmed candidates sequentially, as drafts.
func (s *Service) SaveDrafts(ctx context.Context, items []domainknowledge.Item) ([]int, error) {
	return s.saveCandidates(ctx, items, domainknowledge.StatusDraft)
}

// SaveAndApprove revalidates and persists confirmed candidates sequentially,
// directly as approved — skipping the draft review stage. See
// specs/Athena.md §12 ("Save as knowledge"), the third option alongside
// SaveDrafts ("Save as drafts") and discarding the candidates entirely
// ("Dismiss").
func (s *Service) SaveAndApprove(ctx context.Context, items []domainknowledge.Item) ([]int, error) {
	return s.saveCandidates(ctx, items, domainknowledge.StatusApproved)
}

// saveCandidates revalidates and persists confirmed candidates sequentially,
// regenerating every server-owned field and stamping status. Each saved item
// is indexed right after its own persistence — but once one item's indexing
// fails, the rest of the batch is still saved, just no longer indexed: every
// unindexed item (attempted or not) is recovered uniformly by the backfill
// flow, so there is no value in paying for N further failing embedding
// calls in one request (e.g. a missing OpenRouter key). See Design
// decisions in specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md.
func (s *Service) saveCandidates(ctx context.Context, items []domainknowledge.Item, status string) ([]int, error) {
	if err := s.index.BeginMutation(); err != nil {
		return nil, err
	}
	defer s.index.EndMutation()

	savedIndices := make([]int, 0, len(items))
	var indexingErr error
	for index, input := range items {
		topic, topicErr := domainknowledge.NormalizeTopic(input.Topic)
		if topicErr != nil {
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
			continue
		}
		if err := s.items.Save(ctx, item); err != nil {
			return savedIndices, err
		}
		savedIndices = append(savedIndices, index)

		if indexingErr == nil {
			indexingErr = s.indexKnowledgeItem(ctx, item)
		}
	}
	return savedIndices, indexingErr
}
