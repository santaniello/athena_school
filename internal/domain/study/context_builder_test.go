package study

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputReserve_branches(t *testing.T) {
	cases := []struct {
		name   string
		window int
		want   int
	}{
		{"tiny window is capped to a quarter", 4000, 1000},
		{"quarter just below the floor wins", 8188, 2047},
		{"quarter exactly equals the floor", 8192, 2048},
		{"quarter just above the floor loses to it", 8196, 2048},
		{"tenth below the floor uses the floor", 16_000, 2048},
		{"window where tenth equals the floor", 20_480, 2048},
		{"window where tenth passes the floor", 20_490, 2049},
		{"mid window uses a tenth", 128_000, 12_800},
		{"tenth exactly at the ceiling", 163_840, 16_384},
		{"tenth above the ceiling is clamped", 200_000, 16_384},
		{"huge window is clamped", 1_000_000, 16_384},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given a context window
			// When computing the output reserve
			got := OutputReserve(c.window)

			// Then it follows min(max(window/10, 2048), 16384, window/4)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestNeedsCompaction_boundaryAtEightyPercent(t *testing.T) {
	cases := []struct {
		name      string
		estimated int
		window    int
		want      bool
	}{
		{"just below 80%", 799, 1000, false},
		{"exactly 80%", 800, 1000, true},
		{"above 80%", 801, 1000, true},
		{"non-round window just below", 75, 95, false},
		{"non-round window exactly at", 76, 95, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given an estimated prompt size and a window
			// When asking whether compaction is needed
			got := NeedsCompaction(c.estimated, c.window)

			// Then the 80% threshold is inclusive
			assert.Equal(t, c.want, got)
		})
	}
}

func TestCompactionTarget_isHalfTheWindow(t *testing.T) {
	// Given a window
	// When computing the compaction target
	// Then it is half of it
	assert.Equal(t, 50_000, CompactionTarget(100_000))
}

// messagesOfTokens builds messages whose EstimateTokens equals the given
// sizes (each EstimateTokens is (runes+2)/3 + 8).
func messagesOfTokens(sizes ...int) []Message {
	messages := make([]Message, 0, len(sizes))
	for i, size := range sizes {
		runes := (size - 8) * 3
		messages = append(messages, Message{
			ID:      string(rune('a' + i)),
			Content: strings.Repeat("x", runes),
		})
	}
	return messages
}

func TestMessagesOfTokens_helperMatchesEstimateTokens(t *testing.T) {
	// Given messages built for exact token sizes
	messages := messagesOfTokens(108, 308)

	// When estimating them
	// Then the sizes match
	assert.Equal(t, 108, EstimateTokens(messages[0].Content))
	assert.Equal(t, 308, EstimateTokens(messages[1].Content))
}

