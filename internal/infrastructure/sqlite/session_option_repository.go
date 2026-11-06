package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/exercise"
	"github.com/santaniello/athena/internal/domain/study"
)

// SessionOptionRepository is the SQLite-backed implementation of
// exercise.SessionOptionRepository. It reads only session and folder columns,
// never message content.
type SessionOptionRepository struct {
	db *sql.DB
}

// NewSessionOptionRepository creates a SessionOptionRepository backed by db.
// db must already have its migrations applied (see Open).
func NewSessionOptionRepository(db *sql.DB) *SessionOptionRepository {
	return &SessionOptionRepository{db: db}
}

// ListAll returns every study session with its folder name, newest first and
// ties broken by id so the order is stable.
func (r *SessionOptionRepository) ListAll(ctx context.Context) ([]exercise.SessionOption, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT s.id, s.topic, f.name, s.started_at
		FROM sessions s JOIN folders f ON f.id = s.folder_id
		WHERE s.mode = ?
		ORDER BY s.started_at DESC, s.id`, study.ModeStudy)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing session options: %w", err)
	}
	defer func() { _ = rows.Close() }()

	options := []exercise.SessionOption{}
	for rows.Next() {
		var option exercise.SessionOption
		if err := rows.Scan(&option.SessionID, &option.Topic, &option.FolderName, &option.StartedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session option: %w", err)
		}
		options = append(options, option)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating session options: %w", err)
	}
	return options, nil
}
