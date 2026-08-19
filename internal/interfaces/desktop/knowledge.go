package desktop

import (
	"errors"
	"fmt"
	"log"
	"time"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// KnowledgeItemResult is an unpersisted extraction candidate returned to the UI.
type KnowledgeItemResult struct {
	ID              string   `json:"id"`
	Topic           string   `json:"topic"`
	Concept         string   `json:"concept"`
	Definition      string   `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"tradeOffs"`
	RelatedConcepts []string `json:"relatedConcepts"`
	Source          string   `json:"source"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// KnowledgeItemInput mirrors the full candidate returned by ExtractKnowledge.
type KnowledgeItemInput struct {
	ID              string   `json:"id"`
	Topic           string   `json:"topic"`
	Concept         string   `json:"concept"`
	Definition      string   `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"tradeOffs"`
	RelatedConcepts []string `json:"relatedConcepts"`
	Source          string   `json:"source"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// ExtractionResult carries candidates and the transcript truncation signal.
type ExtractionResult struct {
	Items     []KnowledgeItemResult `json:"items"`
	Truncated bool                  `json:"truncated"`
}

// KnowledgeSaveResult identifies persisted inputs even when a later save fails.
type KnowledgeSaveResult struct {
	SavedIndices []int  `json:"savedIndices"`
	Error        string `json:"error"`
}

// ExtractKnowledge extracts unpersisted knowledge candidates for review.
func (a *App) ExtractKnowledge(sessionID string, confirmedTruncation bool) (ExtractionResult, error) {
	items, truncated, err := a.knowledge.ExtractFromSession(a.ctx, sessionID, confirmedTruncation)
	if errors.Is(err, applicationknowledge.ErrMalformedExtraction) {
		log.Printf("knowledge extraction returned malformed JSON: %v", err)
		return ExtractionResult{Items: []KnowledgeItemResult{}}, nil
	}
	if err != nil {
		return ExtractionResult{}, err
	}
	results := make([]KnowledgeItemResult, len(items))
	for index, item := range items {
		results[index] = toKnowledgeItemResult(item)
	}
	return ExtractionResult{Items: results, Truncated: truncated}, nil
}

// SaveExtractedKnowledge persists only the candidates confirmed by the user.
func (a *App) SaveExtractedKnowledge(inputs []KnowledgeItemInput) KnowledgeSaveResult {
	items := make([]domainknowledge.Item, len(inputs))
	for index, input := range inputs {
		items[index] = domainknowledge.Item{
			ID: input.ID, Topic: input.Topic, Concept: input.Concept, Definition: input.Definition,
			Properties: input.Properties, TradeOffs: input.TradeOffs, RelatedConcepts: input.RelatedConcepts,
			Source: input.Source, Status: input.Status,
		}
	}
	savedIndices, err := a.knowledge.SaveDrafts(a.ctx, items)
	result := KnowledgeSaveResult{SavedIndices: savedIndices}
	if err != nil {
		result.Error = fmt.Sprintf("knowledge save failed: %v", err)
	}
	return result
}

// ListKnowledgeItems returns every Item matching topic/status. An empty
// topic or status means no constraint on that field.
func (a *App) ListKnowledgeItems(topic, status string) ([]KnowledgeItemResult, error) {
	items, err := a.knowledge.ListItems(a.ctx, topic, status)
	if err != nil {
		return nil, err
	}
	results := make([]KnowledgeItemResult, len(items))
	for index, item := range items {
		results[index] = toKnowledgeItemResult(item)
	}
	return results, nil
}

// ListKnowledgeTopics returns every distinct topic, alphabetically.
func (a *App) ListKnowledgeTopics() ([]string, error) {
	return a.knowledge.ListTopics(a.ctx)
}

// ApproveKnowledgeItem transitions id from draft to approved and returns
// the updated item.
func (a *App) ApproveKnowledgeItem(id string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.Approve(a.ctx, id)
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// DeprecateKnowledgeItem transitions id from approved to deprecated and
// returns the updated item.
func (a *App) DeprecateKnowledgeItem(id string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.Deprecate(a.ctx, id)
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// UpdateKnowledgeItem overwrites id's editable fields and returns the
// updated item. Status, Source and CreatedAt are never touched.
func (a *App) UpdateKnowledgeItem(id string, input KnowledgeItemInput) (KnowledgeItemResult, error) {
	item, err := a.knowledge.UpdateItem(a.ctx, id, applicationknowledge.ItemFields{
		Topic:           input.Topic,
		Concept:         input.Concept,
		Definition:      input.Definition,
		Properties:      input.Properties,
		TradeOffs:       input.TradeOffs,
		RelatedConcepts: input.RelatedConcepts,
	})
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// DeleteKnowledgeItem permanently removes id and every chunk it owns. This
// cannot be undone.
func (a *App) DeleteKnowledgeItem(id string) error {
	return a.knowledge.DeleteItem(a.ctx, id)
}

func toKnowledgeItemResult(item domainknowledge.Item) KnowledgeItemResult {
	return KnowledgeItemResult{
		ID: item.ID, Topic: item.Topic, Concept: item.Concept, Definition: item.Definition,
		Properties: item.Properties, TradeOffs: item.TradeOffs, RelatedConcepts: item.RelatedConcepts,
		Source: item.Source, Status: item.Status,
		CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano),
	}
}