func TestBuildContext_nothingToFoldWhenPromptIsAtOrBelowTarget(t *testing.T) {
	// Given a 1000-token window (target 500) and a prompt of exactly 500
	input := BuildInput{
		ContextLength: 1000,
		FixedTokens:   100,
		PendingTokens: 100,
		Messages:      messagesOfTokens(150, 150),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then nothing is folded and the total is reported
	require.NoError(t, err)
	assert.Equal(t, 0, plan.FoldCount)
	assert.Equal(t, 500, plan.EstimatedTokens)
}

func TestBuildContext_foldsTheSmallestPrefixThatReachesTheTarget(t *testing.T) {
	// Given a 100,000 window (target 50,000) and 80,400 tokens in total:
	// four 20,000-token messages, three 100-token ones and a 100-token pending
	input := BuildInput{
		ContextLength: 100_000,
		PendingTokens: 100,
		Messages:      messagesOfTokens(20_000, 20_000, 20_000, 20_000, 100, 100, 100),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then folding one message is not enough (4,000 projected summary +
	// 60,400 = 64,400) but folding two is (44,400 <= 50,000)
	require.NoError(t, err)
	assert.Equal(t, 2, plan.FoldCount)
	assert.Equal(t, 44_400, plan.EstimatedTokens)
	assert.Equal(t, 80_400, plan.UnfoldedTokens)
}

func TestPromptBudget_isTheWindowMinusTheOutputReserve(t *testing.T) {
	// Given a 100,000 window whose reserve is 10,000
	// When computing the prompt budget
	// Then it is 90,000
	assert.Equal(t, 90_000, PromptBudget(100_000))
}

func TestBuildContext_existingLargerSummaryRaisesTheProjectedSummarySize(t *testing.T) {
	// Given an existing 4,500-token summary (above the 4,000 default target)
	// and enough history that folding is needed: 4,500 + 60,400 = 64,900
	input := BuildInput{
		ContextLength: 100_000,
		SummaryTokens: 4_500,
		PendingTokens: 100,
		Messages:      messagesOfTokens(20_000, 20_000, 20_000, 100, 100, 100),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then the projection keeps the larger existing size: folding one leaves
	// 4,500 + 40,000 + 300 + 100 = 44,900 (not the 4,000-based 44,400)
	require.NoError(t, err)
	assert.Equal(t, 1, plan.FoldCount)
	assert.Equal(t, 44_900, plan.EstimatedTokens)
}

func TestBuildContext_keepsTheMinimumRecentMessagesVerbatim(t *testing.T) {
	// Given a prompt above the target (60,100 > 50,000) but within the budget,
	// made only of the MinRecentMessages-1 protected persisted messages plus
	// the pending one
	input := BuildInput{
		ContextLength: 100_000,
		PendingTokens: 100,
		Messages:      messagesOfTokens(20_000, 20_000, 20_000),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then nothing is folded, because every message is in the protected tail
	require.NoError(t, err)
	assert.Equal(t, 0, plan.FoldCount)
	assert.Equal(t, 60_100, plan.EstimatedTokens)
}

func TestBuildContext_existingSummaryCountsTowardTheTotal(t *testing.T) {
	// Given an existing 3,000-token summary and little else
	input := BuildInput{
		ContextLength: 100_000,
		SummaryTokens: 3_000,
		FixedTokens:   200,
		PendingTokens: 100,
		Messages:      messagesOfTokens(100),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then the summary is part of the estimate
	require.NoError(t, err)
	assert.Equal(t, 0, plan.FoldCount)
	assert.Equal(t, 3_000+200+100+100, plan.EstimatedTokens)
}

func TestBuildContext_foldsAsMuchAsAllowedWhenTargetIsUnreachableButBudgetFits(t *testing.T) {
	// Given a 20,000 window (reserve 2,048 → budget 17,952, target 10,000)
	// whose protected tail alone (3 x 3,000 + pending 1,000 + fixed 1,000
	// + projected summary 4,000 = 15,000) is above the target but within budget
	input := BuildInput{
		ContextLength: 20_000,
		FixedTokens:   1_000,
		PendingTokens: 1_000,
		Messages:      messagesOfTokens(9_000, 3_000, 3_000, 3_000),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then the only foldable message is folded and the result is accepted
	require.NoError(t, err)
	assert.Equal(t, 1, plan.FoldCount)
	assert.Equal(t, 15_000, plan.EstimatedTokens)
}

func TestBuildContext_doesNotFoldWhenTheProjectedSummaryWouldNotShrinkThePrompt(t *testing.T) {
	// Given a prompt above the target made of a big pending message and tiny
	// history: folding the single foldable 8-token message would add a
	// 4,000-token summary and make the prompt bigger (10,024 vs 6,032)
	input := BuildInput{
		ContextLength: 10_000,
		PendingTokens: 6_000,
		Messages:      messagesOfTokens(8, 8, 8, 8),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then nothing is folded and the current size is reported
	require.NoError(t, err)
	assert.Equal(t, 0, plan.FoldCount)
	assert.Equal(t, 6_032, plan.EstimatedTokens)
}

func TestBuildContext_budgetImpossibleWhenEvenTheFoldedPromptExceedsTheBudget(t *testing.T) {
	// Given a 10,000 window (budget 7,952) where folding helps (17,000 →
	// 15,000) but still does not fit
	input := BuildInput{
		ContextLength: 10_000,
		FixedTokens:   1_000,
		PendingTokens: 1_000,
		Messages:      messagesOfTokens(6_000, 3_000, 3_000, 3_000),
	}

	// When building the context
	_, err := BuildContext(input)

	// Then it fails with ErrBudgetImpossible, without looping
	require.ErrorIs(t, err, ErrBudgetImpossible)
}

func TestBuildContext_budgetImpossibleWhenNothingCanBeFolded(t *testing.T) {
	// Given a protected tail that already exceeds the budget (11,500 > 7,952)
	input := BuildInput{
		ContextLength: 10_000,
		FixedTokens:   1_000,
		PendingTokens: 1_000,
		Messages:      messagesOfTokens(500, 3_000, 3_000, 3_000),
	}

	// When building the context
	_, err := BuildContext(input)

	// Then it fails with ErrBudgetImpossible
	require.ErrorIs(t, err, ErrBudgetImpossible)
}

func TestBuildContext_budgetBoundaryIsInclusive(t *testing.T) {
	// Given a 10,000 window (budget 7,952) and an unfoldable tail whose total
	// is exactly the budget: 100 pending + 7,836 + 8 + 8
	input := BuildInput{
		ContextLength: 10_000,
		PendingTokens: 100,
		Messages:      messagesOfTokens(7_836, 8, 8),
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then a total of exactly the budget is accepted
	require.NoError(t, err)
	assert.Equal(t, 7_952, plan.EstimatedTokens)

	// And one token more is rejected
	input.PendingTokens = 101
	_, err = BuildContext(input)
	require.ErrorIs(t, err, ErrBudgetImpossible)
}

func TestBuildContext_pendingMessageThatCannotFitAloneIsTooLarge(t *testing.T) {
	// Given fixed + pending one token over the budget (1,000 + 6,953 > 7,952)
	input := BuildInput{
		ContextLength: 10_000,
		FixedTokens:   1_000,
		PendingTokens: 6_953,
		Messages:      messagesOfTokens(100),
	}

	// When building the context
	_, err := BuildContext(input)

	// Then it is reported as too large, not as an impossible budget
	require.ErrorIs(t, err, ErrMessageTooLarge)
	assert.NotErrorIs(t, err, ErrBudgetImpossible)
}

func TestBuildContext_pendingMessageExactlyFittingAloneIsNotTooLarge(t *testing.T) {
	// Given fixed + pending exactly equal to the budget
	input := BuildInput{
		ContextLength: 10_000,
		FixedTokens:   1_000,
		PendingTokens: 6_952,
	}

	// When building the context
	plan, err := BuildContext(input)

	// Then it is accepted
	require.NoError(t, err)
	assert.Equal(t, 7_952, plan.EstimatedTokens)
}

func TestContextUsageAfterCompaction_lowersBlockedAndWarningToNormal(t *testing.T) {
	cases := []struct {
		name     string
		previous ContextState
	}{
		{"blocked", ContextStateBlocked},
		{"warning", ContextStateWarning},
		{"normal", ContextStateNormal},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given a session with the same model and window, in some state
			previous := ContextUsage{State: c.previous, Model: "m", UsedTokens: 99_000, ContextLength: 100_000}

			// When the compacted prompt is measured at 40,000 tokens
			got := ContextUsageAfterCompaction(previous, 40_000, 100_000)

			// Then the state is recomputed from the new measurement
			assert.Equal(t, ContextUsage{
				State: ContextStateNormal, Model: "m", UsedTokens: 40_000, ContextLength: 100_000, Estimated: true,
			}, got)
		})
	}
}

func TestContextUsageAfterCompaction_stillHighKeepsComputedState(t *testing.T) {
	// Given a compacted prompt that is still at 85% of the window
	previous := ContextUsage{State: ContextStateBlocked, Model: "m", ContextLength: 100_000}

	// When recomputing
	got := ContextUsageAfterCompaction(previous, 85_000, 100_000)

	// Then it is a warning, not normal
	assert.Equal(t, ContextStateWarning, got.State)
}

func TestContextUsageAfterCompaction_unresolvedWindowPreservesState(t *testing.T) {
	// Given an unresolved window
	previous := ContextUsage{State: ContextStateWarning, Model: "m"}

	// When recomputing
	got := ContextUsageAfterCompaction(previous, 500, 0)

	// Then the state is preserved and only the measurement updates
	assert.Equal(t, ContextStateWarning, got.State)
	assert.Equal(t, 500, got.UsedTokens)
	assert.Equal(t, 0, got.ContextLength)
}
