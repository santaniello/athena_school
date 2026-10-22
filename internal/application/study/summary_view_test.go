package study

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func newSummaryViewService(t *testing.T, summaries domainstudy.ConversationSummaryRepository) *Service {
	t.Helper()
	options := []Option{}
	if summaries != nil {
		options = append(options, WithSummaries(summaries))
	}
	return NewService(studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t),
		nil, nil, nil, nil, nil, options...)
}

func TestGetSummary_returnsTheCurrentSummary(t *testing.T) {
	// Given a session with a summary
	want := domainstudy.ConversationSummary{SessionID: "session-1", Content: "## Progress", CoversUntilMessageID: "message-3", UpdatedAt: time.Now()}
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").Return(want, nil).Once()

	// When reading it
	got, found, err := newSummaryViewService(t, summaries).GetSummary(context.Background(), "session-1")

	// Then it is returned
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, want, got)
}

func TestGetSummary_reportsNotFoundWithoutError(t *testing.T) {
	// Given a session that was never compacted
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").
		Return(domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound).Once()

	// When reading its summary
	_, found, err := newSummaryViewService(t, summaries).GetSummary(context.Background(), "session-1")

	// Then there is simply no summary
	require.NoError(t, err)
	require.False(t, found)
}

func TestGetSummary_withoutSummaryRepositoryReportsNotFound(t *testing.T) {
	// Given a service wired without compaction
	// When reading a summary
	_, found, err := newSummaryViewService(t, nil).GetSummary(context.Background(), "session-1")

	// Then there is no summary
	require.NoError(t, err)
	require.False(t, found)
}

func TestGetSummary_propagatesReadError(t *testing.T) {
	// Given a summary repository that fails
	readErr := errors.New("database unavailable")
	summaries := studymocks.NewMockConversationSummaryRepository(t)
	summaries.EXPECT().Get(context.Background(), "session-1").Return(domainstudy.ConversationSummary{}, readErr).Once()

	// When reading the summary
	_, _, err := newSummaryViewService(t, summaries).GetSummary(context.Background(), "session-1")

	// Then the error propagates
	require.ErrorIs(t, err, readErr)
}
