package study

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

const (
	compactionWindow = 100_000
	compactionTopic  = "Distributed systems"
	compactionPrompt = "What is CAP theorem?"
)

var compactionProfile = domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}

type compactionFixture struct {
	sessions   *studymocks.MockSessionRepository
	messages   *studymocks.MockMessageRepository
	summaries  *studymocks.MockConversationSummaryRepository
	summarizer *applicationstudymocks.MockConversationSummarizer
	llm        *llmmocks.MockProvider
	profiles   *profilemocks.MockStore
	retriever  *knowledgemocks.MockRetriever
	sources    *knowledgemocks.MockMessageSourceRepository
	service    *Service
}

func newCompactionFixture(t *testing.T) *compactionFixture {
	t.Helper()
	f := &compactionFixture{
		sessions:   studymocks.NewMockSessionRepository(t),
		messages:   studymocks.NewMockMessageRepository(t),
		summaries:  studymocks.NewMockConversationSummaryRepository(t),
		summarizer: applicationstudymocks.NewMockConversationSummarizer(t),
		llm:        llmmocks.NewMockProvider(t),
		profiles:   profilemocks.NewMockStore(t),
		retriever:  knowledgemocks.NewMockRetriever(t),
		sources:    knowledgemocks.NewMockMessageSourceRepository(t),
	}
	tx := applicationstudymocks.NewMockTransactor(t)
	tx.EXPECT().WithinTx(mock.Anything, mock.Anything).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).
		Maybe()
	f.profiles.EXPECT().Load().Return(compactionProfile, nil).Maybe()
	f.service = NewService(
		f.sessions, f.messages, f.llm, f.profiles, foldermocks.NewMockRepository(t), f.retriever,
		tx, nil, f.sources, nil, nil,
		WithSummarizer(f.summarizer), WithSummaries(f.summaries),
	)
	return f
}

func (f *compactionFixture) send(hooks CompactionHooks, content string) error {
	return f.service.SendMessage(
		context.Background(), "session-1", compactionTopic, content, domainknowledge.SourceModeNotes,
		noopSourcesHandler, noopChunkHandler, nil, nil, WithCompactionHooks(hooks),
	)
}

// expectTurnCompletes sets up everything that follows a successful prompt
// build: retrieval without hits, the user and assistant appends, the sources
// save and a stream that records its request.
func (f *compactionFixture) expectTurnCompletes(request *domainllm.ChatRequest) {
	f.messages.EXPECT().Append(context.Background(), mock.AnythingOfType("study.Message")).Return(nil).Twice()
	f.sources.EXPECT().Save(context.Background(), mock.AnythingOfType("string"), []domainknowledge.Source(nil)).Return(nil).Once()
	f.retriever.EXPECT().Retrieve(context.Background(), "session-1", mock.AnythingOfType("[]string")).
		Return(domainknowledge.RetrievalResult{}, nil).Once()
	f.llm.EXPECT().
		ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, req domainllm.ChatRequest, _ func(string) error) {
			if request != nil {
				*request = req
			}
		}).
		Return(domainllm.StreamResponse{}, nil).Once()
}

// expectReplyContexts sets the two context writes of a successful turn that
// carries no usage: the provisional one for the user message, then the one
// estimated from the prompt actually sent (system message, optional summary
// message, sent) plus the empty reply, with an unresolved window. prior is the
// context the session holds when the turn starts persisting.
func (f *compactionFixture) expectReplyContexts(prior domainstudy.ContextUsage, summary *domainstudy.ConversationSummary, sent []domainstudy.Message) {
	provisional := domainstudy.NextContextUsage(
		prior, prior.Model, prior.UsedTokens+domainstudy.EstimateTokens(compactionPrompt), prior.ContextLength, true)
	replyTokens := domainstudy.EstimateTokens(buildSystemPrompt(compactionProfile, compactionTopic, "")) +
		tokensOf(sent) + domainstudy.EstimateTokens("")
	if summary != nil {
		replyTokens += domainstudy.EstimateTokens(buildSummaryContext(summary.Content, domainknowledge.SourceModeNotes).Content)
	}
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", provisional).Return(nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", domainstudy.NextContextUsage(provisional, "", replyTokens, 0, true)).Return(nil).Once()
}

