package knowledge

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

const (
	maxConceptChars    = 120
	maxDefinitionChars = 2000
	maxListItems       = 10
	maxListEntryChars  = 200
)

type extractionEnvelope struct {
	Items []extractedItem `json:"items"`
}

type extractedItem struct {
	Concept         string   `json:"concept"`
	Definition      string   `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"trade_offs"`
	RelatedConcepts []string `json:"related_concepts"`
}

func parseExtraction(raw, topic string, maxItems int, now time.Time) ([]domainknowledge.Item, error) {
	object, ok := extractJSONObject(raw)
	if !ok {
		return nil, ErrMalformedExtraction
	}
	var envelope extractionEnvelope
	if err := json.Unmarshal([]byte(object), &envelope); err != nil {
		return nil, ErrMalformedExtraction
	}
	envelope.Items = envelope.Items[:min(len(envelope.Items), maxItems)]

	items := make([]domainknowledge.Item, 0, len(envelope.Items))
	for _, candidate := range envelope.Items {
		item := domainknowledge.Item{
			ID:              uuid.NewString(),
			Topic:           strings.TrimSpace(topic),
			Concept:         truncateString(candidate.Concept, maxConceptChars),
			Definition:      truncateString(candidate.Definition, maxDefinitionChars),
			Properties:      normalizeList(candidate.Properties),
			TradeOffs:       normalizeList(candidate.TradeOffs),
			RelatedConcepts: normalizeList(candidate.RelatedConcepts),
			Source:          domainknowledge.SourceAthena,
			Status:          domainknowledge.StatusDraft,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if item.Validate() == nil {
			items = append(items, item)
		}
	}
	return items, nil
}

func extractJSONObject(raw string) (string, bool) {
	start := strings.Index(raw, "{")
	if start == -1 {
		return "", false
	}
	relativeEnd := strings.LastIndex(raw[start:], "}")
	if relativeEnd == -1 {
		return "", false
	}
	end := start + relativeEnd
	return raw[start : end+1], true
}

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
