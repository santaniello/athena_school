package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/santaniello/athena/internal/domain/exercise"
)

const attemptColumns = `id, challenge_id, rubric_version_id, session_exercise_id, state, started_at, deadline, submitted_at, submit_reason, failure_reason, pass_score`

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
		`INSERT INTO challenge_attempts (`+attemptColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		attempt.ID, attempt.ChallengeID, attempt.RubricVersionID, nullIfEmpty(attempt.SessionExerciseID),
		string(attempt.State), formatChallengeTime(attempt.StartedAt), formatChallengeTime(attempt.Deadline),
		nullableTime(attempt.SubmittedAt), nullIfEmpty(string(attempt.SubmitReason)), nullIfEmpty(attempt.FailureReason),
		nullIfZero(attempt.PassScore),
	)
	if isOneInProgressViolation(err) {
		return fmt.Errorf("sqlite: creating attempt: %w", exercise.ErrAnotherAttemptInProgress)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating attempt: %w", err)
	}
	return nil
}

// isOneInProgressViolation reports whether err is the UNIQUE violation of the
// one-running-exam index on challenge_attempts.state. Other UNIQUE violations
// (an id) are different failures.
func isOneInProgressViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "challenge_attempts.state")
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
// one transaction. The attempt must already be SUBMITTED and the submission
// must belong to it, else exercise.ErrInvalidSubmitTransition, so a submission
// can never be stored while the attempt stays in progress. The update only matches a row still IN_PROGRESS, so a
// repeated or concurrent submit returns exercise.ErrAttemptNotInProgress and
// stores nothing.
func (r *AttemptRepository) Submit(ctx context.Context, attempt exercise.ChallengeAttempt, submission exercise.Submission) error {
	if attempt.State != exercise.AttemptSubmitted || submission.AttemptID != attempt.ID {
		return exercise.ErrInvalidSubmitTransition
	}
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
	var sessionExerciseID, submittedAt, submitReason, failureReason sql.NullString
	var passScore sql.NullInt64
	if err := row.Scan(&a.ID, &a.ChallengeID, &a.RubricVersionID, &sessionExerciseID, &state,
		&startedAt, &deadline, &submittedAt, &submitReason, &failureReason, &passScore); err != nil {
		return exercise.ChallengeAttempt{}, err
	}
	a.State = exercise.AttemptState(state)
	a.SessionExerciseID = sessionExerciseID.String
	a.SubmitReason = exercise.SubmitReason(submitReason.String)
	a.FailureReason = failureReason.String
	a.PassScore = int(passScore.Int64)

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

// nullIfZero returns nil, stored as NULL, for zero.
func nullIfZero(value int) any {
	if value == 0 {
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

// DO NOT "fix" the cleanup below to spare EVALUATED attempts. DeleteUnfinished
// and DeleteAllUnfinished delete every attempt that is not FINISHED, EVALUATED
// ones and their evaluations included, on purpose: ADR-005 decision 9 makes the
// evaluation one stage of a sitting that ends with the defense (spec 3.12), so
// an EVALUATED attempt left at startup is an abandoned exam. Spec 3.5 only
// scopes DiscardUnfinishedEvaluations (EVALUATING and EVALUATION_FAILED) and
// does not amend the 3.4 cleanup. This was reviewed and decided on PR #135.

// finishedState is the attempt state spec 3.12 adds; DeleteUnfinished and
// DeleteAllUnfinished never touch it.
const finishedState = string(exercise.AttemptFinished)

// DeleteUnfinished deletes the attempt and its submission in one transaction
// unless it is FINISHED. An unknown id is a no-op.
func (r *AttemptRepository) DeleteUnfinished(ctx context.Context, id string) error {
	err := withTx(ctx, r.db, func(q sqlExecer) error {
		_, err := deleteUnfinished(ctx, q, ` AND id = ?`, id)
		return err
	})
	if err != nil {
		return fmt.Errorf("sqlite: deleting unfinished attempt: %w", err)
	}
	return nil
}

// DeleteAllUnfinished deletes every attempt that is not FINISHED, with its
// submissions, in one transaction, and returns how many attempts it removed.
func (r *AttemptRepository) DeleteAllUnfinished(ctx context.Context) (int, error) {
	var removed int
	err := withTx(ctx, r.db, func(q sqlExecer) error {
		var err error
		removed, err = deleteUnfinished(ctx, q, ``)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("sqlite: deleting unfinished attempts: %w", err)
	}
	return removed, nil
}

// deleteUnfinished deletes the children first, since nothing cascades from
// challenge_attempts, then the attempts themselves. extra is a fixed SQL
// fragment narrowing the selection; its values travel as bound args.
func deleteUnfinished(ctx context.Context, q sqlExecer, extra string, args ...any) (int, error) {
	return deleteAttemptsWhere(ctx, q, `state <> ?`+extra, append([]any{finishedState}, args...)...)
}

// deleteAttemptsWhere deletes the attempts the fixed SQL condition selects,
// children first: criterion results, evaluations and submissions. Its values
// travel as bound args.
func deleteAttemptsWhere(ctx context.Context, q sqlExecer, selection string, args ...any) (int, error) {
	attemptIDs := `SELECT id FROM challenge_attempts WHERE ` + selection
	for _, statement := range []string{
		`DELETE FROM criterion_results WHERE evaluation_id IN (SELECT id FROM evaluations WHERE attempt_id IN (` + attemptIDs + `))`,
		`DELETE FROM evaluations WHERE attempt_id IN (` + attemptIDs + `)`,
		`DELETE FROM submissions WHERE attempt_id IN (` + attemptIDs + `)`,
	} {
		if _, err := q.ExecContext(ctx, statement, args...); err != nil {
			return 0, err
		}
	}
	result, err := q.ExecContext(ctx, `DELETE FROM challenge_attempts WHERE `+selection, args...)
	if err != nil {
		return 0, err
	}
	removed, err := result.RowsAffected()
	return int(removed), err
}

// StartEvaluation moves the attempt to EVALUATING with one conditional UPDATE
// that only matches SUBMITTED or EVALUATION_FAILED, clearing the failure
// reason. If no row changes, including when a concurrent call got there first,
// it returns exercise.ErrAttemptNotEvaluable, so two calls can never both start.
func (r *AttemptRepository) StartEvaluation(ctx context.Context, id string) error {
	changed, err := r.updateState(ctx,
		`UPDATE challenge_attempts SET state = ?, failure_reason = NULL WHERE id = ? AND state IN (?, ?)`,
		string(exercise.AttemptEvaluating), id, string(exercise.AttemptSubmitted), string(exercise.AttemptEvaluationFailed))
	if err != nil {
		return fmt.Errorf("sqlite: starting evaluation: %w", err)
	}
	if !changed {
		return exercise.ErrAttemptNotEvaluable
	}
	return nil
}

// CompleteEvaluation moves an EVALUATING attempt to EVALUATED and freezes the
// minimum passing score on it, else exercise.ErrAttemptNotEvaluating.
func (r *AttemptRepository) CompleteEvaluation(ctx context.Context, id string, passScore int) error {
	changed, err := r.updateState(ctx,
		`UPDATE challenge_attempts SET state = ?, pass_score = ? WHERE id = ? AND state = ?`,
		string(exercise.AttemptEvaluated), passScore, id, string(exercise.AttemptEvaluating))
	if err != nil {
		return fmt.Errorf("sqlite: completing evaluation: %w", err)
	}
	if !changed {
		return exercise.ErrAttemptNotEvaluating
	}
	return nil
}

// FailEvaluation moves an EVALUATING attempt to EVALUATION_FAILED with the
// reason, else exercise.ErrAttemptNotEvaluating.
func (r *AttemptRepository) FailEvaluation(ctx context.Context, id, reason string) error {
	changed, err := r.updateState(ctx,
		`UPDATE challenge_attempts SET state = ?, failure_reason = ? WHERE id = ? AND state = ?`,
		string(exercise.AttemptEvaluationFailed), nullIfEmpty(reason), id, string(exercise.AttemptEvaluating))
	if err != nil {
		return fmt.Errorf("sqlite: failing evaluation: %w", err)
	}
	if !changed {
		return exercise.ErrAttemptNotEvaluating
	}
	return nil
}

// updateState runs a conditional state UPDATE and reports whether a row changed.
func (r *AttemptRepository) updateState(ctx context.Context, query string, args ...any) (bool, error) {
	result, err := execer(ctx, r.db).ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed > 0, err
}

// DeleteUnfinishedEvaluations deletes every attempt left in EVALUATING or
// EVALUATION_FAILED, with its submission, in one transaction, and returns how
// many it removed. Evaluated attempts are untouched.
func (r *AttemptRepository) DeleteUnfinishedEvaluations(ctx context.Context) (int, error) {
	var removed int
	err := withTx(ctx, r.db, func(q sqlExecer) error {
		var err error
		removed, err = deleteAttemptsWhere(ctx, q, `state IN (?, ?)`,
			string(exercise.AttemptEvaluating), string(exercise.AttemptEvaluationFailed))
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("sqlite: deleting unfinished evaluations: %w", err)
	}
	return removed, nil
}
