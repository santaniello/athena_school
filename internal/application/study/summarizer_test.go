package study

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
)

var summaryTestNow = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

// scriptedChat answers each Chat call with the next scripted result and
// records every request.
type scriptedChat struct {
	requests []domainllm.ChatRequest
	replies  []chatReply
}

type chatReply struct {
	content string
	err     error
}

func (c *scriptedChat) respond(_ context.Context, req domainllm.ChatRequest) (domainllm.ChatResponse, error) {
	c.requests = append(c.requests, req)
	reply := c.replies[len(c.requests)-1]
	return domainllm.ChatResponse{Content: reply.content}, reply.err
}

func newTestSummarizer(t *testing.T, language string, chat *scriptedChat) *Summarizer {
	t.Helper()
	provider := llmmocks.NewMockProvider(t)
	provider.EXPECT().Chat(context.Background(), mock.AnythingOfType("llm.ChatRequest")).RunAndReturn(chat.respond).Maybe()
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{AssistantLanguage: language}, nil).Maybe()
	s := NewSummarizer(provider, profiles)
	s.now = func() time.Time { return summaryTestNow }
	return s
}

func msg(id, role, content string) domainstudy.Message {
	return domainstudy.Message{ID: id, SessionID: "s1", Role: role, Content: content}
}

func systemOf(req domainllm.ChatRequest) string { return req.Messages[0].Content }
func userOf(req domainllm.ChatRequest) string   { return req.Messages[len(req.Messages)-1].Content }

func TestSummarizer_Summarize_foldsPreviousSummaryAndNewMessages(t *testing.T) {
	// Given a previous summary and two new messages
	chat := &scriptedChat{replies: []chatReply{{content: "  ## Progress\n- pointers\n  "}}}
	s := newTestSummarizer(t, domainprofile.AssistantLanguageEnglish, chat)
	previous := &domainstudy.ConversationSummary{SessionID: "s1", Content: "PREVIOUS-SUMMARY", CoversUntilMessageID: "m0"}
	messages := []domainstudy.Message{msg("m1", "user", "What is a pointer?"), msg("m2", "assistant", "An address.")}

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", previous, messages)

	// Then the result covers up to the last message, with a trimmed body
	require.NoError(t, err)
	assert.Equal(t, domainstudy.ConversationSummary{
		SessionID:            "s1",
		Content:              "## Progress\n- pointers",
		CoversUntilMessageID: "m2",
		TokenEstimate:        domainstudy.EstimateTokens("## Progress\n- pointers"),
		UpdatedAt:            summaryTestNow,
	}, got)

	// And exactly one cheap-tier call attributed to the session carried everything
	require.Len(t, chat.requests, 1)
	req := chat.requests[0]
	assert.Equal(t, "s1", req.SessionID)
	assert.Equal(t, domainllm.TaskConversationSummary, req.Task)
	for _, section := range []string{"## Progress", "## Open difficulties", "## Pending", "## Discussion log"} {
		assert.Contains(t, systemOf(req), section)
	}
	assert.Contains(t, userOf(req), "PREVIOUS-SUMMARY")
	assert.Contains(t, userOf(req), "What is a pointer?")
	assert.Contains(t, userOf(req), "An address.")
	assert.NotContains(t, userOf(req), "m0")
}

func TestSummarizer_Summarize_marksEmptyPreviousSummary(t *testing.T) {
	// Given no previous summary
	chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
	s := newTestSummarizer(t, "", chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the prompt says there is none
	require.NoError(t, err)
	assert.Contains(t, userOf(chat.requests[0]), "(none)")
}

func TestSummarizer_Summarize_localizesTheSummaryLanguage(t *testing.T) {
	tests := []struct {
		language string
		want     string
	}{
		{domainprofile.AssistantLanguagePortuguese, "Brazilian Portuguese"},
		{domainprofile.AssistantLanguageEnglish, "English"},
	}
	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			// Given a profile with that assistant language
			chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
			s := newTestSummarizer(t, tt.language, chat)

			// When summarizing
			_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

			// Then the system prompt asks for that language, keeping section titles fixed
			require.NoError(t, err)
			assert.Contains(t, systemOf(chat.requests[0]), tt.want)
			assert.Contains(t, systemOf(chat.requests[0]), "## Open difficulties")
		})
	}
}

