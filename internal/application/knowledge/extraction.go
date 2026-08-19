package knowledge

import (
	"context"
	"strings"
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
// specs/Athena.md §12 ("Salvar como conhecimento"), the third option
// alongside SaveDrafts ("Salvar como rascunho") and discarding the
// candidates entirely ("Ignorar").
func (s *Service) SaveAndApprove(ctx context.Context, items []domainknowledge.Item) ([]int, error) {
	return s.saveCandidates(ctx, items, domainknowledge.StatusApproved)
}

// saveCandidates revalidates and persists confirmed candidates sequentially,
// regenerating every server-owned field and stamping status.
func (s *Service) saveCandidates(ctx context.Context, items []domainknowledge.Item, status string) ([]int, error) {
	savedIndices := make([]int, 0, len(items))
	for index, input := range items {
		now := time.Now().UTC()
		item := domainknowledge.Item{
			ID:              uuid.NewString(),
			Topic:           strings.TrimSpace(input.Topic),
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
	}
	return savedIndices, nil
}
