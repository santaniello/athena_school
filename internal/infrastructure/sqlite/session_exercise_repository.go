package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"github.com/santaniello/athena/internal/domain/exercise"
)

const sessionExerciseColumns = `id, session_id, challenge_id, message_id, created_at`

// SessionExerciseRepository is the SQLite-backed implementation of
// exercise.SessionExerciseRepository.
type SessionExerciseRepository struct {
	db *sql.DB
}

// NewSessionExerciseRepository creates a SessionExerciseRepository backed by
// db. db must already have its migrations applied (see Open).
func NewSessionExerciseRepository(db *sql.DB) *SessionExerciseRepository {
	return &SessionExerciseRepository{db: db}
}

// Create stores the link, or returns exercise.ErrSessionExerciseExists if the
// session is already linked to the challenge.
func (r *SessionExerciseRepository) Create(ctx context.Context, link exercise.SessionExercise) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO session_exercises (`+sessionExerciseColumns+`) VALUES (?, ?, ?, ?, ?)`,
		link.ID, link.SessionID, link.ChallengeID, link.MessageID, formatChallengeTime(link.CreatedAt),
	)
	if isSessionExerciseConflict(err) {
		return fmt.Errorf("sqlite: creating session exercise: %w", exercise.ErrSessionExerciseExists)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating session exercise: %w", err)
	}
	return nil
}

// isSessionExerciseConflict reports whether err is the UNIQUE violation of
// (session_id, challenge_id). A clash on the id is a different failure.
func isSessionExerciseConflict(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "session_exercises.session_id")
}

// GetBySessionAndChallenge returns the link, or exercise.ErrSessionExerciseNotFound.
func (r *SessionExerciseRepository) GetBySessionAndChallenge(ctx context.Context, sessionID, challengeID string) (exercise.SessionExercise, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+sessionExerciseColumns+` FROM session_exercises WHERE session_id = ? AND challenge_id = ?`,
		sessionID, challengeID)
	link, err := scanSessionExercise(row)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.SessionExercise{}, exercise.ErrSessionExerciseNotFound
	}
	if err != nil {
		return exercise.SessionExercise{}, fmt.Errorf("sqlite: reading session exercise: %w", err)
	}
	return link, nil
}

// ListBySession returns the session's links, oldest first.
func (r *SessionExerciseRepository) ListBySession(ctx context.Context, sessionID string) ([]exercise.SessionExercise, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+sessionExerciseColumns+` FROM session_exercises WHERE session_id = ? ORDER BY created_at, id`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing session exercises: %w", err)
	}
	defer func() { _ = rows.Close() }()

	links := []exercise.SessionExercise{}
	for rows.Next() {
		link, err := scanSessionExercise(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning session exercise: %w", err)
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating session exercises: %w", err)
	}
	return links, nil
}

func scanSessionExercise(row rowScanner) (exercise.SessionExercise, error) {
	var link exercise.SessionExercise
	var createdAt string
	if err := row.Scan(&link.ID, &link.SessionID, &link.ChallengeID, &link.MessageID, &createdAt); err != nil {
		return exercise.SessionExercise{}, err
	}
	var err error
	if link.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return exercise.SessionExercise{}, err
	}
	return link, nil
}
