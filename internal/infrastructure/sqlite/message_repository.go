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
