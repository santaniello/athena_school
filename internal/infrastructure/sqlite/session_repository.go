package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/study"
)

// SessionRepository is the SQLite-backed implementation of
// study.SessionRepository.
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository creates a SessionRepository backed by db. db must
// already have its migrations applied (see Open).
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create inserts a new study session.
func (r *SessionRepository) Create(ctx context.Context, session study.Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (id, topic, mode, started_at) VALUES (?, ?, ?, ?)`,
		session.ID, session.Topic, session.Mode, session.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating session: %w", err)
	}
	return nil
}

// End sets endedAt on the session with the given id, or returns
// study.ErrSessionNotFound if it does not exist.
func (r *SessionRepository) End(ctx context.Context, id string, endedAt time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET ended_at = ? WHERE id = ?`, endedAt, id,
	)
	if err != nil {
		return fmt.Errorf("sqlite: ending session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: checking rows affected: %w", err)
	}
	if rows == 0 {
		return study.ErrSessionNotFound
	}
	return nil
}
