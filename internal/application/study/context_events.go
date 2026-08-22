package study

import (
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ContextEvent is delivered whenever a session's ContextUsage transitions
// to a new state (or its context length becomes known for the first time).
// It never carries an error: a context notification can never roll back
// persistence or fail a chat response.
type ContextEvent struct {
	State         domainstudy.ContextState
	UsedTokens    int
	ContextLength int
	Estimated     bool
}

// ContextCallback receives ContextEvent transitions. Typed and
// non-error-returning, following the existing source/chunk callback
// boundary (see send_message.go's onSources/onChunk) but, unlike those,
// unable to abort the turn.
type ContextCallback func(ContextEvent)

// ContextUnavailableCallback receives the transient "could not determine
// this session's context limit" technical notice. Not persisted context
// state — see stream.go and resume.go.
type ContextUnavailableCallback func(message string)

// emitContextTransition calls onContext when next's state differs from
// previous's, or when next's context length became known for the first
// time (previous.ContextLength == 0, next.ContextLength > 0) even if the
// state itself stayed the same.
func emitContextTransition(previous, next domainstudy.ContextUsage, onContext ContextCallback) {
	if onContext == nil {
		return
	}
	stateChanged := next.State != previous.State
	lengthNewlyKnown := previous.ContextLength == 0 && next.ContextLength > 0
	if !stateChanged && !lengthNewlyKnown {
		return
	}
	onContext(ContextEvent{
		State:         next.State,
		UsedTokens:    next.UsedTokens,
		ContextLength: next.ContextLength,
		Estimated:     next.Estimated,
	})
}

// nativeUsageUsable reports whether a provider's native Usage is trustworthy
// enough to use directly: both fields non-negative, their sum does not
// overflow, and the sum is positive.
func nativeUsageUsable(usage domainllm.Usage) (sum int, ok bool) {
	if usage.InputTokens < 0 || usage.OutputTokens < 0 {
		return 0, false
	}
	sum = usage.InputTokens + usage.OutputTokens
	if sum < usage.InputTokens || sum < usage.OutputTokens {
		return 0, false // overflow wrapped the sum
	}
	if sum <= 0 {
		return 0, false
	}
	return sum, true
}

// measureUsage returns the new complete occupancy for a completed stream
// call: native usage when trustworthy, otherwise the conservative estimate
// summed over every message actually sent (system prompt, any transient RAG
// message, full history) plus the assistant's response. Either way this is
// the full new occupancy, not an increment — see
// domainstudy.NextContextUsage.
func measureUsage(streamResp domainllm.StreamResponse, requestMessages []domainllm.Message, assistantContent string) (usedTokens int, estimated bool) {
	if sum, ok := nativeUsageUsable(streamResp.Usage); ok {
		return sum, false
	}

	total := 0
	for _, m := range requestMessages {
		total += domainstudy.EstimateTokens(m.Content)
	}
	total += domainstudy.EstimateTokens(assistantContent)
	return total, true
}
