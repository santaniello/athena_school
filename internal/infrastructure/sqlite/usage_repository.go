package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	domainllm "github.com/santaniello/athena/internal/domain/llm"
)

// UsageRepository is the SQLite-backed implementation of
// domainllm.UsageRecorder.
type UsageRepository struct {
	db *sql.DB
}

// NewUsageRepository creates a UsageRepository backed by db. db must
// already have its migrations applied (see Open).
func NewUsageRepository(db *sql.DB) *UsageRepository {
	return &UsageRepository{db: db}
}

// Record inserts a new usage entry.
func (r *UsageRepository) Record(ctx context.Context, entry domainllm.UsageEntry) error {
	sessionID := sql.NullString{String: entry.SessionID, Valid: entry.SessionID != ""}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, sessionID, entry.Model, entry.InputTokens, entry.OutputTokens, entry.Cost, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: recording usage: %w", err)
	}
	return nil
}

// SessionTotals sums every usage row attributed to sessionID. A session
// without rows yields the zero UsageTotals.
func (r *UsageRepository) SessionTotals(ctx context.Context, sessionID string) (domainllm.UsageTotals, error) {
	var totals domainllm.UsageTotals
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(cost), 0),
			COALESCE(SUM(CASE WHEN cost = 0 AND (input_tokens > 0 OR output_tokens > 0) THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM usage WHERE session_id = ?`, sessionID,
	).Scan(&totals.InputTokens, &totals.OutputTokens, &totals.Cost, &totals.UnpricedCalls, &totals.Calls)
	if err != nil {
		return domainllm.UsageTotals{}, fmt.Errorf("sqlite: reading session usage totals: %w", err)
	}
	return totals, nil
}