func compactionSession(state domainstudy.ContextState, used int) domainstudy.Session {
	return domainstudy.Session{
		ID: "session-1", Topic: compactionTopic,
		Context: domainstudy.ContextUsage{State: state, Model: "m", UsedTokens: used, ContextLength: compactionWindow},
	}
}

// tokenMessage builds a message of exactly the given EstimateTokens size.
func tokenMessage(id, role string, tokens int) domainstudy.Message {
	return domainstudy.Message{
		ID: id, SessionID: "session-1", Role: role,
		Content: strings.Repeat("x", (tokens-8)*3), CreatedAt: time.Unix(0, 0),
	}
}

// heavyHistory returns count alternating messages of the given size, m1..mN.
func heavyHistory(count, tokens int) []domainstudy.Message {
	history := make([]domainstudy.Message, 0, count)
	for i := 1; i <= count; i++ {
		role := domainstudy.RoleUser
		if i%2 == 0 {
			role = domainstudy.RoleAssistant
		}
		history = append(history, tokenMessage(fmt.Sprintf("m%d", i), role, tokens))
	}
	return history
}

func fixedCompactionTokens() int {
	return domainstudy.EstimateTokens(buildSystemPrompt(compactionProfile, compactionTopic, "")) + knowledgeContextAllowanceTokens
}

func newSummaryOf(coversUntil string) domainstudy.ConversationSummary {
	return domainstudy.ConversationSummary{
		SessionID: "session-1", Content: "summary text", CoversUntilMessageID: coversUntil, TokenEstimate: 4000,
	}
}

func TestSendMessage_compaction_foldsOldestMessages_andSendsSummaryBeforeRecentMessages(t *testing.T) {
	// A 100,000 window holding ten 9,000-token messages is above the 80%
	// trigger whatever the persisted state was; in the blocked case the
	// success must also unblock the turn.
	for _, state := range []domainstudy.ContextState{
		domainstudy.ContextStateNormal, domainstudy.ContextStateWarning, domainstudy.ContextStateBlocked,
	} {
		t.Run(string(state), func(t *testing.T) {
			// Given a session in this state whose next prompt needs compaction
			f := newCompactionFixture(t)
			history := heavyHistory(10, 9000)
			summary := newSummaryOf("m6")
			resetUsage := domainstudy.ContextUsage{
				State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
				UsedTokens: fixedCompactionTokens() + summary.TokenEstimate + tokensOf(history[6:]),
			}
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(state, 96_000), nil).Once()
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(
				domainstudy.Session{ID: "session-1", Topic: compactionTopic, Context: resetUsage}, nil).Once()
			f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
			f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:6]).Return(summary, nil).Once()
			f.summaries.EXPECT().Save(context.Background(), summary).Return(nil).Once()
			f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", resetUsage).Return(nil).Once()
			tail := append(append([]domainstudy.Message{}, history[6:]...), tokenMessage("m11", domainstudy.RoleUser, 20))
			f.expectReplyContexts(resetUsage, &summary, tail)
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "m6").Return(tail, nil).Once()
			var request domainllm.ChatRequest
			f.expectTurnCompletes(&request)

			started := 0
			var compacted []ContextEvent
			hooks := CompactionHooks{
				OnStarted:   func() { started++ },
				OnCompacted: func(e ContextEvent) { compacted = append(compacted, e) },
				OnFailed:    func(err error) { t.Errorf("unexpected failure: %v", err) },
			}

			// When sending a message
			err := f.send(hooks, compactionPrompt)

			// Then the turn succeeds and the prompt is
			// [system, summary, recent messages] with no folded message
			require.NoError(t, err)
			require.Len(t, request.Messages, 2+len(tail))
			assert.Equal(t, "system", request.Messages[0].Role)
			assert.Equal(t, "system", request.Messages[1].Role)
			assert.Contains(t, request.Messages[1].Content, "summary text")
			assert.Equal(t, tail[0].Content, request.Messages[2].Content)
			assert.Equal(t, 1, started)
			assert.Equal(t, []ContextEvent{{
				State: domainstudy.ContextStateNormal, UsedTokens: resetUsage.UsedTokens,
				ContextLength: compactionWindow, Estimated: true,
			}}, compacted)
		})
	}
}

