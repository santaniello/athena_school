package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/quiz"
)

// QuizRepository is the SQLite-backed implementation of quiz.Repository.
type QuizRepository struct {
	db *sql.DB
}

// NewQuizRepository creates a QuizRepository backed by db. db must already have
// its migrations applied (see Open).
func NewQuizRepository(db *sql.DB) *QuizRepository {
	return &QuizRepository{db: db}
}

// CreateQuiz stores the quiz and its questions together or not at all.
func (r *QuizRepository) CreateQuiz(ctx context.Context, q quiz.Quiz) error {
	err := withTx(ctx, r.db, func(x sqlExecer) error {
		if _, err := x.ExecContext(ctx,
			`INSERT INTO quizzes (id, session_id, session_title, difficulty, created_at, message_id) VALUES (?, ?, ?, ?, ?, ?)`,
			q.ID, q.SessionID, q.SessionTitle, string(q.Difficulty), formatChallengeTime(q.CreatedAt), nullableText(q.MessageID),
		); err != nil {
			return err
		}
		for _, question := range q.Questions {
			options, err := marshalStringList(question.Options)
			if err != nil {
				return err
			}
			if _, err := x.ExecContext(ctx,
				`INSERT INTO quiz_questions (id, quiz_id, position, kind, prompt, options, correct_index, explanation)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				question.ID, q.ID, question.Position, string(question.Kind), question.Prompt, options, question.CorrectIndex, question.Explanation,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sqlite: creating quiz: %w", err)
	}
	return nil
}

// GetQuiz returns the quiz with its questions in order, or quiz.ErrQuizNotFound.
func (r *QuizRepository) GetQuiz(ctx context.Context, id string) (quiz.Quiz, error) {
	var (
		q                  quiz.Quiz
		difficulty, create string
		messageID          sql.NullString
	)
	err := execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT id, session_id, session_title, difficulty, created_at, message_id FROM quizzes WHERE id = ?`, id,
	).Scan(&q.ID, &q.SessionID, &q.SessionTitle, &difficulty, &create, &messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return quiz.Quiz{}, quiz.ErrQuizNotFound
	}
	if err != nil {
		return quiz.Quiz{}, fmt.Errorf("sqlite: reading quiz: %w", err)
	}
	q.Difficulty = quiz.Difficulty(difficulty)
	q.MessageID = messageID.String
	if q.CreatedAt, err = parseChallengeTime(create); err != nil {
		return quiz.Quiz{}, err
	}
	if q.Questions, err = r.listQuestions(ctx, id); err != nil {
		return quiz.Quiz{}, err
	}
	return q, nil
}

func (r *QuizRepository) listQuestions(ctx context.Context, quizID string) ([]quiz.Question, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT id, position, kind, prompt, options, correct_index, explanation
		 FROM quiz_questions WHERE quiz_id = ? ORDER BY position`, quizID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing quiz questions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var questions []quiz.Question
	for rows.Next() {
		var (
			question quiz.Question
			kind     string
			options  string
		)
		if err := rows.Scan(&question.ID, &question.Position, &kind, &question.Prompt, &options, &question.CorrectIndex, &question.Explanation); err != nil {
			return nil, fmt.Errorf("sqlite: scanning quiz question: %w", err)
		}
		question.Kind = quiz.QuestionKind(kind)
		if question.Options, err = unmarshalStringList(options); err != nil {
			return nil, err
		}
		questions = append(questions, question)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: reading quiz questions: %w", err)
	}
	return questions, nil
}

// summarySelect is the catalog row of a quiz: its attempt count, the score of
// its latest attempt and the chat message that produced it.
const summarySelect = `
		SELECT z.id, z.session_id, z.session_title, z.difficulty, z.created_at,
		       (SELECT COUNT(*) FROM quiz_questions WHERE quiz_id = z.id),
		       (SELECT COUNT(*) FROM quiz_attempts WHERE quiz_id = z.id),
		       (SELECT score FROM quiz_attempts WHERE quiz_id = z.id ORDER BY finished_at DESC, id DESC LIMIT 1),
		       z.message_id
		FROM quizzes z `

// ListQuizzes returns every quiz, newest first, with its attempt count and the
// score of its latest attempt.
func (r *QuizRepository) ListQuizzes(ctx context.Context) ([]quiz.Summary, error) {
	return r.querySummaries(ctx, `ORDER BY z.created_at DESC, z.id DESC`)
}

// FindByMessage returns the quiz the offer under the chat message produced, or
// quiz.ErrQuizNotFound. A quiz from the menu has no message, so it never matches.
func (r *QuizRepository) FindByMessage(ctx context.Context, messageID string) (quiz.Summary, error) {
	summaries, err := r.querySummaries(ctx, `WHERE z.message_id = ? ORDER BY z.created_at ASC, z.id ASC LIMIT 1`, messageID)
	if err != nil {
		return quiz.Summary{}, err
	}
	if len(summaries) == 0 {
		return quiz.Summary{}, quiz.ErrQuizNotFound
	}
	return summaries[0], nil
}

// ListBySession returns the quizzes the session's offers produced, oldest
// first. A quiz from the menu is not one of them.
func (r *QuizRepository) ListBySession(ctx context.Context, sessionID string) ([]quiz.Summary, error) {
	return r.querySummaries(ctx, `WHERE z.session_id = ? AND z.message_id IS NOT NULL ORDER BY z.created_at ASC, z.id ASC`, sessionID)
}

func (r *QuizRepository) querySummaries(ctx context.Context, tail string, args ...any) ([]quiz.Summary, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx, summarySelect+tail, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing quizzes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var summaries []quiz.Summary
	for rows.Next() {
		var (
			summary           quiz.Summary
			difficulty, stamp string
			latest            sql.NullFloat64
			messageID         sql.NullString
		)
		if err := rows.Scan(&summary.ID, &summary.SessionID, &summary.SessionTitle, &difficulty, &stamp,
			&summary.QuestionCount, &summary.AttemptCount, &latest, &messageID); err != nil {
			return nil, fmt.Errorf("sqlite: scanning quiz summary: %w", err)
		}
		summary.Difficulty = quiz.Difficulty(difficulty)
		summary.MessageID = messageID.String
		if summary.CreatedAt, err = parseChallengeTime(stamp); err != nil {
			return nil, err
		}
		if latest.Valid {
			summary.LatestScore = &latest.Float64
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: reading quizzes: %w", err)
	}
	return summaries, nil
}

// DeleteQuiz removes the quiz; its questions, attempts and answers go with it
// through the foreign keys. It returns quiz.ErrQuizNotFound for an unknown ID.
func (r *QuizRepository) DeleteQuiz(ctx context.Context, id string) error {
	result, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM quizzes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("sqlite: deleting quiz: %w", err)
	}
	return requireRowAffected(result, quiz.ErrQuizNotFound)
}

// CreateAttempt stores the attempt and its answers together or not at all.
func (r *QuizRepository) CreateAttempt(ctx context.Context, attempt quiz.Attempt) error {
	err := withTx(ctx, r.db, func(x sqlExecer) error {
		if _, err := x.ExecContext(ctx,
			`INSERT INTO quiz_attempts (id, quiz_id, correct_count, total, score, finished_at) VALUES (?, ?, ?, ?, ?, ?)`,
			attempt.ID, attempt.QuizID, attempt.CorrectCount, attempt.Total, attempt.Score, formatChallengeTime(attempt.FinishedAt),
		); err != nil {
			return err
		}
		for i, answer := range attempt.Answers {
			var selected sql.NullInt64
			if answer.SelectedIndex != nil {
				selected = sql.NullInt64{Int64: int64(*answer.SelectedIndex), Valid: true}
			}
			if _, err := x.ExecContext(ctx,
				`INSERT INTO quiz_answers (attempt_id, question_id, position, selected_index, correct) VALUES (?, ?, ?, ?, ?)`,
				attempt.ID, answer.QuestionID, i+1, selected, answer.Correct,
			); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sqlite: creating quiz attempt: %w", err)
	}
	return nil
}

const quizAttemptColumns = `id, quiz_id, correct_count, total, score, finished_at`

// GetAttempt returns the attempt with its answers in question order, or
// quiz.ErrAttemptNotFound.
func (r *QuizRepository) GetAttempt(ctx context.Context, id string) (quiz.Attempt, error) {
	attempt, err := scanQuizAttempt(execer(ctx, r.db).QueryRowContext(ctx,
		`SELECT `+quizAttemptColumns+` FROM quiz_attempts WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return quiz.Attempt{}, quiz.ErrAttemptNotFound
	}
	if err != nil {
		return quiz.Attempt{}, fmt.Errorf("sqlite: reading quiz attempt: %w", err)
	}
	if attempt.Answers, err = r.listAnswers(ctx, id); err != nil {
		return quiz.Attempt{}, err
	}
	return attempt, nil
}

// ListAttempts returns the attempts of a quiz, newest first, without their
// answers: the list is a summary, and reading the answers of each attempt would
// cost one more query per attempt. GetAttempt returns one with its answers.
func (r *QuizRepository) ListAttempts(ctx context.Context, quizID string) ([]quiz.Attempt, error) {
	return r.queryAttempts(ctx, quizID)
}

func (r *QuizRepository) queryAttempts(ctx context.Context, quizID string) ([]quiz.Attempt, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT `+quizAttemptColumns+` FROM quiz_attempts WHERE quiz_id = ? ORDER BY finished_at DESC, id DESC`, quizID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing quiz attempts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var attempts []quiz.Attempt
	for rows.Next() {
		attempt, err := scanQuizAttempt(rows)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scanning quiz attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: reading quiz attempts: %w", err)
	}
	return attempts, nil
}

func scanQuizAttempt(row rowScanner) (quiz.Attempt, error) {
	var (
		attempt  quiz.Attempt
		finished string
	)
	if err := row.Scan(&attempt.ID, &attempt.QuizID, &attempt.CorrectCount, &attempt.Total, &attempt.Score, &finished); err != nil {
		return quiz.Attempt{}, err
	}
	var err error
	attempt.FinishedAt, err = parseChallengeTime(finished)
	return attempt, err
}

func (r *QuizRepository) listAnswers(ctx context.Context, attemptID string) ([]quiz.Answer, error) {
	rows, err := execer(ctx, r.db).QueryContext(ctx,
		`SELECT question_id, selected_index, correct FROM quiz_answers WHERE attempt_id = ? ORDER BY position`, attemptID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: listing quiz answers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var answers []quiz.Answer
	for rows.Next() {
		var (
			answer   quiz.Answer
			selected sql.NullInt64
		)
		if err := rows.Scan(&answer.QuestionID, &selected, &answer.Correct); err != nil {
			return nil, fmt.Errorf("sqlite: scanning quiz answer: %w", err)
		}
		if selected.Valid {
			index := int(selected.Int64)
			answer.SelectedIndex = &index
		}
		answers = append(answers, answer)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: reading quiz answers: %w", err)
	}
	return answers, nil
}

// nullableText stores an empty string as NULL.
func nullableText(text string) sql.NullString {
	return sql.NullString{String: text, Valid: text != ""}
}
