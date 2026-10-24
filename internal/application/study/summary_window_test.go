package study

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

func summaryModelResolver(model string) *domainllm.OverridableResolver {
	resolver := domainllm.NewOverridableResolver()
	resolver.SetOverrides(map[domainllm.TaskType]string{domainllm.TaskConversationSummary: model}, "")
	return resolver
}

func TestSummaryWindow_Tokens_isTheCatalogContextLengthOfTheConfiguredSummaryModel(t *testing.T) {
	// Given a summary model whose cached context length is 16,000
	contexts := llmmocks.NewMockModelContextResolver(t)
	contexts.EXPECT().CachedContextLength("small/model").Return(16_000, true).Once()
	window := NewSummaryWindow(summaryModelResolver("small/model"), contexts)

	// When asking for the summary window
	tokens := window.Tokens()

	// Then it is that length
	assert.Equal(t, 16_000, tokens)
}

func TestSummaryWindow_Tokens_fallsBackToTheDefaultWhenTheLengthIsUnknown(t *testing.T) {
	// Given a summary model the catalog cache does not know (yet)
	contexts := llmmocks.NewMockModelContextResolver(t)
	contexts.EXPECT().CachedContextLength("openai/gpt-4o-mini").Return(0, false).Once()
	window := NewSummaryWindow(domainllm.DefaultModelResolver{}, contexts)

	// When asking for the summary window
	// Then the 128k default applies
	assert.Equal(t, domainstudy.SummaryModelWindowTokens, window.Tokens())
}

func TestSummaryWindow_Tokens_ignoresANonPositiveCachedLength(t *testing.T) {
	// Given a cache hit that is not a usable length
	contexts := llmmocks.NewMockModelContextResolver(t)
	contexts.EXPECT().CachedContextLength("openai/gpt-4o-mini").Return(0, true).Once()
	window := NewSummaryWindow(domainllm.DefaultModelResolver{}, contexts)

	// When asking for the summary window
	// Then the default applies instead of a zero window
	assert.Equal(t, domainstudy.SummaryModelWindowTokens, window.Tokens())
}

func TestSummaryWindow_Tokens_followsAModelChangeOnTheNextCall(t *testing.T) {
	// Given a window source over a resolver the user can change
	resolver := summaryModelResolver("a/a")
	contexts := llmmocks.NewMockModelContextResolver(t)
	contexts.EXPECT().CachedContextLength("a/a").Return(32_000, true).Once()
	contexts.EXPECT().CachedContextLength("b/b").Return(1_000_000, true).Once()
	window := NewSummaryWindow(resolver, contexts)
	require.Equal(t, 32_000, window.Tokens())

	// When the user picks another summary model
	resolver.SetOverrides(map[domainllm.TaskType]string{domainllm.TaskConversationSummary: "b/b"}, "")

	// Then the next lookup uses the new model's length
	assert.Equal(t, 1_000_000, window.Tokens())
}

func TestSummarizer_withASmallSummaryModel_aimsForASmallerSummaryAndInput(t *testing.T) {
	// Given a summarizer whose summary model has an 8,000-token window
	chat := &scriptedChat{replies: []chatReply{{content: validSummary("x")}}}
	s := newTestSummarizer(t, domainprofile.AssistantLanguageEnglish, chat)
	contexts := llmmocks.NewMockModelContextResolver(t)
	contexts.EXPECT().CachedContextLength("small/model").Return(8_000, true)
	s.window = NewSummaryWindow(summaryModelResolver("small/model"), contexts)

	// When summarizing one message
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the prompt asks for the scaled target (10% of 8,000 clamped to the
	// 1,000 minimum) instead of the 4,000 of the default window
	require.NoError(t, err)
	require.Len(t, chat.requests, 1)
	assert.Contains(t, systemOf(chat.requests[0]), "under about 1000 tokens")
	assert.NotContains(t, systemOf(chat.requests[0]), "under about 4000 tokens")
}

func TestSummarizer_withoutAWindowSource_keepsTheDefaultTarget(t *testing.T) {
	// Given a summarizer built the old way, with no window source
	chat := &scriptedChat{replies: []chatReply{{content: validSummary("x")}}}
	s := newTestSummarizer(t, domainprofile.AssistantLanguageEnglish, chat)

	// When summarizing one message
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the default 128k-window target applies
	require.NoError(t, err)
	assert.Contains(t, systemOf(chat.requests[0]), "under about 4000 tokens")
}

type fixedSummaryWindow int

func (w fixedSummaryWindow) Tokens() int { return int(w) }

func TestCompactSession_withASmallSummaryModel_foldsFewerMessagesBecauseItsSummaryIsSmaller(t *testing.T) {
	// Given 54,000 tokens of history in a 100,000 window, where the default
	// summary projection (5,000) needs two messages folded to reach the
	// 50,000 target but a small summary model (1,250) only needs one
	f := newCompactionFixture(t)
	f.service.summaryWindow = fixedSummaryWindow(8_000)
	history := heavyHistory(6, 9000)
	summary := newSummaryOf("m1")
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 57_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:1]).Return(summary, nil).Once()
	f.summaries.EXPECT().Save(context.Background(), summary).Return(nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", domainstudy.ContextUsageAfterCompaction(
		compactionSession(domainstudy.ContextStateNormal, 57_000).Context,
		fixedCompactionTokens()+summary.TokenEstimate+tokensOf(history[1:]), compactionWindow)).Return(nil).Once()

	// When compacting explicitly
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then exactly one message is folded
	require.NoError(t, err)
}
