package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// challengeTimeLayout is fixed-width (nine fractional digits, UTC) so that
// ordering the TEXT columns lexicographically is ordering them by time.
const challengeTimeLayout = "2006-01-02T15:04:05.000000000Z"

const challengeColumns = `id, type, title, scenario, functional_requirements, non_functional_requirements,
	constraints, objectives, difficulty, topic, source_session_id, origin, created_at, updated_at`

// ChallengeRepository is the SQLite-backed implementation of
// exercise.ChallengeRepository.
type ChallengeRepository struct {
	db *sql.DB
}

// NewChallengeRepository creates a ChallengeRepository backed by db. db must
// already have its migrations applied (see Open).
func NewChallengeRepository(db *sql.DB) *ChallengeRepository {
	return &ChallengeRepository{db: db}
}

// Create inserts a new challenge. An empty SourceSessionID is stored as NULL.
func (r *ChallengeRepository) Create(ctx context.Context, challenge exercise.Challenge) error {
	lists, err := encodeChallengeLists(challenge)
	if err != nil {
		return err
	}
	var sourceSessionID any
	if challenge.SourceSessionID != "" {
		sourceSessionID = challenge.SourceSessionID
	}
	_, err = execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO challenges (`+challengeColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ID, string(challenge.Type), challenge.Title, challenge.Scenario,
		lists[0], lists[1], lists[2], lists[3],
		string(challenge.Difficulty), challenge.Topic, sourceSessionID, string(challenge.Origin),
		formatChallengeTime(challenge.CreatedAt), formatChallengeTime(challenge.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("sqlite: creating challenge: %w", err)
	}
	return nil
}

// Get returns the challenge with the given id, or exercise.ErrChallengeNotFound.
func (r *ChallengeRepository) Get(ctx context.Context, id string) (exercise.Challenge, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges WHERE id = ?`, id)
	challenge, err := scanChallenge(row)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Challenge{}, exercise.ErrChallengeNotFound
	}
	if err != nil {
		return exercise.Challenge{}, fmt.Errorf("sqlite: getting challenge: %w", err)
	}
	return challenge, nil
}

// encodeChallengeLists returns the functional, non-functional, constraints
// and objectives lists, in that order, as JSON text.
func encodeChallengeLists(challenge exercise.Challenge) ([4]string, error) {
	var encoded [4]string
	lists := [4][]string{
		challenge.FunctionalRequirements, challenge.NonFunctionalRequirements,
		challenge.Constraints, challenge.Objectives,
	}
	for i, list := range lists {
		text, err := marshalStringList(list)
		if err != nil {
			return encoded, err
		}
		encoded[i] = text
	}
	return encoded, nil
}

func scanChallenge(row rowScanner) (exercise.Challenge, error) {
	var c exercise.Challenge
	var challengeType, difficulty, origin, createdAt, updatedAt string
	var lists [4]string
	var sourceSessionID sql.NullString
	err := row.Scan(
		&c.ID, &challengeType, &c.Title, &c.Scenario, &lists[0], &lists[1], &lists[2], &lists[3],
		&difficulty, &c.Topic, &sourceSessionID, &origin, &createdAt, &updatedAt,
	)
	if err != nil {
		return exercise.Challenge{}, err
	}
	c.Type = exercise.ChallengeType(challengeType)
	c.Difficulty = exercise.Difficulty(difficulty)
	c.Origin = exercise.ChallengeOrigin(origin)
	c.SourceSessionID = sourceSessionID.String

	for i, target := range []*[]string{&c.FunctionalRequirements, &c.NonFunctionalRequirements, &c.Constraints, &c.Objectives} {
		if *target, err = unmarshalStringList(lists[i]); err != nil {
			return exercise.Challenge{}, err
		}
	}
	if c.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return exercise.Challenge{}, err
	}
	if c.UpdatedAt, err = parseChallengeTime(updatedAt); err != nil {
		return exercise.Challenge{}, err
	}
	return c, nil
}

func formatChallengeTime(t time.Time) string {
	return t.UTC().Format(challengeTimeLayout)
}

func parseChallengeTime(raw string) (time.Time, error) {
	t, err := time.Parse(challengeTimeLayout, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("sqlite: decoding challenge time: %w", err)
	}
	return t, nil
}
