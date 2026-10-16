package study

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSessionUsage_returnsTotalsAndPersistedContext(t *testing.T) {
	// Given a session with a persisted context and recorded usage
	sessions := studymocks.NewMockSessionRepository(t)
	reader := llmmocks.NewMockUsageReader(t)
	persisted := domainstudy.ContextUsage{
		State: domainstudy.ContextStateWarning, Model: "m", UsedTokens: 8000, ContextLength: 10000, Estimated: true,
	}
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1", Context: persisted}, nil).Once()
	totals := domainllm.UsageTotals{InputTokens: 10, OutputTokens: 5, Cost: 0.01, Calls: 2}
	reader.EXPECT().SessionTotals(context.Background(), "session-1").Return(totals, nil).Once()
	service := NewService(sessions, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithUsageReader(reader))

	// When reading the session usage
	got, err := service.SessionUsage(context.Background(), "session-1")

	// Then both metrics are returned, unmixed
	require.NoError(t, err)
	require.Equal(t, SessionUsageSnapshot{Totals: totals, Context: persisted}, got)
}

func TestSessionUsage_wrapsReaderError(t *testing.T) {
	// Given a usage reader that fails
	readerErr := errors.New("db down")
	sessions := studymocks.NewMockSessionRepository(t)
	reader := llmmocks.NewMockUsageReader(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{ID: "session-1"}, nil).Once()
	reader.EXPECT().SessionTotals(context.Background(), "session-1").Return(domainllm.UsageTotals{}, readerErr).Once()
	service := NewService(sessions, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithUsageReader(reader))

	// When reading the session usage
	_, err := service.SessionUsage(context.Background(), "session-1")

	// Then the cause is preserved
	require.ErrorIs(t, err, readerErr)
}

func TestSessionUsage_wrapsSessionLookupError(t *testing.T) {
	// Given a session that cannot be found
	lookupErr := errors.New("not found")
	sessions := studymocks.NewMockSessionRepository(t)
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(domainstudy.Session{}, lookupErr).Once()
	service := NewService(sessions, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithUsageReader(llmmocks.NewMockUsageReader(t)))

	// When reading the session usage
	_, err := service.SessionUsage(context.Background(), "session-1")

	// Then the cause is preserved and no totals are read
	require.ErrorIs(t, err, lookupErr)
}
