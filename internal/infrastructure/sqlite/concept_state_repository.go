package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

// conceptTimeLayout is fixed-width UTC, so ordering updated_at as text orders
// it as time.
const conceptTimeLayout = "2006-01-02T15:04:05.000000000Z"

// ConceptStateRepository is the SQLite-backed implementation of
// domainstudy.ConceptStateRepository.
type ConceptStateRepository struct {
	db *sql.DB
}

// NewConceptStateRepository creates a ConceptStateRepository backed by db. db
// must already have its migrations applied (see Open).
func NewConceptStateRepository(db *sql.DB) *ConceptStateRepository {
	return &ConceptStateRepository{db: db}
}

// Upsert inserts or replaces the row identified by (SessionID, Name).
func (r *ConceptStateRepository) Upsert(ctx context.Context, state domainstudy.ConceptState) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `
		INSERT INTO session_concepts (session_id, name, status, last_strategy, assistance_count, evidence_count, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, name) DO UPDATE SET
			status = excluded.status,
			last_strategy = excluded.last_strategy,
			assistance_count = excluded.assistance_count,
			evidence_count = excluded.evidence_count,
			updated_at = excluded.updated_at`,
		state.SessionID, state.Name, string(state.Status), string(state.LastStrategy),
		state.AssistanceCount, state.EvidenceCount, state.UpdatedAt.UTC().Format(conceptTimeLayout),
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving session concept: %w", err)
	}
	return nil
}

// ListBySession returns the session's concepts, oldest update first.
func (r *ConceptStateRepository) ListBySession(ctx context.Context, sessionID string) ([]domainstudy.ConceptState, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, `
		SELECT session_id, name, status, last_strategy, assistance_count, evidence_count, updated_at
		FROM session_concepts WHERE session_id = ? ORDER BY updated_at, name`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing session concepts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var states []domainstudy.ConceptState
	for rows.Next() {
		var state domainstudy.ConceptState
		var status, strategy, updatedAt string
		if err := rows.Scan(&state.SessionID, &state.Name, &status, &strategy,
			&state.AssistanceCount, &state.EvidenceCount, &updatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning session concept: %w", err)
		}
		state.Status = domainstudy.ConceptStatus(status)
		state.LastStrategy = domainstudy.TutorStrategy(strategy)
		if state.UpdatedAt, err = time.Parse(conceptTimeLayout, updatedAt); err != nil {
			return nil, fmt.Errorf("sqlite: parsing session concept time: %w", err)
		}
		states = append(states, state)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: listing session concepts: %w", err)
	}
	return states, nil
}

// DeleteOldest removes one concept of the session other than keep: the oldest
// DEMONSTRATED one, else the oldest overall. With no candidate it deletes
// nothing.
func (r *ConceptStateRepository) DeleteOldest(ctx context.Context, sessionID, keep string) error {
	_, err := execer(ctx, r.db).ExecContext(ctx, `
		DELETE FROM session_concepts WHERE session_id = ? AND name = (
			SELECT name FROM session_concepts WHERE session_id = ? AND name <> ?
			ORDER BY (status = 'DEMONSTRATED') DESC, updated_at, name LIMIT 1
		)`, sessionID, sessionID, keep)
	if err != nil {
		return fmt.Errorf("sqlite: evicting session concept: %w", err)
	}
	return nil
}
