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

// UsageRecorder persists a UsageEntry after each Provider call.
type UsageRecorder interface {
	Record(ctx context.Context, entry UsageEntry) error
}

// UsageTotals aggregates every recorded call of one session. UnpricedCalls
// counts calls with tokens but a stored cost of 0: usage.cost is 0 both for
// free-model calls and for providers that omit cost, so Cost is only a
// lower bound when UnpricedCalls > 0. Calls is the number of recorded rows
// (0 means the session has no recorded usage).
type UsageTotals struct {
	InputTokens   int
	OutputTokens  int
	Cost          float64
	UnpricedCalls int
	Calls         int
}

// UsageReader reads the usage recorded by a UsageRecorder.
type UsageReader interface {
	SessionTotals(ctx context.Context, sessionID string) (UsageTotals, error)
}
