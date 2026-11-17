package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/quiz"
)

const quizOfferColumns = `id, session_id, message_id, created_at`

// QuizOfferRepository is the SQLite-backed implementation of quiz.OfferRepository.
type QuizOfferRepository struct {
	db *sql.DB
}

// NewQuizOfferRepository creates a QuizOfferRepository backed by db. db must
// already have its migrations applied (see Open).
func NewQuizOfferRepository(db *sql.DB) *QuizOfferRepository {
	return &QuizOfferRepository{db: db}
}

// Create stores the offer, or returns quiz.ErrOfferPending if the session
// already has one.
func (r *QuizOfferRepository) Create(ctx context.Context, offer quiz.Offer) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO quiz_offers (`+quizOfferColumns+`) VALUES (?, ?, ?, ?)`,
		offer.ID, offer.SessionID, offer.MessageID, formatChallengeTime(offer.CreatedAt),
	)
	// A clash on the id is a different failure from the session's UNIQUE one.
	if isUniqueViolation(err, "quiz_offers.session_id") {
		return fmt.Errorf("sqlite: creating quiz offer: %w", quiz.ErrOfferPending)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating quiz offer: %w", err)
	}
	return nil
}

// Get returns the offer, or quiz.ErrOfferNotFound.
func (r *QuizOfferRepository) Get(ctx context.Context, id string) (quiz.Offer, error) {
	return r.queryOne(ctx, `WHERE id = ?`, id)
}

// GetBySession returns the session's pending offer, or quiz.ErrOfferNotFound.
func (r *QuizOfferRepository) GetBySession(ctx context.Context, sessionID string) (quiz.Offer, error) {
	return r.queryOne(ctx, `WHERE session_id = ?`, sessionID)
}

func (r *QuizOfferRepository) queryOne(ctx context.Context, where, arg string) (quiz.Offer, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx, `SELECT `+quizOfferColumns+` FROM quiz_offers `+where, arg)
	var (
		offer     quiz.Offer
		createdAt string
	)
	err := row.Scan(&offer.ID, &offer.SessionID, &offer.MessageID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return quiz.Offer{}, quiz.ErrOfferNotFound
	}
	if err != nil {
		return quiz.Offer{}, fmt.Errorf("sqlite: reading quiz offer: %w", err)
	}
	if offer.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return quiz.Offer{}, fmt.Errorf("sqlite: reading quiz offer: %w", err)
	}
	return offer, nil
}

// Delete removes the offer. An unknown id is not an error.
func (r *QuizOfferRepository) Delete(ctx context.Context, id string) error {
	if _, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM quiz_offers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: deleting quiz offer: %w", err)
	}
	return nil
}
