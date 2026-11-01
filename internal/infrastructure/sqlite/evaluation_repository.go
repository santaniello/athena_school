package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// EvaluationRepository is the SQLite-backed implementation of
// exercise.EvaluationRepository. Evaluations are final: it has no update and
// no delete; they are removed only with their unfinished attempt (see
// AttemptRepository).
type EvaluationRepository struct {
	db *sql.DB
}

// NewEvaluationRepository creates an EvaluationRepository backed by db. db
// must already have its migrations applied (see Open).
func NewEvaluationRepository(db *sql.DB) *EvaluationRepository {
	return &EvaluationRepository{db: db}
}

// Create stores the evaluation and its criterion results in one transaction,
// or inside the one already on ctx. An attempt has one evaluation per kind.
func (r *EvaluationRepository) Create(ctx context.Context, evaluation exercise.Evaluation) error {
	strengths, err := marshalStringList(evaluation.Strengths)
	if err != nil {
		return err
	}
	gaps, err := marshalStringList(evaluation.Gaps)
	if err != nil {
		return err
	}
	recommendations, err := marshalStringList(evaluation.Recommendations)
	if err != nil {
		return err
	}
	err = withTx(ctx, r.db, func(q sqlExecer) error {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO evaluations (id, attempt_id, kind, model, score, summary, strengths, gaps, recommendations, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			evaluation.ID, evaluation.AttemptID, string(evaluation.Kind), evaluation.Model, evaluation.Score,
			evaluation.Summary, strengths, gaps, recommendations, formatChallengeTime(evaluation.CreatedAt),
		); err != nil {
			return err
		}
		for _, result := range evaluation.Results {
			if _, err := q.ExecContext(ctx,
				`INSERT INTO criterion_results (evaluation_id, criterion_id, level, justification, evidence, insufficient_evidence)
				VALUES (?, ?, ?, ?, ?, ?)`,
				evaluation.ID, result.CriterionID, result.Level, result.Justification, result.Evidence, result.InsufficientEvidence,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sqlite: creating evaluation: %w", err)
	}
	return nil
}

// GetByAttempt returns the attempt's evaluation of the kind, with its results
// in rubric order, or exercise.ErrEvaluationNotFound.
func (r *EvaluationRepository) GetByAttempt(ctx context.Context, attemptID string, kind exercise.EvaluationKind) (exercise.Evaluation, error) {
	q := execer(ctx, r.db)
	var e exercise.Evaluation
	var kindText, strengths, gaps, recommendations, createdAt string
	err := q.QueryRowContext(ctx,
		`SELECT id, attempt_id, kind, model, score, summary, strengths, gaps, recommendations, created_at
		FROM evaluations WHERE attempt_id = ? AND kind = ?`, attemptID, string(kind),
	).Scan(&e.ID, &e.AttemptID, &kindText, &e.Model, &e.Score, &e.Summary, &strengths, &gaps, &recommendations, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Evaluation{}, exercise.ErrEvaluationNotFound
	}
	if err != nil {
		return exercise.Evaluation{}, fmt.Errorf("sqlite: reading evaluation: %w", err)
	}
	e.Kind = exercise.EvaluationKind(kindText)
	if e.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return exercise.Evaluation{}, err
	}
	if e.Strengths, err = unmarshalStringList(strengths); err != nil {
		return exercise.Evaluation{}, err
	}
	if e.Gaps, err = unmarshalStringList(gaps); err != nil {
		return exercise.Evaluation{}, err
	}
	if e.Recommendations, err = unmarshalStringList(recommendations); err != nil {
		return exercise.Evaluation{}, err
	}
	if e.Results, err = r.listResults(ctx, q, e.ID); err != nil {
		return exercise.Evaluation{}, err
	}
	return e, nil
}

// listResults returns the evaluation's criterion results in rubric order.
func (r *EvaluationRepository) listResults(ctx context.Context, q sqlExecer, evaluationID string) ([]exercise.CriterionResult, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT cr.criterion_id, cr.level, cr.justification, cr.evidence, cr.insufficient_evidence
		FROM criterion_results cr JOIN rubric_criteria rc ON rc.id = cr.criterion_id
		WHERE cr.evaluation_id = ? ORDER BY rc.position`, evaluationID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing criterion results: %w", err)
	}
	defer func() { _ = rows.Close() }()

	results := []exercise.CriterionResult{}
	for rows.Next() {
		var result exercise.CriterionResult
		if err := rows.Scan(&result.CriterionID, &result.Level, &result.Justification, &result.Evidence, &result.InsufficientEvidence); err != nil {
			return nil, fmt.Errorf("sqlite: scanning criterion result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating criterion results: %w", err)
	}
	return results, nil
}
