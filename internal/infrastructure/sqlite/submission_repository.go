package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// SubmissionRepository is the SQLite-backed implementation of
// exercise.SubmissionRepository. Submissions are written only by
// AttemptRepository.Submit, so it only reads.
type SubmissionRepository struct {
	db *sql.DB
}

// NewSubmissionRepository creates a SubmissionRepository backed by db. db must
// already have its migrations applied (see Open).
func NewSubmissionRepository(db *sql.DB) *SubmissionRepository {
	return &SubmissionRepository{db: db}
}

// GetByAttempt returns the attempt's submission, or exercise.ErrSubmissionNotFound.
func (r *SubmissionRepository) GetByAttempt(ctx context.Context, attemptID string) (exercise.Submission, error) {
	var s exercise.Submission
	var submittedAt string
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, attempt_id, content_type, content, checksum, submitted_at FROM submissions WHERE attempt_id = ?`, attemptID,
	).Scan(&s.ID, &s.AttemptID, &s.ContentType, &s.Content, &s.Checksum, &submittedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Submission{}, exercise.ErrSubmissionNotFound
	}
	if err != nil {
		return exercise.Submission{}, fmt.Errorf("sqlite: reading submission: %w", err)
	}
	if s.SubmittedAt, err = parseChallengeTime(submittedAt); err != nil {
		return exercise.Submission{}, err
	}
	return s, nil
}
