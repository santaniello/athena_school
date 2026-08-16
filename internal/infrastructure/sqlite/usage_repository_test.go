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