func TestSendMessage_compaction_blockedSessionWhoseSummarizerFails_keepsTheLimitGuarantees(t *testing.T) {
	// Given a blocked session whose summarizer fails; no other expectation is
	// set, so persisting, retrieval or an LLM call would fail the test
	f := newCompactionFixture(t)
	history := heavyHistory(10, 9000)
	blocked := compactionSession(domainstudy.ContextStateBlocked, 96_000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(blocked, nil).Twice()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:6]).
		Return(domainstudy.ConversationSummary{}, ErrInvalidSummaryFormat).Once()
	var failures []error
	hooks := CompactionHooks{OnFailed: func(err error) { failures = append(failures, err) }}

	// When sending a message
	err := f.send(hooks, compactionPrompt)

	// Then the turn is rejected as in spec 2.6 and the failure was reported
	require.ErrorIs(t, err, domainstudy.ErrSessionContextLimitReached)
	require.Len(t, failures, 1)
	assert.ErrorIs(t, failures[0], ErrInvalidSummaryFormat)
}

func TestSendMessage_compaction_warningSessionWhoseSummarizerFails_proceedsWithTheFullHistory(t *testing.T) {
	// Given a warning session whose 72,000-token history fits the window
	f := newCompactionFixture(t)
	history := heavyHistory(8, 9000)
	warning := compactionSession(domainstudy.ContextStateWarning, 85_000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(warning, nil).Twice()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:4]).
		Return(domainstudy.ConversationSummary{}, ErrSummaryTooLarge).Once()
	f.expectReplyContexts(warning.Context, nil, history)
	f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	var request domainllm.ChatRequest
	f.expectTurnCompletes(&request)
	failed := 0

	// When sending a message
	err := f.send(CompactionHooks{OnFailed: func(error) { failed++ }}, compactionPrompt)

	// Then the failure is reported but the turn goes on with the full history
	require.NoError(t, err)
	assert.Equal(t, 1, failed)
	assert.Len(t, request.Messages, 1+len(history))
}

func TestSendMessage_compaction_warningSessionWhoseSummarizerFailsAndPromptExceedsTheWindow_failsBeforePersisting(t *testing.T) {
	// Given a warning session whose 90,000-token history plus the fixed
	// messages exceeds window - reserve, and a failing summarizer; no
	// Append/Retrieve/ChatStream expectation is set
	f := newCompactionFixture(t)
	history := heavyHistory(10, 9000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:6]).
		Return(domainstudy.ConversationSummary{}, errors.New("provider down")).Once()
	failed := 0

	// When sending a message
	err := f.send(CompactionHooks{OnFailed: func(error) { failed++ }}, compactionPrompt)

	// Then it is rejected with ErrBudgetImpossible
	require.ErrorIs(t, err, domainstudy.ErrBudgetImpossible)
	assert.Equal(t, 1, failed)
}

func TestSendMessage_compaction_impossibleBudget_dependsOnTheSessionState(t *testing.T) {
	cases := []struct {
		name  string
		state domainstudy.ContextState
		reads int // GetByID calls: the compaction read, plus the blocked check when it gets that far
		want  error
	}{
		{"not blocked fails with the budget error", domainstudy.ContextStateNormal, 1, domainstudy.ErrBudgetImpossible},
		{"blocked keeps the 2.6 error", domainstudy.ContextStateBlocked, 2, domainstudy.ErrSessionContextLimitReached},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given three 31,000-token messages: the protected tail alone
			// exceeds the budget, so no summarizer call can help
			f := newCompactionFixture(t)
			history := heavyHistory(3, 31_000)
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(c.state, 50_000), nil).Times(c.reads)
			f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
			failed := 0

			// When sending a message
			err := f.send(CompactionHooks{OnFailed: func(error) { failed++ }}, compactionPrompt)

			// Then the matching error is returned, nothing was summarized
			// and the failure was reported
			require.ErrorIs(t, err, c.want)
			assert.Equal(t, 1, failed)
		})
	}
}

