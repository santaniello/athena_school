package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/santaniello/athena/internal/domain/exercise"
)

// AttemptRecordRepository is the SQLite-backed implementation of
// exercise.AttemptRecordRepository. However many challenges it is asked about,
// it runs four queries: attempts, evaluations, criterion results and rubric
// criteria.
type AttemptRecordRepository struct {
	db *sql.DB
}

// NewAttemptRecordRepository creates an AttemptRecordRepository backed by db.
// db must already have its migrations applied (see Open).
func NewAttemptRecordRepository(db *sql.DB) *AttemptRecordRepository {
	return &AttemptRecordRepository{db: db}
}

// ListByChallenges returns the attempts of the challenges, oldest first,
// grouped by challenge id, each with the evaluations it has and, when it has
// any, the rubric criteria it was frozen to.
func (r *AttemptRecordRepository) ListByChallenges(ctx context.Context, challengeIDs []string) (map[string][]exercise.AttemptRecord, error) {
	grouped := map[string][]exercise.AttemptRecord{}
	if len(challengeIDs) == 0 {
		return grouped, nil
	}
	q := execer(ctx, r.db)
	attempts, err := loadAttemptsOf(ctx, q, challengeIDs)
	if err != nil || len(attempts) == 0 {
		return grouped, err
	}
	evaluations, err := loadEvaluationsOf(ctx, q, attempts)
	if err != nil {
		return nil, err
	}
	rubrics, err := loadRubricsOf(ctx, q, attempts, evaluations)
	if err != nil {
		return nil, err
	}
	for _, attempt := range attempts {
		record := exercise.AttemptRecord{Attempt: attempt}
		for _, evaluation := range evaluations[attempt.ID] {
			if evaluation.Kind == exercise.EvaluationKindDiagram {
				record.Diagram = evaluation
			} else {
				record.Understanding = evaluation
			}
		}
		if len(evaluations[attempt.ID]) > 0 {
			record.Rubric = rubrics[attempt.RubricVersionID]
		}
		grouped[attempt.ChallengeID] = append(grouped[attempt.ChallengeID], record)
	}
	return grouped, nil
}

// inPlaceholders returns "?,?,?" for n bound values.
func inPlaceholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func loadAttemptsOf(ctx context.Context, q sqlExecer, challengeIDs []string) ([]exercise.ChallengeAttempt, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+attemptColumns+` FROM challenge_attempts WHERE challenge_id IN (`+inPlaceholders(len(challengeIDs))+`)
		ORDER BY started_at, id`, stringArgs(challengeIDs)...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing attempts of challenges: %w", err)
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

// loadEvaluationsOf returns the attempts' evaluations by attempt id, each with
// its criterion results in rubric order.
func loadEvaluationsOf(ctx context.Context, q sqlExecer, attempts []exercise.ChallengeAttempt) (map[string][]*exercise.Evaluation, error) {
	attemptIDs := make([]string, len(attempts))
	for i, attempt := range attempts {
		attemptIDs[i] = attempt.ID
	}
	rows, err := q.QueryContext(ctx,
		`SELECT id, attempt_id, kind, score FROM evaluations WHERE attempt_id IN (`+inPlaceholders(len(attemptIDs))+`)`,
		stringArgs(attemptIDs)...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing evaluations of attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byAttempt := map[string][]*exercise.Evaluation{}
	byID := map[string]*exercise.Evaluation{}
	evaluationIDs := []string{}
	for rows.Next() {
		var evaluation exercise.Evaluation
		var kind string
		if err := rows.Scan(&evaluation.ID, &evaluation.AttemptID, &kind, &evaluation.Score); err != nil {
			return nil, fmt.Errorf("sqlite: scanning evaluation: %w", err)
		}
		evaluation.Kind = exercise.EvaluationKind(kind)
		evaluation.Results = []exercise.CriterionResult{}
		byAttempt[evaluation.AttemptID] = append(byAttempt[evaluation.AttemptID], &evaluation)
		byID[evaluation.ID] = &evaluation
		evaluationIDs = append(evaluationIDs, evaluation.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating evaluations: %w", err)
	}
	if len(evaluationIDs) == 0 {
		return byAttempt, nil
	}
	return byAttempt, loadResultsInto(ctx, q, byID, evaluationIDs)
}

func loadResultsInto(ctx context.Context, q sqlExecer, byID map[string]*exercise.Evaluation, evaluationIDs []string) error {
	rows, err := q.QueryContext(ctx,
		`SELECT cr.evaluation_id, cr.criterion_id, cr.level, cr.insufficient_evidence
		FROM criterion_results cr JOIN rubric_criteria rc ON rc.id = cr.criterion_id
		WHERE cr.evaluation_id IN (`+inPlaceholders(len(evaluationIDs))+`)
		ORDER BY rc.position`, stringArgs(evaluationIDs)...)
	if err != nil {
		return fmt.Errorf("sqlite: listing criterion results of evaluations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var evaluationID string
		var result exercise.CriterionResult
		if err := rows.Scan(&evaluationID, &result.CriterionID, &result.Level, &result.InsufficientEvidence); err != nil {
			return fmt.Errorf("sqlite: scanning criterion result: %w", err)
		}
		evaluation := byID[evaluationID]
		evaluation.Results = append(evaluation.Results, result)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("sqlite: iterating criterion results: %w", err)
	}
	return nil
}

// loadRubricsOf returns, by version id, the rubric versions of the attempts
// that have evaluations, each with only the criteria an outcome needs: id,
// weight and whether it is mandatory.
func loadRubricsOf(ctx context.Context, q sqlExecer, attempts []exercise.ChallengeAttempt, evaluations map[string][]*exercise.Evaluation) (map[string]*exercise.RubricVersion, error) {
	rubrics := map[string]*exercise.RubricVersion{}
	versionIDs := []string{}
	for _, attempt := range attempts {
		if _, seen := rubrics[attempt.RubricVersionID]; seen || len(evaluations[attempt.ID]) == 0 {
			continue
		}
		rubrics[attempt.RubricVersionID] = &exercise.RubricVersion{ID: attempt.RubricVersionID, ChallengeID: attempt.ChallengeID}
		versionIDs = append(versionIDs, attempt.RubricVersionID)
	}
	if len(versionIDs) == 0 {
		return rubrics, nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT rubric_version_id, id, weight, mandatory FROM rubric_criteria
		WHERE rubric_version_id IN (`+inPlaceholders(len(versionIDs))+`) ORDER BY position`, stringArgs(versionIDs)...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing rubric criteria of versions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var versionID string
		var criterion exercise.RubricCriterion
		if err := rows.Scan(&versionID, &criterion.ID, &criterion.Weight, &criterion.Mandatory); err != nil {
			return nil, fmt.Errorf("sqlite: scanning rubric criterion: %w", err)
		}
		rubrics[versionID].Criteria = append(rubrics[versionID].Criteria, criterion)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterating rubric criteria: %w", err)
	}
	return rubrics, nil
}

func stringArgs(values []string) []any {
	args := make([]any, len(values))
	for i, value := range values {
		args[i] = value
	}
	return args
}
