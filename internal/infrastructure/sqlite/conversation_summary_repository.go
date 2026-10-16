package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// ConversationSummaryRepository is the SQLite-backed implementation of
// domainstudy.ConversationSummaryRepository.
type ConversationSummaryRepository struct {
	db *sql.DB
}

// NewConversationSummaryRepository creates a ConversationSummaryRepository
// backed by db. db must already have its migrations applied (see Open).
func NewConversationSummaryRepository(db *sql.DB) *ConversationSummaryRepository {
	return &ConversationSummaryRepository{db: db}
}

// Get returns the session's summary, or ErrConversationSummaryNotFound.
func (r *ConversationSummaryRepository) Get(ctx context.Context, sessionID string) (domainstudy.ConversationSummary, error) {
	var summary domainstudy.ConversationSummary
	err := execer(ctx, r.db).QueryRowContext(ctx, `
		SELECT session_id, content, covers_until_message_id, token_estimate, updated_at
		FROM conversation_summaries WHERE session_id = ?`, sessionID,
	).Scan(&summary.SessionID, &summary.Content, &summary.CoversUntilMessageID, &summary.TokenEstimate, &summary.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound
	}
	if err != nil {
		return domainstudy.ConversationSummary{}, fmt.Errorf("sqlite: getting conversation summary: %w", err)
	}
	return summary, nil
}

// Save inserts or replaces the session's summary. The ownership check and the
// write are one statement, so it is atomic on its own and also inside a
// caller's WithinTx (execer joins it; the pool has a single connection, so a
// nested BeginTx would block). Nothing is written when the covered message
// does not belong to the session.
func (r *ConversationSummaryRepository) Save(ctx context.Context, summary domainstudy.ConversationSummary) error {
	result, err := execer(ctx, r.db).ExecContext(ctx, `
		INSERT INTO conversation_summaries (session_id, content, covers_until_message_id, token_estimate, updated_at)
		SELECT ?, ?, ?, ?, ?
		WHERE EXISTS (SELECT 1 FROM messages WHERE id = ? AND session_id = ?)
		ON CONFLICT(session_id) DO UPDATE SET
			content = excluded.content,
			covers_until_message_id = excluded.covers_until_message_id,
			token_estimate = excluded.token_estimate,
			updated_at = excluded.updated_at`,
		summary.SessionID, summary.Content, summary.CoversUntilMessageID, summary.TokenEstimate, summary.UpdatedAt,
		summary.CoversUntilMessageID, summary.SessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving conversation summary: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: saving conversation summary: %w", err)
	}
	if affected == 0 {
		return domainstudy.ErrSummaryMessageNotInSession
	}
	return nil
}