func TestSendMessage_compaction_pendingMessageThatCannotFit_isRejectedWithoutBlockingTheSession(t *testing.T) {
	// Given a message of ~100,000 tokens in a 100,000 window
	f := newCompactionFixture(t)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 1_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(nil, nil).Once()
	failed := 0

	// When sending it
	err := f.send(CompactionHooks{OnFailed: func(error) { failed++ }}, strings.Repeat("x", 300_000))

	// Then it is rejected before persisting, and that is not a compaction failure
	require.ErrorIs(t, err, domainstudy.ErrMessageTooLarge)
	assert.Zero(t, failed)
}

func TestSendMessage_compaction_unresolvedWindow_skipsCompactionAndKeepsTheOldBehavior(t *testing.T) {
	// Given a session whose context length is still unknown
	f := newCompactionFixture(t)
	unknown := domainstudy.Session{ID: "session-1", Topic: compactionTopic, Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal}}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(unknown, nil).Twice()
	f.expectReplyContexts(unknown.Context, nil, nil)
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	f.expectTurnCompletes(nil)

	// When sending a message
	err := f.send(CompactionHooks{OnStarted: func() { t.Error("must not compact") }}, compactionPrompt)

	// Then the turn runs with no ListAfter and no Summarize call
	require.NoError(t, err)
}

func TestSendMessage_compaction_normalSessionBelowTheThreshold_doesNotCompact(t *testing.T) {
	// Given a normal session whose next prompt is far below 80%
	f := newCompactionFixture(t)
	history := heavyHistory(4, 1000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 5_000), nil).Twice()
	f.expectReplyContexts(compactionSession(domainstudy.ContextStateNormal, 5_000).Context, nil, history)
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	f.expectTurnCompletes(nil)

	// When sending a message
	err := f.send(CompactionHooks{OnStarted: func() { t.Error("must not compact") }}, compactionPrompt)

	// Then no summarizer call happens
	require.NoError(t, err)
}

func TestSendMessage_compaction_triggersExactlyAtEightyPercentOfTheWindow(t *testing.T) {
	// The estimate is fixed + history + pending, so size the history to land
	// the prompt exactly on, then one token below, the 80,000-token threshold.
	pending := domainstudy.EstimateTokens(compactionPrompt)
	atThreshold := compactionWindow*80/100 - fixedCompactionTokens() - pending
	for _, c := range []struct {
		name     string
		history  int
		compacts bool
	}{
		{"exactly at 80% compacts", atThreshold, true},
		{"one token below does not", atThreshold - 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Given a normal session whose prompt is sized against the threshold
			f := newCompactionFixture(t)
			// 1 big message (the foldable one) + 3 protected small ones.
			history := []domainstudy.Message{
				tokenMessage("m1", domainstudy.RoleUser, c.history-3*50),
				tokenMessage("m2", domainstudy.RoleAssistant, 50),
				tokenMessage("m3", domainstudy.RoleUser, 50),
				tokenMessage("m4", domainstudy.RoleAssistant, 50),
			}
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 1), nil).Twice()
			f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
			if c.compacts {
				// The summarizer fails so the turn falls back to the full
				// history; only the attempt matters here.
				f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:1]).
					Return(domainstudy.ConversationSummary{}, errors.New("stop here")).Once()
			}
			f.expectReplyContexts(compactionSession(domainstudy.ContextStateNormal, 1).Context, nil, history)
			f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
			f.expectTurnCompletes(nil)
			started := 0

			// When sending a message
			_ = f.send(CompactionHooks{OnStarted: func() { started++ }}, compactionPrompt)

			// Then only the exact threshold starts a compaction
			assert.Equal(t, map[bool]int{true: 1, false: 0}[c.compacts], started)
		})
	}
}

