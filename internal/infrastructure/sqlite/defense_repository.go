package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// DefenseRepository is the SQLite-backed implementation of
// exercise.DefenseRepository. Turns have no delete: they are removed only with
// their unfinished attempt (see AttemptRepository).
type DefenseRepository struct {
	db *sql.DB
}

// NewDefenseRepository creates a DefenseRepository backed by db. db must
// already have its migrations applied (see Open).
func NewDefenseRepository(db *sql.DB) *DefenseRepository {
	return &DefenseRepository{db: db}
}

// AddQuestion stores an unanswered turn. UNIQUE (attempt_id, position) refuses
// a position the attempt already has, even under concurrent calls.
func (r *DefenseRepository) AddQuestion(ctx context.Context, turn exercise.DefenseTurn) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO defense_turns (id, attempt_id, position, criterion_id, question, asked_at) VALUES (?, ?, ?, ?, ?, ?)`,
		turn.ID, turn.AttemptID, turn.Position, turn.CriterionID, turn.Question, formatChallengeTime(turn.AskedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: adding defense question: %w", err)
	}
	return nil
}

// SaveAnswer stores the answer of a turn that has none yet. A turn already
// answered, or unknown, is exercise.ErrNoQuestionPending.
func (r *DefenseRepository) SaveAnswer(ctx context.Context, turnID, answer string, answeredAt time.Time) error {
	result, err := execer(ctx, r.db).ExecContext(ctx,
		`UPDATE defense_turns SET answer = ?, answered_at = ? WHERE id = ? AND answered_at IS NULL`,
		answer, formatChallengeTime(answeredAt), turnID,
	)
	if err != nil {
		return fmt.Errorf("sqlite: saving defense answer: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite: saving defense answer: %w", err)
	}
	if changed == 0 {
		return exercise.ErrNoQuestionPending
	}
	return nil
}

// ListByAttempt returns the attempt's turns in the order asked.
func (r *DefenseRepository) ListByAttempt(ctx context.Context, attemptID string) ([]exercise.DefenseTurn, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, attempt_id, position, criterion_id, question, answer, asked_at, answered_at
		FROM defense_turns WHERE attempt_id = ? ORDER BY position`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing defense turns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	turns := []exercise.DefenseTurn{}
	for rows.Next() {
		turn, err := scanDefenseTurn(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning defense turn: %w", err)
		}
		turns = append(turns, turn)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating defense turns: %w", err)
	}
	return turns, nil
}

func scanDefenseTurn(row rowScanner) (exercise.DefenseTurn, error) {
	var turn exercise.DefenseTurn
	var askedAt string
	var answeredAt sql.NullString
	if err := row.Scan(&turn.ID, &turn.AttemptID, &turn.Position, &turn.CriterionID,
		&turn.Question, &turn.Answer, &askedAt, &answeredAt); err != nil {
		return exercise.DefenseTurn{}, err
	}
	var err error
	if turn.AskedAt, err = parseChallengeTime(askedAt); err != nil {
		return exercise.DefenseTurn{}, err
	}
	if answeredAt.Valid {
		if turn.AnsweredAt, err = parseChallengeTime(answeredAt.String); err != nil {
			return exercise.DefenseTurn{}, err
		}
	}
	return turn, nil
}
