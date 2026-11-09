package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// HintRepository is the SQLite-backed implementation of exercise.HintRepository.
type HintRepository struct {
	db *sql.DB
}

// NewHintRepository creates a HintRepository backed by db. db must already
// have its migrations applied (see Open).
func NewHintRepository(db *sql.DB) *HintRepository {
	return &HintRepository{db: db}
}

// Add stores the hint with one statement that inserts only while the attempt
// has fewer than exercise.MaxHints, so concurrent calls can never exceed the
// cap. A full attempt is exercise.ErrNoHintsLeft; a position another call
// already took is exercise.ErrHintInProgress.
func (r *HintRepository) Add(ctx context.Context, hint exercise.Hint) error {
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO attempt_hints (id, attempt_id, position, content, created_at)
		 SELECT ?, ?, ?, ?, ?
		 WHERE (SELECT COUNT(*) FROM attempt_hints WHERE attempt_id = ?) < ?`,
		hint.ID, hint.AttemptID, hint.Position, hint.Content, formatChallengeTime(hint.CreatedAt),
		hint.AttemptID, exercise.MaxHints,
	)
	if isHintPositionViolation(err) {
		return fmt.Errorf("sqlite: adding hint: %w", exercise.ErrHintInProgress)
	}
	if err != nil {
		return fmt.Errorf("sqlite: adding hint: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: adding hint: %w", err)
	}
	if changed == 0 {
		return fmt.Errorf("sqlite: adding hint: %w", exercise.ErrNoHintsLeft)
	}
	return nil
}

// isHintPositionViolation reports whether err is the UNIQUE violation of
// (attempt_id, position) on attempt_hints.
func isHintPositionViolation(err error) bool {
	return isUniqueViolation(err, "attempt_hints.position")
}

// ListByAttempt returns the attempt's hints in the order given.
func (r *HintRepository) ListByAttempt(ctx context.Context, attemptID string) ([]exercise.Hint, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, attempt_id, position, content, created_at FROM attempt_hints WHERE attempt_id = ? ORDER BY position`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing hints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	hints := []exercise.Hint{}
	for rows.Next() {
		var hint exercise.Hint
		var createdAt string
		if err := rows.Scan(&hint.ID, &hint.AttemptID, &hint.Position, &hint.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("sqlite: scanning hint: %w", err)
		}
		if hint.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
			return nil, err
		}
		hints = append(hints, hint)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating hints: %w", err)
	}
	return hints, nil
}