func TestSendMessage_compaction_existingSummary_isSentEvenWhenNoNewCompactionIsNeeded(t *testing.T) {
	// Given a session with a stored summary and a short tail after it
	f := newCompactionFixture(t)
	summary := newSummaryOf("m3")
	tail := heavyHistory(2, 1000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 6_000), nil).Twice()
	f.expectReplyContexts(compactionSession(domainstudy.ContextStateNormal, 6_000).Context, &summary, tail)
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(summary, nil).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "m3").Return(tail, nil).Twice()
	var request domainllm.ChatRequest
	f.expectTurnCompletes(&request)

	// When sending a message
	err := f.send(CompactionHooks{}, compactionPrompt)

	// Then the prompt is [system, summary, tail]
	require.NoError(t, err)
	require.Len(t, request.Messages, 2+len(tail))
	assert.Contains(t, request.Messages[1].Content, "summary text")
}

func TestSendMessage_compaction_summaryRepositoryFailureIsAHardError(t *testing.T) {
	// Given a summary repository that fails
	f := newCompactionFixture(t)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 1), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, errors.New("disk")).Once()

	// When sending a message
	err := f.send(CompactionHooks{}, compactionPrompt)

	// Then the turn fails before persisting anything
	require.ErrorContains(t, err, "loading summary")
}

func TestSendMessage_compaction_persistenceFailure_isReportedAndNothingIsSaved(t *testing.T) {
	// Given a warning session whose summary saves fail; the history fits the window
	f := newCompactionFixture(t)
	history := heavyHistory(8, 9000)
	summary := newSummaryOf("m4")
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Twice()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:4]).Return(summary, nil).Once()
	f.summaries.EXPECT().Save(context.Background(), summary).Return(errors.New("disk")).Once()
	f.expectReplyContexts(compactionSession(domainstudy.ContextStateWarning, 85_000).Context, nil, history)
	f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	f.expectTurnCompletes(nil)
	var failures []error

	// When sending a message
	err := f.send(CompactionHooks{
		OnCompacted: func(ContextEvent) { t.Error("must not report success") },
		OnFailed:    func(err error) { failures = append(failures, err) },
	}, compactionPrompt)

	// Then the failure is reported and the turn continues with the full history
	require.NoError(t, err)
	require.Len(t, failures, 1)
	assert.ErrorContains(t, failures[0], "persisting compaction")
}

func TestCompactSession_foldsRegardlessOfTheThreshold_andReportsSuccess(t *testing.T) {
	// Given a normal session below 80% (57,300 of 100,000 estimated) but
	// above the 50% target, so there is something worth folding
	f := newCompactionFixture(t)
	history := heavyHistory(6, 9000)
	summary := newSummaryOf("m2")
	usage := compactionSession(domainstudy.ContextStateNormal, 57_000)
	reset := domainstudy.ContextUsage{
		State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + summary.TokenEstimate + tokensOf(history[2:]),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(usage, nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:2]).Return(summary, nil).Once()
	f.summaries.EXPECT().Save(context.Background(), summary).Return(nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", reset).Return(nil).Once()
	started, compacted := 0, 0

	// When compacting explicitly
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{
		OnStarted: func() { started++ }, OnCompacted: func(ContextEvent) { compacted++ },
	})

	// Then the summary and the reset usage are persisted and both hooks fire
	require.NoError(t, err)
	assert.Equal(t, 1, started)
	assert.Equal(t, 1, compacted)
}

func TestCompactSession_returnsTheCause_whenTheSummarizerFails(t *testing.T) {
	// Given a blocked session and a failing summarizer
	f := newCompactionFixture(t)
	history := heavyHistory(10, 9000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateBlocked, 96_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:6]).
		Return(domainstudy.ConversationSummary{}, ErrInvalidSummaryFormat).Once()
	var failures []error

	// When retrying the compaction
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{OnFailed: func(err error) { failures = append(failures, err) }})

	// Then the cause is returned and reported
	require.ErrorIs(t, err, ErrInvalidSummaryFormat)
	require.Len(t, failures, 1)
}

func TestCompactSession_returnsErrNothingToCompact_whenOnlyTheProtectedTailRemains(t *testing.T) {
	// Given a session with fewer messages than the protected tail
	f := newCompactionFixture(t)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 100), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(heavyHistory(3, 100), nil).Once()

	// When compacting explicitly
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then there is nothing to do
	require.ErrorIs(t, err, ErrNothingToCompact)
}