func TestSummarizer_Summarize_instructsAgainstVerbatimInstructionsAndOverclaiming(t *testing.T) {
	// Given any run
	chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
	s := newTestSummarizer(t, "", chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the guardrails from the spec are in the prompt
	require.NoError(t, err)
	system := systemOf(chat.requests[0])
	assert.Contains(t, system, "the user asked to")
	assert.Contains(t, system, "understood so far")
	assert.Contains(t, system, "Open difficulties")
}

func TestSummarizer_Summarize_returnsError_whenThereAreNoMessages(t *testing.T) {
	// Given an empty message list
	s := newTestSummarizer(t, "", &scriptedChat{})

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, nil)

	// Then
	assert.ErrorIs(t, err, ErrNoMessagesToSummarize)
}

func TestSummarizer_Summarize_returnsError_whenProfileCannotBeLoaded(t *testing.T) {
	// Given a failing profile store
	boom := errors.New("disk")
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{}, boom).Once()
	s := NewSummarizer(llmmocks.NewMockProvider(t), profiles)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then
	assert.ErrorIs(t, err, boom)
}

func TestSummarizer_Summarize_returnsError_whenTheModelFails(t *testing.T) {
	// Given a failing chat call
	boom := errors.New("provider down")
	chat := &scriptedChat{replies: []chatReply{{err: boom}}}
	s := newTestSummarizer(t, "", chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the cause is preserved and no summary is returned
	assert.ErrorIs(t, err, boom)
}

func TestSummarizer_Summarize_returnsError_whenTheModelReturnsBlank(t *testing.T) {
	for _, blank := range []string{"", " \n\t "} {
		// Given a blank response
		chat := &scriptedChat{replies: []chatReply{{content: blank}}}
		s := newTestSummarizer(t, "", chat)

		// When summarizing
		got, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

		// Then
		require.Error(t, err)
		assert.Equal(t, domainstudy.ConversationSummary{}, got)
	}
}

// lineCost is the token estimate of one rendered message line.
func lineCost(m domainstudy.Message) int { return domainstudy.EstimateTokens(renderMessage(m)) }

func threeEqualMessages() []domainstudy.Message {
	return []domainstudy.Message{
		msg("m1", "user", strings.Repeat("a", 90)),
		msg("m2", "assistant", strings.Repeat("b", 90)),
		msg("m3", "user", strings.Repeat("c", 90)),
	}
}

func TestSummarizer_Summarize_foldsInSequentialBatches_whenInputExceedsTheBudget(t *testing.T) {
	// Given a budget that fits exactly two of three messages
	messages := threeEqualMessages()
	chat := &scriptedChat{replies: []chatReply{{content: "after-batch-1"}, {content: "final"}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = lineCost(messages[0]) + lineCost(messages[1])

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, messages)

	// Then two calls run, the first output feeding the second
	require.NoError(t, err)
	require.Len(t, chat.requests, 2)
	assert.Contains(t, userOf(chat.requests[0]), strings.Repeat("a", 90))
	assert.Contains(t, userOf(chat.requests[0]), strings.Repeat("b", 90))
	assert.NotContains(t, userOf(chat.requests[0]), strings.Repeat("c", 90))
	assert.Contains(t, userOf(chat.requests[1]), "after-batch-1")
	assert.Contains(t, userOf(chat.requests[1]), strings.Repeat("c", 90))
	assert.NotContains(t, userOf(chat.requests[1]), strings.Repeat("a", 90))
	assert.Equal(t, "final", got.Content)
	assert.Equal(t, "m3", got.CoversUntilMessageID)
}

func TestSummarizer_Summarize_splitsOneMessagePerBatch_whenBudgetIsOneTokenShort(t *testing.T) {
	// Given a budget one token short of fitting two messages
	messages := threeEqualMessages()
	chat := &scriptedChat{replies: []chatReply{{content: "1"}, {content: "2"}, {content: "3"}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = lineCost(messages[0]) + lineCost(messages[1]) - 1

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, messages)

	// Then every message gets its own call
	require.NoError(t, err)
	assert.Len(t, chat.requests, 3)
}

func TestSummarizer_Summarize_keepsEveryCallWithinTheInputBudget(t *testing.T) {
	// Given many messages and a small budget
	var messages []domainstudy.Message
	for i := range 20 {
		messages = append(messages, msg(string(rune('a'+i)), "user", strings.Repeat("x", 60+i)))
	}
	replies := make([]chatReply, 20)
	for i := range replies {
		replies[i] = chatReply{content: "s"}
	}
	chat := &scriptedChat{replies: replies}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = 80

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, messages)

	// Then no call carries more conversation text than the budget
	require.NoError(t, err)
	for _, req := range chat.requests {
		assert.LessOrEqual(t, domainstudy.EstimateTokens(conversationOf(userOf(req))), s.inputBudget)
	}
}

func TestSummarizer_Summarize_truncatesAMessageLargerThanTheBudgetForThePromptOnly(t *testing.T) {
	// Given one message far larger than the budget
	huge := msg("m1", "user", strings.Repeat("Q", 5_000))
	chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = 100

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{huge})

	// Then the prompt carries a truncated copy with a marker, within budget
	require.NoError(t, err)
	body := conversationOf(userOf(chat.requests[0]))
	assert.Contains(t, body, truncationMarker)
	assert.Equal(t, 3*(100-8)-len([]rune("user: "+truncationMarker))-1, strings.Count(body, "Q"))
	assert.Equal(t, 100, domainstudy.EstimateTokens(body))
	assert.Len(t, huge.Content, 5_000)
}

func TestSummarizer_Summarize_stopsAndReturnsError_whenALaterBatchFails(t *testing.T) {
	// Given the second of two batches fails
	boom := errors.New("rate limited")
	messages := threeEqualMessages()
	chat := &scriptedChat{replies: []chatReply{{content: "partial"}, {err: boom}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = lineCost(messages[0]) + lineCost(messages[1])

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, messages)

	// Then nothing usable is returned
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, domainstudy.ConversationSummary{}, got)
}

func smallCapSummarizer(t *testing.T, chat *scriptedChat) *Summarizer {
	s := newTestSummarizer(t, "", chat)
	s.targetTokens = 100 // hard cap 125
	return s
}

func TestSummarizer_Summarize_acceptsASummaryExactlyAtTheHardCap(t *testing.T) {
	// Given a summary whose estimate equals the hard cap (125)
	atCap := strings.Repeat("a", 351)
	require.Equal(t, 125, domainstudy.EstimateTokens(atCap))
	chat := &scriptedChat{replies: []chatReply{{content: atCap}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then it is accepted without a condensing pass
	require.NoError(t, err)
	assert.Len(t, chat.requests, 1)
	assert.Equal(t, atCap, got.Content)
}

func TestSummarizer_Summarize_condensesOnce_whenTheSummaryIsOverTheHardCap(t *testing.T) {
	// Given a summary one token over the hard cap, and a condensed answer
	overCap := strings.Repeat("a", 352)
	require.Equal(t, 126, domainstudy.EstimateTokens(overCap))
	chat := &scriptedChat{replies: []chatReply{{content: overCap}, {content: "condensed"}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then the second call condenses the summary alone, still attributed to the session
	require.NoError(t, err)
	require.Len(t, chat.requests, 2)
	condense := chat.requests[1]
	assert.Equal(t, "s1", condense.SessionID)
	assert.Equal(t, domainllm.TaskConversationSummary, condense.Task)
	assert.Contains(t, userOf(condense), overCap)
	assert.NotContains(t, userOf(condense), "<conversation>")
	assert.Contains(t, systemOf(condense), "Discussion log")
	assert.Equal(t, "condensed", got.Content)
	assert.Equal(t, domainstudy.EstimateTokens("condensed"), got.TokenEstimate)
}

func TestSummarizer_Summarize_returnsErrSummaryTooLarge_whenCondensingIsNotEnough(t *testing.T) {
	// Given a model that cannot shrink the summary
	overCap := strings.Repeat("a", 400)
	chat := &scriptedChat{replies: []chatReply{{content: overCap}, {content: overCap}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then it fails after exactly one extra pass
	assert.ErrorIs(t, err, ErrSummaryTooLarge)
	assert.Len(t, chat.requests, 2)
	assert.Equal(t, domainstudy.ConversationSummary{}, got)
}

func TestSummarizer_Summarize_returnsError_whenCondensingFails(t *testing.T) {
	// Given the condensing call fails
	boom := errors.New("timeout")
	chat := &scriptedChat{replies: []chatReply{{content: strings.Repeat("a", 400)}, {err: boom}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then
	assert.ErrorIs(t, err, boom)
}

func TestSummarizer_Summarize_returnsError_whenCondensingReturnsBlank(t *testing.T) {
	// Given the condensing call returns blank
	chat := &scriptedChat{replies: []chatReply{{content: strings.Repeat("a", 400)}, {content: "  "}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrSummaryTooLarge)
}

func TestSummarizer_Summarize_keepsTokenEstimateUnderTheCap_acrossManyIncrementalRuns(t *testing.T) {
	// Given a synthetic model that appends to the summary on every fold and
	// can condense when asked
	s := newTestSummarizer(t, "", &scriptedChat{})
	provider := llmmocks.NewMockProvider(t)
	provider.EXPECT().Chat(context.Background(), mock.AnythingOfType("llm.ChatRequest")).RunAndReturn(
		func(_ context.Context, req domainllm.ChatRequest) (domainllm.ChatResponse, error) {
			if !strings.Contains(userOf(req), "<conversation>") {
				return domainllm.ChatResponse{Content: userOf(req)[:100]}, nil
			}
			previous := strings.TrimSpace(strings.SplitN(strings.SplitN(userOf(req), "<conversation>", 2)[0], "Previous summary:", 2)[1])
			if previous == "(none)" {
				previous = ""
			}
			return domainllm.ChatResponse{Content: previous + "\n- discussed one more topic in some detail"}, nil
		},
	).Maybe()
	s.llm = provider
	s.targetTokens = 100
	hardCap := domainstudy.SummaryHardCapTokens(100)

	var previous *domainstudy.ConversationSummary
	for i := range 40 {
		// When folding one more message each time
		got, err := s.Summarize(context.Background(), "s1", previous, []domainstudy.Message{msg(string(rune('A'+i)), "user", "more")})

		// Then the stored estimate never exceeds the hard cap
		require.NoError(t, err)
		require.LessOrEqual(t, got.TokenEstimate, hardCap)
		previous = &got
	}
}

// conversationOf extracts the transcript between the <conversation> tags of
// a fold request's user message.
func conversationOf(user string) string {
	_, rest, _ := strings.Cut(user, "<conversation>\n")
	body, _, _ := strings.Cut(rest, "\n</conversation>")
	return body
}

func TestSummarizer_Summarize_doesNotTruncateAMessageThatExactlyFitsTheBudget(t *testing.T) {
	// Given a message whose line costs exactly the budget
	m := msg("m1", "user", strings.Repeat("y", 90))
	chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = lineCost(m)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{m})

	// Then it is sent whole
	require.NoError(t, err)
	body := conversationOf(userOf(chat.requests[0]))
	assert.NotContains(t, body, truncationMarker)
	assert.Equal(t, 90, strings.Count(body, "y"))
}

func TestSummarizer_Summarize_survivesABudgetSmallerThanTheTruncationMarker(t *testing.T) {
	// Given a budget too small to hold any content next to the marker
	chat := &scriptedChat{replies: []chatReply{{content: "ok"}}}
	s := newTestSummarizer(t, "", chat)
	s.inputBudget = 10

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", strings.Repeat("q", 500))})

	// Then the content is dropped to nothing instead of panicking
	require.NoError(t, err)
	assert.Equal(t, "user: "+truncationMarker, conversationOf(userOf(chat.requests[0])))
}

func TestSummarizer_Summarize_acceptsACondensedSummaryExactlyAtTheHardCap(t *testing.T) {
	// Given a condensing pass that lands exactly on the hard cap (125)
	atCap := strings.Repeat("a", 351)
	chat := &scriptedChat{replies: []chatReply{{content: strings.Repeat("a", 400)}, {content: atCap}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	got, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then it is accepted
	require.NoError(t, err)
	assert.Equal(t, atCap, got.Content)
}

func TestSummarizer_Summarize_rejectsACondensedSummaryOneTokenOverTheHardCap(t *testing.T) {
	// Given a condensing pass that lands one token over (126)
	chat := &scriptedChat{replies: []chatReply{{content: strings.Repeat("a", 400)}, {content: strings.Repeat("a", 352)}}}
	s := smallCapSummarizer(t, chat)

	// When summarizing
	_, err := s.Summarize(context.Background(), "s1", nil, []domainstudy.Message{msg("m1", "user", "hi")})

	// Then
	assert.ErrorIs(t, err, ErrSummaryTooLarge)
}
