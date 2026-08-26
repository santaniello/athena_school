package knowledge

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
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
	Concept         string                 `json:"concept"`
	Definition      string                 `json:"definition"`
	Properties      []string               `json:"properties"`
	TradeOffs       []string               `json:"trade_offs"`
	RelatedConcepts []string               `json:"related_concepts"`
	Evidence        []extractedEvidenceRef `json:"evidence"`
}

type extractedEvidenceRef struct {
	MessageID string `json:"message_id"`
	Quote     string `json:"quote"`
}

type parsedCandidate struct {
	domainknowledge.Item
	EvidenceRefs []domainknowledge.EvidenceRef
}

func parseExtraction(raw, topic string, maxItems int, now time.Time, includedMessages []domainstudy.Message) ([]parsedCandidate, error) {
	object, ok := extractJSONObject(raw)
	if !ok {
		return nil, ErrMalformedExtraction
	}
	var envelope extractionEnvelope
	if err := json.Unmarshal([]byte(object), &envelope); err != nil {
		return nil, ErrMalformedExtraction
	}
	envelope.Items = envelope.Items[:min(len(envelope.Items), maxItems)]

	normalizedTopic, topicErr := domainknowledge.NormalizeTopic(topic)
	if topicErr != nil {
		return []parsedCandidate{}, nil
	}

	messagesByID := make(map[string]domainstudy.Message, len(includedMessages))
	for _, message := range includedMessages {
		messagesByID[message.ID] = message
	}

	items := make([]parsedCandidate, 0, len(envelope.Items))
	for _, candidate := range envelope.Items {
		item := domainknowledge.Item{
			ID:              uuid.NewString(),
			Topic:           normalizedTopic,
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
		evidenceRefs := validateEvidenceRefs(candidate.Evidence, messagesByID)
		if item.Validate() == nil && len(evidenceRefs) > 0 {
			items = append(items, parsedCandidate{Item: item, EvidenceRefs: evidenceRefs})
		}
	}
	return items, nil
}

func validateEvidenceRefs(refs []extractedEvidenceRef, messagesByID map[string]domainstudy.Message) []domainknowledge.EvidenceRef {
	valid := make([]domainknowledge.EvidenceRef, 0, min(len(refs), maxEvidencePerItem))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		message, exists := messagesByID[ref.MessageID]
		quote := strings.TrimSpace(ref.Quote)
		key := ref.MessageID + "\x00" + quote
		_, duplicated := seen[key]
		if !exists || quote == "" || len([]rune(quote)) > maxEvidenceQuoteChars ||
			!strings.Contains(message.Content, quote) || duplicated {
			continue
		}
		seen[key] = struct{}{}
		valid = append(valid, domainknowledge.EvidenceRef{MessageID: ref.MessageID, Quote: quote})
		if len(valid) == maxEvidencePerItem {
			break
		}
	}
	return valid
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