func TestCompactSession_returnsErrNothingToCompact_whenTheWindowIsUnknown(t *testing.T) {
	// Given a session without a resolved window
	f := newCompactionFixture(t)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1"}, nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()

	// When compacting explicitly
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then it reports there is nothing to compact
	require.ErrorIs(t, err, ErrNothingToCompact)
}

func TestCompactSession_withoutCompactionWiring_returnsErrNothingToCompact(t *testing.T) {
	// Given a service built without a summarizer
	service := NewService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	// When compacting explicitly
	err := service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then no port is touched
	require.ErrorIs(t, err, ErrNothingToCompact)
}

func TestCompactSession_whileATurnIsInFlight_returnsErrStudyTurnInProgress(t *testing.T) {
	// Given a session with a turn in flight
	f := newCompactionFixture(t)
	require.NoError(t, f.service.inFlight.begin("session-1"))

	// When compacting the same session
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then it is rejected without any port call
	require.ErrorIs(t, err, ErrStudyTurnInProgress)

	// And a different session is independent
	f.sessions.EXPECT().GetByID(context.Background(), "session-2").Return(domainstudy.Session{ID: "session-2"}, nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-2").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	require.ErrorIs(t, f.service.CompactSession(context.Background(), "session-2", CompactionHooks{}), ErrNothingToCompact)
}

func TestCompactSession_unknownSession_returnsTheRepositoryError(t *testing.T) {
	// Given a session that does not exist
	f := newCompactionFixture(t)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound).Once()

	// When compacting
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then the error is propagated
	require.ErrorIs(t, err, domainstudy.ErrSessionNotFound)
}

func TestCompactSession_succeedsWithoutAnyHooks(t *testing.T) {
	// Given a compaction that succeeds and a caller that registered no hooks
	f := newCompactionFixture(t)
	history := heavyHistory(6, 9000)
	summary := newSummaryOf("m2")
	reset := domainstudy.ContextUsage{
		State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + summary.TokenEstimate + tokensOf(history[2:]),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 57_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:2]).Return(summary, nil).Once()
	f.summaries.EXPECT().Save(context.Background(), summary).Return(nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", reset).Return(nil).Once()

	// When compacting
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then nil hooks are skipped, not called
	require.NoError(t, err)
}

func TestCompactSession_failsWithoutAnyHooks(t *testing.T) {
	// Given a failing summarizer and a caller that registered no hooks
	f := newCompactionFixture(t)
	history := heavyHistory(6, 9000)
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateNormal, 57_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:2]).
		Return(domainstudy.ConversationSummary{}, ErrSummaryTooLarge).Once()

	// When compacting
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then the cause is returned without calling a nil hook
	require.ErrorIs(t, err, ErrSummaryTooLarge)
}

func TestSendMessage_compaction_promptExactlyAtTheBudgetStillProceedsWhenTheSummarizerFails(t *testing.T) {
	// The unfolded prompt is fixed + history + pending; size the first
	// message so it lands exactly on window - reserve (90,000), then one over.
	pending := domainstudy.EstimateTokens(compactionPrompt)
	budget := domainstudy.PromptBudget(compactionWindow)
	exactFirst := budget - fixedCompactionTokens() - pending - 3*50
	for _, c := range []struct {
		name    string
		extra   int
		fitsOut bool
	}{
		{"exactly at the budget proceeds", 0, true},
		{"one token over is rejected", 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Given a warning session whose summarizer fails
			f := newCompactionFixture(t)
			history := []domainstudy.Message{
				tokenMessage("m1", domainstudy.RoleUser, exactFirst+c.extra),
				tokenMessage("m2", domainstudy.RoleAssistant, 50),
				tokenMessage("m3", domainstudy.RoleUser, 50),
				tokenMessage("m4", domainstudy.RoleAssistant, 50),
			}
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Times(map[bool]int{true: 2, false: 1}[c.fitsOut])
			f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
			f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:1]).
				Return(domainstudy.ConversationSummary{}, errors.New("provider down")).Once()
			if c.fitsOut {
				f.expectReplyContexts(compactionSession(domainstudy.ContextStateWarning, 85_000).Context, nil, history)
				f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
				f.expectTurnCompletes(nil)
			}

			// When sending a message
			err := f.send(CompactionHooks{}, compactionPrompt)

			// Then only a prompt over the budget is rejected
			if c.fitsOut {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, domainstudy.ErrBudgetImpossible)
			}
		})
	}
}

