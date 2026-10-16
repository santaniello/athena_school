package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

func newTestUsageRepository(t *testing.T) (*UsageRepository, *sql.DB) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "athena.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`INSERT INTO folders (id, name, created_at) VALUES ('folder-1', 'General', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES
		('sess-1', 'Go', 'socratic', 'folder-1', CURRENT_TIMESTAMP),
		('sess-2', 'Rust', 'socratic', 'folder-1', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	return NewUsageRepository(db), db
}

func TestUsageRepository_Record_insertsRow_readableViaRawQuery(t *testing.T) {
	// Given a repository and a usage entry
	repo, db := newTestUsageRepository(t)
	ctx := context.Background()
	entry := domainllm.UsageEntry{
		ID:           "usage-1",
		SessionID:    "sess-1",
		Model:        "anthropic/claude-sonnet-4.5",
		InputTokens:  10,
		OutputTokens: 15,
		Cost:         0.0012,
		CreatedAt:    time.Now().UTC().Truncate(time.Second),
	}

	// When recording it
	err := repo.Record(ctx, entry)

	// Then the row is readable in the usage table with every column intact
	require.NoError(t, err)
	var (
		sessionID    string
		model        string
		inputTokens  int
		outputTokens int
		cost         float64
		createdAt    time.Time
	)
	queryErr := db.QueryRow(
		`SELECT session_id, model, input_tokens, output_tokens, cost, created_at FROM usage WHERE id = ?`, "usage-1",
	).Scan(&sessionID, &model, &inputTokens, &outputTokens, &cost, &createdAt)
	require.NoError(t, queryErr)
	assert.Equal(t, entry.SessionID, sessionID)
	assert.Equal(t, entry.Model, model)
	assert.Equal(t, entry.InputTokens, inputTokens)
	assert.Equal(t, entry.OutputTokens, outputTokens)
	assert.Equal(t, entry.Cost, cost)
	assert.True(t, entry.CreatedAt.Equal(createdAt))
}

func TestUsageRepository_Record_withoutSessionStoresNullSessionID(t *testing.T) {
	// Given a usage entry produced outside a study session
	repo, db := newTestUsageRepository(t)
	ctx := context.Background()
	entry := domainllm.UsageEntry{
		ID: "usage-1", Model: "anthropic/claude-sonnet-4.5",
		InputTokens: 10, OutputTokens: 5, Cost: 0.001,
	}

	// When recording the usage
	err := repo.Record(ctx, entry)

	// Then it is stored without a foreign-key value, not as an empty ID
	require.NoError(t, err)
	var sessionID sql.NullString
	require.NoError(t, db.QueryRow(`SELECT session_id FROM usage WHERE id = ?`, entry.ID).Scan(&sessionID))
	assert.False(t, sessionID.Valid)
}

func TestUsageRepository_Record_returnsError_whenIDAlreadyExists(t *testing.T) {
	// Given a repository with an existing usage entry
	repo, _ := newTestUsageRepository(t)
	ctx := context.Background()
	require.NoError(t, repo.Record(ctx, domainllm.UsageEntry{ID: "usage-1", SessionID: "sess-1", Model: "m"}))

	// When recording another entry with the same ID
	err := repo.Record(ctx, domainllm.UsageEntry{ID: "usage-1", SessionID: "sess-2", Model: "m"})

	// Then it fails instead of silently overwriting or being swallowed
	assert.Error(t, err)
}

func TestUsageRepository_SessionTotals_sumsOnlyTheRequestedSession(t *testing.T) {
	// Given usage rows for two sessions, including an embedding-style row
	// (input tokens only)
	repo, _ := newTestUsageRepository(t)
	ctx := context.Background()
	for _, entry := range []domainllm.UsageEntry{
		{ID: "u1", SessionID: "sess-1", Model: "m", InputTokens: 10, OutputTokens: 5, Cost: 0.01},
		{ID: "u2", SessionID: "sess-1", Model: "e", InputTokens: 7, Cost: 0.002},
		{ID: "u3", SessionID: "sess-2", Model: "m", InputTokens: 100, OutputTokens: 100, Cost: 1},
	} {
		require.NoError(t, repo.Record(ctx, entry))
	}

	// When reading sess-1 totals
	totals, err := repo.SessionTotals(ctx, "sess-1")

	// Then only sess-1 rows are summed and every call had a reported cost
	require.NoError(t, err)
	assert.Equal(t, 17, totals.InputTokens)
	assert.Equal(t, 5, totals.OutputTokens)
	assert.InDelta(t, 0.012, totals.Cost, 1e-9)
	assert.Equal(t, 0, totals.UnpricedCalls)
	assert.Equal(t, 2, totals.Calls)
}

func TestUsageRepository_SessionTotals_countsCallsWithTokensButNoCostAsUnpriced(t *testing.T) {
	// Given a priced call, an unpriced call with tokens and an all-zero row
	repo, _ := newTestUsageRepository(t)
	ctx := context.Background()
	for _, entry := range []domainllm.UsageEntry{
		{ID: "u1", SessionID: "sess-1", Model: "m", InputTokens: 10, OutputTokens: 5, Cost: 0.01},
		{ID: "u2", SessionID: "sess-1", Model: "m", InputTokens: 3, OutputTokens: 4},
		{ID: "u3", SessionID: "sess-1", Model: "m"},
	} {
		require.NoError(t, repo.Record(ctx, entry))
	}

	// When reading the totals
	totals, err := repo.SessionTotals(ctx, "sess-1")

	// Then only the row with tokens and cost 0 is unpriced
	require.NoError(t, err)
	assert.Equal(t, 1, totals.UnpricedCalls)
	assert.Equal(t, 3, totals.Calls)
}

func TestUsageRepository_SessionTotals_returnsZeroValue_whenSessionHasNoUsage(t *testing.T) {
	// Given a session without any recorded call
	repo, _ := newTestUsageRepository(t)

	// When reading its totals
	totals, err := repo.SessionTotals(context.Background(), "sess-1")

	// Then it is an empty total, not an error
	require.NoError(t, err)
	assert.Equal(t, domainllm.UsageTotals{}, totals)
}

func TestUsageRepository_SessionTotals_returnsError_whenQueryFails(t *testing.T) {
	// Given a repository whose database is closed
	repo, db := newTestUsageRepository(t)
	require.NoError(t, db.Close())

	// When reading totals
	_, err := repo.SessionTotals(context.Background(), "sess-1")

	// Then the failure is surfaced
	assert.Error(t, err)
}
