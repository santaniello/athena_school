package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		`INSERT INTO sessions (id, topic, mode, folder_id, started_at) VALUES (?, ?, ?, ?, ?)`,
		session.ID, session.Topic, session.Mode, session.FolderID, session.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating session: %w", err)
	}
	return nil
}

// GetByID returns the session with the given id, or
// study.ErrSessionNotFound if it does not exist.
func (r *SessionRepository) GetByID(ctx context.Context, id string) (study.Session, error) {
	var s study.Session
	err := r.db.QueryRowContext(ctx,
		`SELECT id, topic, mode, folder_id, started_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.Topic, &s.Mode, &s.FolderID, &s.StartedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return study.Session{}, study.ErrSessionNotFound
	}
	if err != nil {
		return study.Session{}, fmt.Errorf("sqlite: getting session: %w", err)
	}
	return s, nil
}

// ListByFolder returns every session in the given folder.
func (r *SessionRepository) ListByFolder(ctx context.Context, folderID string) ([]study.Session, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, topic, mode, folder_id, started_at FROM sessions WHERE folder_id = ?`, folderID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing sessions by folder: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sessions := []study.Session{}
	for rows.Next() {
		var s study.Session
		if err := rows.Scan(&s.ID, &s.Topic, &s.Mode, &s.FolderID, &s.StartedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session: %w", err)
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating sessions: %w", err)
	}
	return sessions, nil
}

// MoveToFolder reassigns the session with the given id to folderID, or
// returns study.ErrSessionNotFound if it does not exist.
func (r *SessionRepository) MoveToFolder(ctx context.Context, id, folderID string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE sessions SET folder_id = ? WHERE id = ?`, folderID, id)
	if err != nil {
		return fmt.Errorf("sqlite: moving session to folder: %w", err)
	}
	return requireRowAffected(result, study.ErrSessionNotFound)
}

// ReassignFolder moves every session in fromFolderID to toFolderID.
func (r *SessionRepository) ReassignFolder(ctx context.Context, fromFolderID, toFolderID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sessions SET folder_id = ? WHERE folder_id = ?`, toFolderID, fromFolderID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: reassigning folder: %w", err)
	}
	return nil
}

// Delete permanently removes the session with the given id, or returns
// study.ErrSessionNotFound if it does not exist.
func (r *SessionRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting session: %w", err)
	}
	return requireRowAffected(result, study.ErrSessionNotFound)
}
