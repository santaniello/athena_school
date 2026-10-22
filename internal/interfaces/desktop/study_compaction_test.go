package desktop

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/study"
	studymocks2 "github.com/santaniello/athena/internal/application/study/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestCompactionFailureReason_mapsErrorsToShortUserFacingKinds(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"invalid format", fmt.Errorf("wrapped: %w", study.ErrInvalidSummaryFormat), "invalid format"},
		{"too large", study.ErrSummaryTooLarge, "summary too large"},
		{"budget", domainstudy.ErrBudgetImpossible, "conversation too large"},
		{"credits", domainllm.ErrInsufficientCredits, "provider unavailable"},
		{"anything else", errors.New("dial tcp: i/o timeout"), "provider unavailable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Given an error returned by a failed compaction
			// When mapping it to the reason shown to the user
			got := compactionFailureReason(c.err)

			// Then it is a short fixed kind, never the raw error text
			assert.Equal(t, c.want, got)
		})
	}
}

func TestErrorCode_mapsPrePersistenceCompactionErrors(t *testing.T) {
	// Given the errors SendMessage returns before persisting anything
	// When mapping them
	// Then each has a stable wire code
	assert.Equal(t, "budget_impossible", errorCode(domainstudy.ErrBudgetImpossible))
	assert.Equal(t, "message_too_large", errorCode(fmt.Errorf("x: %w", domainstudy.ErrMessageTooLarge)))
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(original) })
	return &buf
}

func TestApp_compactionHooks_emitEventsAndLogTheFailureWithoutLeakingTheRawError(t *testing.T) {
	// Given an app and a captured log
	app, captured := newTestStudyApp(t, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t),
		llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t))
	logs := captureLog(t)
	hooks := app.compactionHooks("session-1")

	// When the three hooks fire
	hooks.OnStarted()
	hooks.OnCompacted(study.ContextEvent{State: domainstudy.ContextStateNormal, UsedTokens: 40, ContextLength: 100})
	hooks.OnFailed(fmt.Errorf("secret provider detail: %w", study.ErrInvalidSummaryFormat))

	// Then each emits its session-scoped event, and only the failure is logged
	assert.Equal(t, []StudyCompactingEvent{{SessionID: "session-1"}}, captured.compacting)
	assert.Equal(t, []StudyCompactedEvent{{SessionID: "session-1", UsedTokens: 40, ContextLength: 100}}, captured.compacted)
	assert.Equal(t, []StudyCompactionFailedEvent{{SessionID: "session-1", Reason: "invalid format"}}, captured.compactionFailed)
	assert.Contains(t, logs.String(), `"session-1"`)
	assert.Contains(t, logs.String(), "secret provider detail")
}

func TestApp_CompactStudySession_failure_returnsTheErrorEmitsFailedEventAndLogsIt(t *testing.T) {
	// Given a blocked session with enough history to compact and a
	// summarizer that returns a malformed summary
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summarizer := studymocks2.NewMockConversationSummarizer(t)
	profiles := profilemocks.NewMockStore(t)
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{
		ID: "session-1", Topic: "Go",
		Context: domainstudy.ContextUsage{State: domainstudy.ContextStateBlocked, Model: "m", UsedTokens: 96_000, ContextLength: 100_000},
	}, nil)
	summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound)
	history := make([]domainstudy.Message, 0, 10)
	for i := 1; i <= 10; i++ {
		history = append(history, domainstudy.Message{
			ID: fmt.Sprintf("m%d", i), SessionID: "session-1", Role: domainstudy.RoleUser, Content: strings.Repeat("x", 27_000),
		})
	}
	messages.EXPECT().ListAfter(context.Background(), "session-1", "").Return(history, nil)
	summarizer.EXPECT().Summarize(context.Background(), "session-1", (*domainstudy.ConversationSummary)(nil), history[:6]).
		Return(domainstudy.ConversationSummary{}, fmt.Errorf("raw detail: %w", study.ErrInvalidSummaryFormat))
	app, captured := newTestStudyApp(t, sessions, messages, llmmocks.NewMockProvider(t), profiles, foldermocks.NewMockRepository(t),
		study.WithSummarizer(summarizer), study.WithSummaries(summaries))
	logs := captureLog(t)

	// When the user retries the compaction
	err := app.CompactStudySession("session-1")

	// Then the binding returns the error, emits the failure with a short
	// reason only, and logs the session id and the raw error
	require.ErrorIs(t, err, study.ErrInvalidSummaryFormat)
	assert.Equal(t, []StudyCompactingEvent{{SessionID: "session-1"}}, captured.compacting)
	require.Len(t, captured.compactionFailed, 1)
	assert.Equal(t, "invalid format", captured.compactionFailed[0].Reason)
	assert.NotContains(t, captured.compactionFailed[0].Reason, "raw detail")
	assert.Contains(t, logs.String(), "raw detail")
	assert.Empty(t, captured.compacted)
}

func TestApp_ResumeStudySession_withASummary_returnsItsCursor(t *testing.T) {
	// Given a session compacted up to message-3
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").
		Return(domainstudy.Session{ID: "session-1", Context: domainstudy.ContextUsage{State: domainstudy.ContextStateNormal}}, nil).Once()
	messages.EXPECT().
		ListPage(context.Background(), "session-1", (*domainstudy.MessageCursor)(nil), study.DefaultMessagePageSize+1).
		Return(nil, nil).Once()
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{SessionID: "session-1", Content: "## Progress", CoversUntilMessageID: "message-3"}, nil).Once()
	app, _ := newTestStudyApp(t, sessions, messages, llmmocks.NewMockProvider(t), profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t),
		study.WithSummaries(summaries))

	// When resuming the session
	result, err := app.ResumeStudySession("session-1")

	// Then the DTO carries the cursor of the summary
	require.NoError(t, err)
	assert.Equal(t, "message-3", result.SummaryCoversUntilMessageID)
}

func TestApp_GetStudySessionSummary_returnsTheSummary(t *testing.T) {
	// Given a session with a summary
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	updatedAt := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{SessionID: "session-1", Content: "## Progress", CoversUntilMessageID: "message-3", UpdatedAt: updatedAt}, nil).Once()
	app, _ := newTestStudyApp(t, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), study.WithSummaries(summaries))

	// When reading it
	result, err := app.GetStudySessionSummary("session-1")

	// Then the DTO exposes content, cursor and timestamp
	require.NoError(t, err)
	assert.Equal(t, StudySummaryResult{Found: true, Content: "## Progress", CoversUntilMessageID: "message-3", UpdatedAt: "2026-10-21T12:00:00Z"}, result)
}

func TestApp_GetStudySessionSummary_withoutSummary_returnsNotFound(t *testing.T) {
	// Given a session that was never compacted
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()
	app, _ := newTestStudyApp(t, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), study.WithSummaries(summaries))

	// When reading its summary
	result, err := app.GetStudySessionSummary("session-1")

	// Then it is reported as not found without an error
	require.NoError(t, err)
	assert.Equal(t, StudySummaryResult{}, result)
}

func TestApp_GetStudySessionSummary_propagatesReadError(t *testing.T) {
	// Given a summary repository that fails
	readErr := errors.New("database unavailable")
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, readErr).Once()
	app, _ := newTestStudyApp(t, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), study.WithSummaries(summaries))

	// When reading the summary
	_, err := app.GetStudySessionSummary("session-1")

	// Then the error propagates
	require.ErrorIs(t, err, readErr)
}
