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
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO usage (id, session_id, model, input_tokens, output_tokens, cost, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.SessionID, entry.Model, entry.InputTokens, entry.OutputTokens, entry.Cost, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: recording usage: %w", err)
	}
	return nil
}
