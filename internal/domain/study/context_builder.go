package study

import "errors"

const (
	// CompactionTargetRatio is the fraction of the window a compacted prompt
	// aims for. Compacting down to half the window, rather than to just below
	// the 80% trigger, avoids recompacting every few turns (spec 2.25).
	CompactionTargetRatio = 0.5

	// MinRecentMessages is how many of the most recent messages, counting the
	// pending user message, are always sent verbatim.
	MinRecentMessages = 4

	minOutputReserveTokens = 2048
	maxOutputReserveTokens = 16384
)

var (
	// ErrBudgetImpossible is returned by BuildContext when even the summary,
	// the protected recent messages and the fixed messages do not fit in the
	// window minus the output reserve.
	ErrBudgetImpossible = errors.New("conversation does not fit the context window even when compacted")
	// ErrMessageTooLarge is returned by BuildContext when the pending user
	// message does not fit the budget by itself (with the fixed messages).
	ErrMessageTooLarge = errors.New("message is too large for the context window")
)

// OutputReserve returns the tokens of the window planned for the model's
// reply: min(max(window/10, 2048), 16384, window/4). It is a planning budget,
// not an enforced limit.
func OutputReserve(window int) int {
	return min(max(window/10, minOutputReserveTokens), maxOutputReserveTokens, window/4)
}

// PromptBudget returns the tokens a prompt may occupy: the window minus the
// output reserve.
func PromptBudget(window int) int {
	return window - OutputReserve(window)
}

// NeedsCompaction reports whether an estimated prompt size has reached the
// 80% warning threshold of the window (inclusive).
func NeedsCompaction(estimatedTokens, window int) bool {
	return ComputeContextState(estimatedTokens, window) != ContextStateNormal
}

// CompactionTarget returns the prompt size compaction aims for.
func CompactionTarget(window int) int {
	return int(float64(window) * CompactionTargetRatio)
}

// BuildInput describes the prompt to be built. Every size is an
// EstimateTokens-style estimate.
type BuildInput struct {
	// SummaryTokens is the size of the current summary, 0 when there is none.
	SummaryTokens int
	// Messages are the persisted messages after the summary cursor, oldest
	// first. They do not include the pending message.
	Messages []Message
	// PendingTokens is the size of the user message being sent; it is never
	// folded.
	PendingTokens int
	// FixedTokens is the size of the system and knowledge messages.
	FixedTokens int
	// ContextLength is the model window; it must be positive.
	ContextLength int
}

// ContextPlan is the result of BuildContext.
type ContextPlan struct {
	// UnfoldedTokens is the estimated size of the prompt as it stands, with
	// nothing folded.
	UnfoldedTokens int
	// FoldCount is how many of the oldest input Messages are folded into the
	// summary. Zero means nothing is folded.
	FoldCount int
	// EstimatedTokens is the estimated prompt size after folding, including
	// the projected size of the new summary.
	EstimatedTokens int
}

// BuildContext decides how many of the oldest messages to fold into the
// summary so the prompt reaches CompactionTarget. The most recent
// MinRecentMessages (counting the pending message) are always kept verbatim.
// If the target is unreachable it folds as much as allowed, provided that
// shrinks the prompt, and accepts the result as long as it fits the window
// minus OutputReserve. It returns ErrMessageTooLarge when the pending message
// does not fit by itself and ErrBudgetImpossible when the best achievable
// prompt does not fit; it never retries.
func BuildContext(input BuildInput) (ContextPlan, error) {
	budget := PromptBudget(input.ContextLength)
	target := CompactionTarget(input.ContextLength)

	if input.FixedTokens+input.PendingTokens > budget {
		return ContextPlan{}, ErrMessageTooLarge
	}

	maxFold := max(0, len(input.Messages)-(MinRecentMessages-1))
	projectedSummary := max(input.SummaryTokens, SummaryTargetTokens(SummaryModelWindowTokens))

	// suffix[k] is the token size of Messages[k:].
	suffix := make([]int, len(input.Messages)+1)
	for i := len(input.Messages) - 1; i >= 0; i-- {
		suffix[i] = suffix[i+1] + EstimateTokens(input.Messages[i].Content)
	}
	total := func(fold int) int {
		summary := input.SummaryTokens
		if fold > 0 {
			summary = projectedSummary
		}
		return summary + input.FixedTokens + input.PendingTokens + suffix[fold]
	}

	chosen := 0
	if total(0) > target {
		chosen = maxFold
		for fold := 1; fold <= maxFold; fold++ {
			if total(fold) <= target {
				chosen = fold
				break
			}
		}
		if total(chosen) >= total(0) {
			chosen = 0
		}
	}

	if total(chosen) > budget {
		return ContextPlan{}, ErrBudgetImpossible
	}
	return ContextPlan{UnfoldedTokens: total(0), FoldCount: chosen, EstimatedTokens: total(chosen)}, nil
}
