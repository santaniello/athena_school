package llm

import (
	"context"
	"time"
)

// UsageEntry records the tokens and cost billed for a single LLM call.
type UsageEntry struct {
	ID           string
	SessionID    string
	Model        string
	InputTokens  int
	OutputTokens int
	Cost         float64
	CreatedAt    time.Time
}

// UsageRecorder persists a UsageEntry after each LLMProvider call.
type UsageRecorder interface {
	Record(ctx context.Context, entry UsageEntry) error
}
