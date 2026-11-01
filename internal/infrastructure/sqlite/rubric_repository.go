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

const rubricVersionColumns = `id, challenge_id, version, base_version, created_at`

const rubricCriterionColumns = `id, key, name, description, weight, source, mandatory, levels`

// RubricRepository is the SQLite-backed implementation of
// exercise.RubricRepository. It has no update or delete: a stored rubric
// version is immutable.
type RubricRepository struct {
	db *sql.DB
}

// NewRubricRepository creates a RubricRepository backed by db. db must
// already have its migrations applied (see Open).
func NewRubricRepository(db *sql.DB) *RubricRepository {
	return &RubricRepository{db: db}
}

// Create stores the version and its criteria in one transaction, or joins the
// transaction already on ctx (see SQLTransactor). Criteria keep their slice
// order as their position.
func (r *RubricRepository) Create(ctx context.Context, version exercise.RubricVersion) error {
	levels := make([]string, len(version.Criteria))
	for i, criterion := range version.Criteria {
		encoded, err := marshalStringList(criterion.Levels[:])
		if err != nil {
			return err
		}
		levels[i] = encoded
	}

	err := r.inTx(ctx, func(q sqlExecer) error {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO rubric_versions (`+rubricVersionColumns+`) VALUES (?, ?, ?, ?, ?)`,
			version.ID, version.ChallengeID, version.Version, version.BaseVersion, formatChallengeTime(version.CreatedAt),
		); err != nil {
			return err
		}
		for position, criterion := range version.Criteria {
			if _, err := q.ExecContext(ctx,
				`INSERT INTO rubric_criteria (id, rubric_version_id, position, key, name, description, weight, source, mandatory, levels)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				criterion.ID, version.ID, position, criterion.Key, criterion.Name, criterion.Description,
				criterion.Weight, string(criterion.Source), criterion.Mandatory, levels[position],
			); err != nil {
				return err
			}
		}
		return nil
	})
	if isRubricVersionConflict(err) {
		return fmt.Errorf("sqlite: creating rubric version: %w", exercise.ErrRubricVersionConflict)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating rubric version: %w", err)
	}
	return nil
}

// isRubricVersionConflict reports whether err is the UNIQUE violation of
// (challenge_id, version) on rubric_versions. Other UNIQUE violations (an
// id or a criterion key) are different failures and are not conflicts.
func isRubricVersionConflict(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "rubric_versions.challenge_id")
}

// Get returns the version with its criteria, or exercise.ErrRubricNotFound.
func (r *RubricRepository) Get(ctx context.Context, versionID string) (exercise.RubricVersion, error) {
	return r.queryOne(ctx, `WHERE id = ?`, versionID)
}

// Latest returns the challenge's highest version number, or
// exercise.ErrRubricNotFound if it has no rubric.
func (r *RubricRepository) Latest(ctx context.Context, challengeID string) (exercise.RubricVersion, error) {
	return r.queryOne(ctx, `WHERE challenge_id = ? ORDER BY version DESC LIMIT 1`, challengeID)
}

// ListVersions returns every version of the challenge's rubric, oldest first,
// each with its criteria.
func (r *RubricRepository) ListVersions(ctx context.Context, challengeID string) ([]exercise.RubricVersion, error) {
	// The pool holds a single connection (see Open), so the version rows must
	// be fully read and closed before the criteria queries below.
	versions, err := r.listVersionRows(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		if versions[i].Criteria, err = r.loadCriteria(ctx, versions[i].ID); err != nil {
			return nil, err
		}
	}
	return versions, nil
}

func (r *RubricRepository) listVersionRows(ctx context.Context, challengeID string) ([]exercise.RubricVersion, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+rubricVersionColumns+` FROM rubric_versions WHERE challenge_id = ? ORDER BY version`, challengeID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing rubric versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	versions := []exercise.RubricVersion{}
	for rows.Next() {
		version, err := scanRubricVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning rubric version: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating rubric versions: %w", err)
	}
	return versions, nil
}

// queryOne reads the single version selected by the fixed SQL fragment
// suffix (a WHERE/ORDER BY/LIMIT clause with one bound argument).
func (r *RubricRepository) queryOne(ctx context.Context, suffix, arg string) (exercise.RubricVersion, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+rubricVersionColumns+` FROM rubric_versions `+suffix, arg)
	version, err := scanRubricVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.RubricVersion{}, exercise.ErrRubricNotFound
	}
	if err != nil {
		return exercise.RubricVersion{}, fmt.Errorf("sqlite: reading rubric version: %w", err)
	}
	if version.Criteria, err = r.loadCriteria(ctx, version.ID); err != nil {
		return exercise.RubricVersion{}, err
	}
	return version, nil
}

func (r *RubricRepository) loadCriteria(ctx context.Context, versionID string) ([]exercise.RubricCriterion, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+rubricCriterionColumns+` FROM rubric_criteria WHERE rubric_version_id = ? ORDER BY position`, versionID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing rubric criteria: %w", err)
	}
	defer func() { _ = rows.Close() }()

	criteria := []exercise.RubricCriterion{}
	for rows.Next() {
		criterion, err := scanRubricCriterion(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning rubric criterion: %w", err)
		}
		criteria = append(criteria, criterion)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating rubric criteria: %w", err)
	}
	return criteria, nil
}

// inTx runs fn in the transaction already on ctx, or in a new one that
// commits when fn succeeds and rolls back otherwise.
func (r *RubricRepository) inTx(ctx context.Context, fn func(sqlExecer) error) error {
	if tx, ok := ctx.Value(txKey{}).(*sql.Tx); ok {
		return fn(tx)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func scanRubricVersion(row rowScanner) (exercise.RubricVersion, error) {
	var version exercise.RubricVersion
	var createdAt string
	if err := row.Scan(&version.ID, &version.ChallengeID, &version.Version, &version.BaseVersion, &createdAt); err != nil {
		return exercise.RubricVersion{}, err
	}
	var err error
	if version.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return exercise.RubricVersion{}, err
	}
	return version, nil
}

func scanRubricCriterion(row rowScanner) (exercise.RubricCriterion, error) {
	var criterion exercise.RubricCriterion
	var source, levels string
	if err := row.Scan(&criterion.ID, &criterion.Key, &criterion.Name, &criterion.Description,
		&criterion.Weight, &source, &criterion.Mandatory, &levels); err != nil {
		return exercise.RubricCriterion{}, err
	}
	criterion.Source = exercise.CriterionSource(source)

	decoded, err := unmarshalStringList(levels)
	if err != nil {
		return exercise.RubricCriterion{}, err
	}
	if len(decoded) != exercise.LevelCount {
		return exercise.RubricCriterion{}, fmt.Errorf("sqlite: rubric criterion %q has %d levels, want %d", criterion.Key, len(decoded), exercise.LevelCount)
	}
	copy(criterion.Levels[:], decoded)
	return criterion, nil
}
