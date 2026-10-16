package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/study"
)

// MessageRepository is the SQLite-backed implementation of
// study.MessageRepository.
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a MessageRepository backed by db. db must
// already have its migrations applied (see Open).
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Append inserts a new message.
func (r *MessageRepository) Append(ctx context.Context, message study.Message) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO messages (id, session_id, role, content, created_at) VALUES (?, ?, ?, ?, ?)`,
		message.ID, message.SessionID, message.Role, message.Content, message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: appending message: %w", err)
	}
	return nil
}

// ListBySession returns every message for sessionID, ordered by when it was
// created. It returns an empty slice, not an error, when there are none.
func (r *MessageRepository) ListBySession(ctx context.Context, sessionID string) ([]study.Message, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing messages: %w", err)
	}
	return scanMessages(rows)
}

// ListAfter returns the messages of sessionID that follow afterMessageID,
// ordered by (created_at, rowid) so messages sharing a timestamp keep their
// insertion order. An empty afterMessageID returns the whole conversation;
// a cursor that is not a message of the session yields
// study.ErrMessageNotInSession.
func (r *MessageRepository) ListAfter(ctx context.Context, sessionID, afterMessageID string) ([]study.Message, error) {
	if afterMessageID == "" {
		return r.listOrdered(ctx, sessionID)
	}
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, session_id, role, content, created_at FROM messages
		 WHERE session_id = ?1
		   AND (created_at, rowid) > (SELECT created_at, rowid FROM messages WHERE id = ?2 AND session_id = ?1)
		 ORDER BY created_at ASC, rowid ASC`,
		sessionID, afterMessageID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing messages after %q: %w", afterMessageID, err)
	}
	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		// Either the cursor was the last message or it is not in the session;
		// the row-value comparison is NULL for an unknown cursor, so check it.
		var exists bool
		if err := execer(ctx, r.db).QueryRowContext(ctx,
			`SELECT EXISTS(SELECT 1 FROM messages WHERE id = ? AND session_id = ?)`,
			afterMessageID, sessionID,
		).Scan(&exists); err != nil {
			return nil, fmt.Errorf("sqlite: checking message cursor: %w", err)
		}
		if !exists {
			return nil, study.ErrMessageNotInSession
		}
	}
	return messages, nil
}

// listOrdered is ListBySession with the deterministic (created_at,
// rowid) ordering used by ListAfter.
func (r *MessageRepository) listOrdered(ctx context.Context, sessionID string) ([]study.Message, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, session_id, role, content, created_at FROM messages WHERE session_id = ? ORDER BY created_at ASC, rowid ASC`,
		sessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing messages: %w", err)
	}
	return scanMessages(rows)
}

func scanMessages(rows *sql.Rows) ([]study.Message, error) {
	defer func() { _ = rows.Close() }()

	messages := []study.Message{}
	for rows.Next() {
		var message study.Message
		if err := rows.Scan(&message.ID, &message.SessionID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating messages: %w", err)
	}
	return messages, nil
}
