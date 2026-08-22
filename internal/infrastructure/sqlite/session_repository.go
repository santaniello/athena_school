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
	if err := validateContextUsage(session.Context); err != nil {
		return fmt.Errorf("sqlite: invalid session context: %w", err)
	}
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO sessions (
			id, topic, mode, folder_id, started_at,
			context_state, context_model, context_used_tokens, context_length, context_estimated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Topic, session.Mode, session.FolderID, session.StartedAt,
		string(session.Context.State), session.Context.Model, session.Context.UsedTokens,
		session.Context.ContextLength, boolToInt(session.Context.Estimated),
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
	var contextState string
	var estimated int
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, topic, mode, folder_id, started_at,
			context_state, context_model, context_used_tokens, context_length, context_estimated
		FROM sessions WHERE id = ?`, id,
	).Scan(
		&s.ID, &s.Topic, &s.Mode, &s.FolderID, &s.StartedAt,
		&contextState, &s.Context.Model, &s.Context.UsedTokens, &s.Context.ContextLength, &estimated,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return study.Session{}, study.ErrSessionNotFound
	}
	if err != nil {
		return study.Session{}, fmt.Errorf("sqlite: getting session: %w", err)
	}
	if err := decodeContextUsage(&s.Context, contextState, estimated); err != nil {
		return study.Session{}, fmt.Errorf("sqlite: decoding session context: %w", err)
	}
	return s, nil
}

// ListByFolder returns every session in the given folder.
func (r *SessionRepository) ListByFolder(ctx context.Context, folderID string) ([]study.Session, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, topic, mode, folder_id, started_at,
			context_state, context_model, context_used_tokens, context_length, context_estimated
		FROM sessions WHERE folder_id = ?`, folderID,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing sessions by folder: %w", err)
	}
	defer func() { _ = rows.Close() }()

	sessions := []study.Session{}
	for rows.Next() {
		var s study.Session
		var contextState string
		var estimated int
		if err := rows.Scan(
			&s.ID, &s.Topic, &s.Mode, &s.FolderID, &s.StartedAt,
			&contextState, &s.Context.Model, &s.Context.UsedTokens, &s.Context.ContextLength, &estimated,
		); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session: %w", err)
		}
		if err := decodeContextUsage(&s.Context, contextState, estimated); err != nil {
			return nil, fmt.Errorf("sqlite: decoding session context: %w", err)
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
	result, err := execer(ctx, r.db).ExecContext(ctx, `UPDATE sessions SET folder_id = ? WHERE id = ?`, folderID, id)
	if err != nil {
		return fmt.Errorf("sqlite: moving session to folder: %w", err)
	}
	return requireRowAffected(result, study.ErrSessionNotFound)
}

// ReassignFolder moves every session in fromFolderID to toFolderID.
func (r *SessionRepository) ReassignFolder(ctx context.Context, fromFolderID, toFolderID string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
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
	result, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting session: %w", err)
	}
	return requireRowAffected(result, study.ErrSessionNotFound)
}

// UpdateContext persists sessionID's new ContextUsage, or returns
// study.ErrSessionNotFound if it does not exist.
func (r *SessionRepository) UpdateContext(ctx context.Context, sessionID string, usage study.ContextUsage) error {
	if err := validateContextUsage(usage); err != nil {
		return fmt.Errorf("sqlite: invalid session context: %w", err)
	}
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`UPDATE sessions SET
			context_state = ?, context_model = ?, context_used_tokens = ?, context_length = ?, context_estimated = ?
		WHERE id = ?`,
		string(usage.State), usage.Model, usage.UsedTokens, usage.ContextLength, boolToInt(usage.Estimated), sessionID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: updating session context: %w", err)
	}
	return requireRowAffected(result, study.ErrSessionNotFound)
}

// validateContextUsage rejects a ContextUsage before it reaches SQL: an
// unknown state or a negative token/length count is a programming error,
// not data to normalize silently.
func validateContextUsage(usage study.ContextUsage) error {
	switch usage.State {
	case study.ContextStateNormal, study.ContextStateWarning, study.ContextStateBlocked:
	default:
		return fmt.Errorf("invalid context state %q", usage.State)
	}
	if usage.UsedTokens < 0 {
		return fmt.Errorf("negative context used tokens %d", usage.UsedTokens)
	}
	if usage.ContextLength < 0 {
		return fmt.Errorf("negative context length %d", usage.ContextLength)
	}
	return nil
}

// decodeContextUsage validates and applies raw column values (state,
// estimated) scanned alongside usage's already-scanned Model/UsedTokens/
// ContextLength fields. A read of a malformed row is a decode error, not
// silently normalized data.
func decodeContextUsage(usage *study.ContextUsage, state string, estimated int) error {
	switch study.ContextState(state) {
	case study.ContextStateNormal, study.ContextStateWarning, study.ContextStateBlocked:
		usage.State = study.ContextState(state)
	default:
		return fmt.Errorf("unknown context_state %q", state)
	}
	if usage.UsedTokens < 0 {
		return fmt.Errorf("negative context_used_tokens %d", usage.UsedTokens)
	}
	if usage.ContextLength < 0 {
		return fmt.Errorf("negative context_length %d", usage.ContextLength)
	}
	switch estimated {
	case 0:
		usage.Estimated = false
	case 1:
		usage.Estimated = true
	default:
		return fmt.Errorf("invalid context_estimated %d", estimated)
	}
	return nil
}

// boolToInt encodes a bool as SQLite's 0/1 INTEGER convention.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
