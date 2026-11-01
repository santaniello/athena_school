package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/exercise"
)

const attemptColumns = `id, challenge_id, rubric_version_id, session_exercise_id, state, started_at, deadline, submitted_at, submit_reason`

// AttemptRepository is the SQLite-backed implementation of
// exercise.AttemptRepository.
type AttemptRepository struct {
	db *sql.DB
}

// NewAttemptRepository creates an AttemptRepository backed by db. db must
// already have its migrations applied (see Open).
func NewAttemptRepository(db *sql.DB) *AttemptRepository {
	return &AttemptRepository{db: db}
}

// Create stores the attempt. An empty SessionExerciseID, a zero SubmittedAt and
// an empty SubmitReason are stored as NULL. A second IN_PROGRESS attempt is
// refused by the database.
func (r *AttemptRepository) Create(ctx context.Context, attempt exercise.ChallengeAttempt) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO challenge_attempts (`+attemptColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.ID, attempt.ChallengeID, attempt.RubricVersionID, nullIfEmpty(attempt.SessionExerciseID),
		string(attempt.State), formatChallengeTime(attempt.StartedAt), formatChallengeTime(attempt.Deadline),
		nullableTime(attempt.SubmittedAt), nullIfEmpty(string(attempt.SubmitReason)),
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating attempt: %w", err)
	}
	return nil
}

// Get returns the attempt, or exercise.ErrAttemptNotFound.
func (r *AttemptRepository) Get(ctx context.Context, id string) (exercise.ChallengeAttempt, error) {
	return r.queryOne(ctx, `WHERE id = ?`, id)
}

// FindInProgress returns the running attempt, or exercise.ErrAttemptNotFound.
func (r *AttemptRepository) FindInProgress(ctx context.Context) (exercise.ChallengeAttempt, error) {
	return r.queryOne(ctx, `WHERE state = ?`, string(exercise.AttemptInProgress))
}

func (r *AttemptRepository) queryOne(ctx context.Context, where, arg string) (exercise.ChallengeAttempt, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx, `SELECT `+attemptColumns+` FROM challenge_attempts `+where, arg)
	attempt, err := scanAttempt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.ChallengeAttempt{}, exercise.ErrAttemptNotFound
	}
	if err != nil {
		return exercise.ChallengeAttempt{}, fmt.Errorf("sqlite: reading attempt: %w", err)
	}
	return attempt, nil
}

// ListByChallenge returns every attempt of the challenge, oldest first.
func (r *AttemptRepository) ListByChallenge(ctx context.Context, challengeID string) ([]exercise.ChallengeAttempt, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+attemptColumns+` FROM challenge_attempts WHERE challenge_id = ? ORDER BY started_at, id`, challengeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	attempts := []exercise.ChallengeAttempt{}
	for rows.Next() {
		attempt, err := scanAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating attempts: %w", err)
	}
	return attempts, nil
}

// Submit moves the attempt to its submitted state and stores the submission in
// one transaction. The update only matches a row still IN_PROGRESS, so a
// repeated or concurrent submit returns exercise.ErrAttemptNotInProgress and
// stores nothing.
func (r *AttemptRepository) Submit(ctx context.Context, attempt exercise.ChallengeAttempt, submission exercise.Submission) error {
	err := withTx(ctx, r.db, func(q sqlExecer) error {
		result, err := q.ExecContext(ctx,
			`UPDATE challenge_attempts SET state = ?, submitted_at = ?, submit_reason = ? WHERE id = ? AND state = ?`,
			string(attempt.State), formatChallengeTime(attempt.SubmittedAt), string(attempt.SubmitReason),
			attempt.ID, string(exercise.AttemptInProgress),
		)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return exercise.ErrAttemptNotInProgress
		}
		_, err = q.ExecContext(ctx,
			`INSERT INTO submissions (id, attempt_id, content_type, content, checksum, submitted_at) VALUES (?, ?, ?, ?, ?, ?)`,
			submission.ID, submission.AttemptID, submission.ContentType, submission.Content,
			submission.Checksum, formatChallengeTime(submission.SubmittedAt),
		)
		return err
	})
	if errors.Is(err, exercise.ErrAttemptNotInProgress) {
		return err
	}
	if err != nil {
		return fmt.Errorf("sqlite: submitting attempt: %w", err)
	}
	return nil
}

func scanAttempt(row rowScanner) (exercise.ChallengeAttempt, error) {
	var a exercise.ChallengeAttempt
	var state, startedAt, deadline string
	var sessionExerciseID, submittedAt, submitReason sql.NullString
	if err := row.Scan(&a.ID, &a.ChallengeID, &a.RubricVersionID, &sessionExerciseID, &state,
		&startedAt, &deadline, &submittedAt, &submitReason); err != nil {
		return exercise.ChallengeAttempt{}, err
	}
	a.State = exercise.AttemptState(state)
	a.SessionExerciseID = sessionExerciseID.String
	a.SubmitReason = exercise.SubmitReason(submitReason.String)

	var err error
	if a.StartedAt, err = parseChallengeTime(startedAt); err != nil {
		return exercise.ChallengeAttempt{}, err
	}
	if a.Deadline, err = parseChallengeTime(deadline); err != nil {
		return exercise.ChallengeAttempt{}, err
	}
	if submittedAt.Valid {
		if a.SubmittedAt, err = parseChallengeTime(submittedAt.String); err != nil {
			return exercise.ChallengeAttempt{}, err
		}
	}
	return a, nil
}

// nullIfEmpty returns nil, stored as NULL, for an empty string.
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nullableTime returns nil, stored as NULL, for the zero time.
func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatChallengeTime(t)
}
