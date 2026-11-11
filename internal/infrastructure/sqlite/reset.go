package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

// Resetter is the SQLite-backed implementation of reset.Resetter.
type Resetter struct {
	db *sql.DB
}

// NewResetter creates a Resetter backed by db. db must already have its
// migrations applied (see Open).
func NewResetter(db *sql.DB) *Resetter {
	return &Resetter{db: db}
}

// Reset permanently deletes every study session (and its messages), every
// folder, every knowledge-domain row and every Architecture Lab row
// (challenges, rubrics, attempts, submissions, evaluations, hints, defense
// turns, session links and pending offers), atomically. usage survives
// detached (session_id set NULL by its own foreign key),
// matching how deleting one session already behaves — see
// TestOpen_migratesLegacyForeignKeysAndDetachesUsageWithoutRemovingIt.
// Knowledge rows already leave through the sessions' ON DELETE CASCADE; the
// explicit deletes below only make the reset independent of that.
func (r *Resetter) Reset(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: beginning reset transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	statements := []struct {
		query string
		args  []any
	}{
		// The Architecture Lab goes first and children before parents: no
		// Phase 3 table cascades (a challenge is never deleted by anything
		// else), so each level is cleared explicitly.
		{`DELETE FROM defense_turns`, nil},
		{`DELETE FROM attempt_hints`, nil},
		{`DELETE FROM criterion_results`, nil},
		{`DELETE FROM evaluations`, nil},
		{`DELETE FROM submissions`, nil},
		{`DELETE FROM challenge_attempts`, nil},
		{`DELETE FROM session_exercises`, nil},
		{`DELETE FROM exercise_offers`, nil},
		{`DELETE FROM rubric_criteria`, nil},
		{`DELETE FROM rubric_versions`, nil},
		{`DELETE FROM challenges`, nil},
		{`DELETE FROM sessions`, nil},
		{`DELETE FROM folders`, nil},
		{`DELETE FROM knowledge_items`, nil},
		{`DELETE FROM knowledge_chunks`, nil},
		{`DELETE FROM ingested_files`, nil},
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return fmt.Errorf("sqlite: resetting local data: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: committing reset transaction: %w", err)
	}
	committed = true
	return nil
}