func TestSendMessage_compaction_blockedSessionWithNothingToFold_isUnblockedByTheRecomputedUsage(t *testing.T) {
	// Given a session blocked by native usage (96,000 of 100,000) whose
	// history is only the protected tail, so no message can be folded, but
	// whose estimated prompt is far below the window
	f := newCompactionFixture(t)
	history := heavyHistory(3, 100)
	blocked := compactionSession(domainstudy.ContextStateBlocked, 96_000)
	reset := domainstudy.ContextUsage{
		State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + tokensOf(history),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(blocked, nil).Once()
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(
		domainstudy.Session{ID: "session-1", Topic: compactionTopic, Context: reset}, nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", reset).Return(nil).Once()
	provisional := domainstudy.NextContextUsage(
		reset, "m", reset.UsedTokens+domainstudy.EstimateTokens(compactionPrompt), compactionWindow, true)
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", provisional).Return(nil).Once()
	// The reply carries no usage or model, so its measurement is estimated
	// from the stored history plus the empty reply, with an unresolved window.
	replyTokens := domainstudy.EstimateTokens(buildSystemPrompt(compactionProfile, compactionTopic, "")) +
		tokensOf(history) + domainstudy.EstimateTokens("")
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", domainstudy.NextContextUsage(provisional, "", replyTokens, 0, true)).Return(nil).Once()
	f.messages.EXPECT().ListBySession(context.Background(), "session-1").Return(history, nil).Once()
	f.expectTurnCompletes(nil)

	// When sending a message
	err := f.send(CompactionHooks{
		OnStarted:   func() { t.Error("nothing was summarized") },
		OnCompacted: func(ContextEvent) { t.Error("nothing was compacted") },
		OnFailed:    func(err error) { t.Errorf("unexpected failure: %v", err) },
	}, compactionPrompt)

	// Then the recomputed usage is persisted and the turn is no longer blocked
	require.NoError(t, err)
}

func TestCompactSession_blockedSessionWithNothingToFold_isRecoveredByTheRecomputedUsage(t *testing.T) {
	// Given a session blocked by native usage whose estimated prompt fits
	f := newCompactionFixture(t)
	history := heavyHistory(3, 100)
	reset := domainstudy.ContextUsage{
		State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + tokensOf(history),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateBlocked, 96_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", reset).Return(nil).Once()

	// When retrying the compaction
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then recovering the state counts as success, not as "nothing to compact"
	require.NoError(t, err)
}

func TestCompactSession_warningSessionWhoseRecomputedUsageStaysWarning_hasNothingToCompact(t *testing.T) {
	// Given a warning session whose estimated protected tail alone is above
	// 80% of the window but still inside the budget
	f := newCompactionFixture(t)
	history := heavyHistory(3, 28_000)
	same := domainstudy.ContextUsage{
		State: domainstudy.ContextStateWarning, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + tokensOf(history),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", same).Return(nil).Once()

	// When retrying the compaction
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{})

	// Then the state did not recover, so there is still nothing to compact
	require.ErrorIs(t, err, ErrNothingToCompact)
}

func TestCompactSession_blockedSessionWithNothingToFoldWhoseUpdateFails_returnsTheCause(t *testing.T) {
	// Given the same recoverable blocked session, but the update fails
	f := newCompactionFixture(t)
	history := heavyHistory(3, 100)
	reset := domainstudy.ContextUsage{
		State: domainstudy.ContextStateNormal, Model: "m", ContextLength: compactionWindow, Estimated: true,
		UsedTokens: fixedCompactionTokens() + tokensOf(history),
	}
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateBlocked, 96_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", reset).Return(errors.New("disk")).Once()
	var failures []error

	// When retrying the compaction
	err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{OnFailed: func(err error) { failures = append(failures, err) }})

	// Then the cause is returned and reported
	require.ErrorContains(t, err, "persisting compaction")
	require.Len(t, failures, 1)
}

func TestCompactSession_summaryLargerThanProjected_isRejectedWhenThePromptWouldExceedTheBudget(t *testing.T) {
	// The first three messages are folded and the last three are protected;
	// the protected tail is sized so the prompt lands exactly on the budget
	// with the projected summary size, and one token over with a bigger one.
	projected := domainstudy.SummaryTargetTokens(domainstudy.SummaryModelWindowTokens)
	rest := domainstudy.PromptBudget(compactionWindow) - fixedCompactionTokens() - projected
	for _, c := range []struct {
		name  string
		extra int
		fits  bool
	}{
		{"summary exactly as projected is persisted", 0, true},
		{"summary one token larger is rejected", 1, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			// Given a warning session whose summarizer returns that summary
			f := newCompactionFixture(t)
			history := append(heavyHistory(3, 9000),
				tokenMessage("m4", domainstudy.RoleAssistant, rest-200),
				tokenMessage("m5", domainstudy.RoleUser, 100),
				tokenMessage("m6", domainstudy.RoleAssistant, 100),
			)
			summary := newSummaryOf("m3")
			summary.TokenEstimate = projected + c.extra
			f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Once()
			f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
			f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
			f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:3]).Return(summary, nil).Once()
			if c.fits {
				f.summaries.EXPECT().Save(context.Background(), summary).Return(nil).Once()
				f.sessions.EXPECT().UpdateContext(context.Background(), "session-1", domainstudy.ContextUsageAfterCompaction(
					compactionSession(domainstudy.ContextStateWarning, 85_000).Context,
					fixedCompactionTokens()+summary.TokenEstimate+tokensOf(history[3:]), compactionWindow,
				)).Return(nil).Once()
			}
			var failures []error

			// When retrying the compaction
			err := f.service.CompactSession(context.Background(), "session-1", CompactionHooks{OnFailed: func(err error) { failures = append(failures, err) }})

			// Then only a prompt that fits the budget is persisted
			if c.fits {
				require.NoError(t, err)
				assert.Empty(t, failures)
			} else {
				require.ErrorIs(t, err, domainstudy.ErrBudgetImpossible)
				require.Len(t, failures, 1)
			}
		})
	}
}

