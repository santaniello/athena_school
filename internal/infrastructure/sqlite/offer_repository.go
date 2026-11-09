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

const offerColumns = `id, session_id, message_id, created_at`

// OfferRepository is the SQLite-backed implementation of
// exercise.OfferRepository.
type OfferRepository struct {
	db *sql.DB
}

// NewOfferRepository creates an OfferRepository backed by db. db must already
// have its migrations applied (see Open).
func NewOfferRepository(db *sql.DB) *OfferRepository {
	return &OfferRepository{db: db}
}

// Create stores the offer, or returns exercise.ErrOfferPending if the session
// already has one.
func (r *OfferRepository) Create(ctx context.Context, offer exercise.Offer) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO exercise_offers (`+offerColumns+`) VALUES (?, ?, ?, ?)`,
		offer.ID, offer.SessionID, offer.MessageID, formatChallengeTime(offer.CreatedAt),
	)
	if isOfferConflict(err) {
		return fmt.Errorf("sqlite: creating exercise offer: %w", exercise.ErrOfferPending)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating exercise offer: %w", err)
	}
	return nil
}

// isOfferConflict reports whether err is the UNIQUE violation of session_id.
// A clash on the id is a different failure.
func isOfferConflict(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) || sqliteErr.Code() != sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return false
	}
	return strings.Contains(sqliteErr.Error(), "exercise_offers.session_id")
}

// Get returns the offer, or exercise.ErrOfferNotFound.
func (r *OfferRepository) Get(ctx context.Context, id string) (exercise.Offer, error) {
	return r.queryOne(ctx, `WHERE id = ?`, id)
}

// GetBySession returns the session's pending offer, or exercise.ErrOfferNotFound.
func (r *OfferRepository) GetBySession(ctx context.Context, sessionID string) (exercise.Offer, error) {
	return r.queryOne(ctx, `WHERE session_id = ?`, sessionID)
}

func (r *OfferRepository) queryOne(ctx context.Context, where string, arg string) (exercise.Offer, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx, `SELECT `+offerColumns+` FROM exercise_offers `+where, arg)
	var offer exercise.Offer
	var createdAt string
	err := row.Scan(&offer.ID, &offer.SessionID, &offer.MessageID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return exercise.Offer{}, exercise.ErrOfferNotFound
	}
	if err != nil {
		return exercise.Offer{}, fmt.Errorf("sqlite: reading exercise offer: %w", err)
	}
	if offer.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return exercise.Offer{}, fmt.Errorf("sqlite: reading exercise offer: %w", err)
	}
	return offer, nil
}

// Delete removes the offer. An unknown id is not an error.
func (r *OfferRepository) Delete(ctx context.Context, id string) error {
	if _, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM exercise_offers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: deleting exercise offer: %w", err)
	}
	return nil
}
