package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// challengeTimeLayout is fixed-width (nine fractional digits, UTC) so that
// ordering the TEXT columns lexicographically is ordering them by time.
const challengeTimeLayout = "2006-01-02T15:04:05.000000000Z"

const challengeColumns = `id, type, kind, title, scenario, functional_requirements, non_functional_requirements,
	constraints, objectives, difficulty, topic, source_session_id, origin, time_limit_minutes, defense_questions, created_at, updated_at`

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
		`INSERT INTO challenges (`+challengeColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		challenge.ID, string(challenge.Type), string(challenge.Kind), challenge.Title, challenge.Scenario,
		lists[0], lists[1], lists[2], lists[3],
		string(challenge.Difficulty), challenge.Topic, sourceSessionID, string(challenge.Origin),
		challenge.TimeLimitMinutes, challenge.DefenseQuestions,
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

// FindLatestBySourceSession returns the most recently created challenge whose
// source session is sessionID, or exercise.ErrChallengeNotFound.
func (r *ChallengeRepository) FindLatestBySourceSession(ctx context.Context, sessionID string) (exercise.Challenge, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges WHERE source_session_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, sessionID)
	challenge, err := scanChallenge(row)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Challenge{}, exercise.ErrChallengeNotFound
	}
	if err != nil {
		return exercise.Challenge{}, fmt.Errorf("sqlite: finding challenge by source session: %w", err)
	}
	return challenge, nil
}

// List returns one page of the challenges matching filter (all non-empty
// fields combined with AND), newest update first, ties broken by id, plus the
// total number of matches. Only fixed SQL fragments are concatenated; every
// user-supplied value travels as a bound argument.
func (r *ChallengeRepository) List(ctx context.Context, filter exercise.ChallengeFilter, page exercise.Page) (exercise.ChallengePage, error) {
	page = page.Normalize()
	where, args := challengeWhere(filter)

	var total int
	if err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM challenges`+where, args...,
	).Scan(&total); err != nil {
		return exercise.ChallengePage{}, fmt.Errorf("sqlite: counting challenges: %w", err)
	}

	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+challengeColumns+` FROM challenges`+where+` ORDER BY updated_at DESC, id LIMIT ? OFFSET ?`,
		append(args, page.Limit, page.Offset)...,
	)
	if err != nil {
		return exercise.ChallengePage{}, fmt.Errorf("sqlite: listing challenges: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := []exercise.Challenge{}
	for rows.Next() {
		challenge, err := scanChallenge(rows)
		if err != nil {
			return exercise.ChallengePage{}, fmt.Errorf("sqlite: scanning challenge: %w", err)
		}
		items = append(items, challenge)
	}
	if err := rows.Err(); err != nil {
		return exercise.ChallengePage{}, fmt.Errorf("sqlite: iterating challenges: %w", err)
	}
	return exercise.ChallengePage{Items: items, Total: total}, nil
}

// ListTopics returns the distinct topics of all challenges, alphabetically.
func (r *ChallengeRepository) ListTopics(ctx context.Context) ([]string, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, `SELECT DISTINCT topic FROM challenges ORDER BY topic`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing challenge topics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	topics := []string{}
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, fmt.Errorf("sqlite: scanning challenge topic: %w", err)
		}
		topics = append(topics, topic)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating challenge topics: %w", err)
	}
	return topics, nil
}

// challengeWhere builds the WHERE clause (with its leading space, or empty)
// and bound arguments for filter.
func challengeWhere(filter exercise.ChallengeFilter) (string, []any) {
	var conditions []string
	var args []any
	if query := strings.TrimSpace(filter.Query); query != "" {
		pattern := "%" + escapeLike(strings.ToLower(query)) + "%"
		conditions = append(conditions, `(LOWER(title) LIKE ? ESCAPE '\' OR LOWER(scenario) LIKE ? ESCAPE '\' OR LOWER(topic) LIKE ? ESCAPE '\')`)
		args = append(args, pattern, pattern, pattern)
	}
	if topic := exercise.NormalizeTopic(filter.Topic); topic != "" {
		conditions = append(conditions, `topic = ?`)
		args = append(args, topic)
	}
	if filter.Difficulty != "" {
		conditions = append(conditions, `difficulty = ?`)
		args = append(args, string(filter.Difficulty))
	}
	if filter.Origin != "" {
		conditions = append(conditions, `origin = ?`)
		args = append(args, string(filter.Origin))
	}
	if len(conditions) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conditions, " AND "), args
}

// escapeLike makes %, _ and the escape character itself literal in a LIKE
// pattern used with ESCAPE '\'.
func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
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
	var challengeType, kind, difficulty, origin, createdAt, updatedAt string
	var lists [4]string
	var sourceSessionID sql.NullString
	err := row.Scan(
		&c.ID, &challengeType, &kind, &c.Title, &c.Scenario, &lists[0], &lists[1], &lists[2], &lists[3],
		&difficulty, &c.Topic, &sourceSessionID, &origin, &c.TimeLimitMinutes, &c.DefenseQuestions, &createdAt, &updatedAt,
	)
	if err != nil {
		return exercise.Challenge{}, err
	}
	c.Type = exercise.ChallengeType(challengeType)
	c.Kind = exercise.ChallengeKind(kind)
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