func TestSendMessage_compaction_summaryLargerThanProjected_countsThePendingMessageAgainstTheBudget(t *testing.T) {
	// Given a 3,000-token pending message and a protected tail sized so that
	// tail + pending + fixed + a summary one token above the projection lands
	// exactly one token over the budget
	const pendingTokens = 3000
	pending := strings.Repeat("x", (pendingTokens-8)*3)
	projected := domainstudy.SummaryTargetTokens(domainstudy.SummaryModelWindowTokens)
	rest := domainstudy.PromptBudget(compactionWindow) - fixedCompactionTokens() - pendingTokens - projected
	f := newCompactionFixture(t)
	history := append(heavyHistory(3, 9000),
		tokenMessage("m4", domainstudy.RoleAssistant, rest-200),
		tokenMessage("m5", domainstudy.RoleUser, 100),
		tokenMessage("m6", domainstudy.RoleAssistant, 100),
	)
	summary := newSummaryOf("m3")
	summary.TokenEstimate = projected + 1
	f.sessions.EXPECT().GetByID(context.Background(), "session-1").Return(compactionSession(domainstudy.ContextStateWarning, 85_000), nil).Once()
	f.summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	f.messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil).Once()
	f.summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:3]).Return(summary, nil).Once()
	failed := 0

	// When sending the message
	err := f.send(CompactionHooks{OnFailed: func(error) { failed++ }}, pending)

	// Then the summary is not persisted and the turn fails before sending
	require.ErrorIs(t, err, domainstudy.ErrBudgetImpossible)
	assert.Equal(t, 1, failed)
}
