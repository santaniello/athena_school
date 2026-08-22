package study

import "unicode/utf8"

// ComputeContextState classifies usedTokens against contextLength using
// integer comparisons (usedTokens*100 vs contextLength*80/95) rather than
// floating-point percentages, so the 80%/95% boundaries are exact — no
// rounding can push a measurement to the wrong side. Equality enters the
// higher state. contextLength must be positive; callers with an
// unresolved (zero) context length must not call this — see
// NextContextUsage, which handles that case by preserving the previous
// state instead.
func ComputeContextState(usedTokens, contextLength int) ContextState {
	switch {
	case usedTokens*100 >= contextLength*95:
		return ContextStateBlocked
	case usedTokens*100 >= contextLength*80:
		return ContextStateWarning
	default:
		return ContextStateNormal
	}
}

// higherState returns whichever of a, b is closer to ContextStateBlocked.
func higherState(a, b ContextState) ContextState {
	rank := map[ContextState]int{ContextStateNormal: 0, ContextStateWarning: 1, ContextStateBlocked: 2}
	if rank[a] >= rank[b] {
		return a
	}
	return b
}

// NextContextUsage computes the ContextUsage that should replace previous
// after a new measurement (model, usedTokens, contextLength, estimated).
// usedTokens is the *complete* new occupancy (input+output for a real
// stream measurement, or the full conservative-estimate sum for a
// provisional/fallback one) — never an increment to add to previous.
//
//   - contextLength <= 0 (unresolved): the state cannot be judged against a
//     boundary, so previous.State is preserved unconditionally; Model,
//     UsedTokens and Estimated still update to the new measurement.
//   - contextLength > 0 and the model/contextLength are unchanged from
//     previous: the state is monotonic — it can only move toward
//     ContextStateBlocked, even if the freshly computed state would be
//     lower (e.g. a smaller real measurement replacing a larger estimate).
//   - contextLength > 0 and the model or contextLength changed: the state
//     is recomputed fresh and may move in either direction.
func NextContextUsage(previous ContextUsage, model string, usedTokens, contextLength int, estimated bool) ContextUsage {
	if contextLength <= 0 {
		return ContextUsage{
			State:         previous.State,
			Model:         model,
			UsedTokens:    usedTokens,
			ContextLength: 0,
			Estimated:     estimated,
		}
	}

	state := ComputeContextState(usedTokens, contextLength)
	if previous.Model == model && previous.ContextLength == contextLength {
		state = higherState(previous.State, state)
	}
	return ContextUsage{
		State:         state,
		Model:         model,
		UsedTokens:    usedTokens,
		ContextLength: contextLength,
		Estimated:     estimated,
	}
}

// EstimateTokens is the conservative per-message token estimate used
// whenever a turn has no usable native usage: ceil(code points / 3) + 8,
// computed with integer arithmetic (no floats). The +8 covers per-message
// role/structural overhead; the formula is language-agnostic (English,
// Portuguese, code and mixed-language content all use the same divisor).
func EstimateTokens(content string) int {
	runes := utf8.RuneCountInString(content)
	return (runes+2)/3 + 8
}
