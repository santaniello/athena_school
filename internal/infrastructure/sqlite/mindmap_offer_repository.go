package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/domain/mindmap"
)

const mindMapOfferColumns = `id, session_id, message_id, created_at`

// MindMapOfferRepository is the SQLite-backed implementation of mindmap.OfferRepository.
type MindMapOfferRepository struct {
	db *sql.DB
}

// NewMindMapOfferRepository creates a MindMapOfferRepository backed by db. db must
// already have its migrations applied (see Open).
func NewMindMapOfferRepository(db *sql.DB) *MindMapOfferRepository {
	return &MindMapOfferRepository{db: db}
}

// Create stores the offer, or returns mindmap.ErrOfferPending if the session
// already has one.
func (r *MindMapOfferRepository) Create(ctx context.Context, offer mindmap.Offer) error {
	_, err := execer(ctx, r.db).ExecContext(ctx,
		`INSERT INTO mind_map_offers (`+mindMapOfferColumns+`) VALUES (?, ?, ?, ?)`,
		offer.ID, offer.SessionID, offer.MessageID, formatChallengeTime(offer.CreatedAt),
	)
	// A clash on the id is a different failure from the session's UNIQUE one.
	if isUniqueViolation(err, "mind_map_offers.session_id") {
		return fmt.Errorf("sqlite: creating mind map offer: %w", mindmap.ErrOfferPending)
	}
	if err != nil {
		return fmt.Errorf("sqlite: creating mind map offer: %w", err)
	}
	return nil
}

// Get returns the offer, or mindmap.ErrOfferNotFound.
func (r *MindMapOfferRepository) Get(ctx context.Context, id string) (mindmap.Offer, error) {
	return r.queryOne(ctx, `WHERE id = ?`, id)
}

// GetBySession returns the session's pending offer, or mindmap.ErrOfferNotFound.
func (r *MindMapOfferRepository) GetBySession(ctx context.Context, sessionID string) (mindmap.Offer, error) {
	return r.queryOne(ctx, `WHERE session_id = ?`, sessionID)
}

func (r *MindMapOfferRepository) queryOne(ctx context.Context, where, arg string) (mindmap.Offer, error) {
	row := execer(ctx, r.db).QueryRowContext(ctx, `SELECT `+mindMapOfferColumns+` FROM mind_map_offers `+where, arg)
	var (
		offer     mindmap.Offer
		createdAt string
	)
	err := row.Scan(&offer.ID, &offer.SessionID, &offer.MessageID, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return mindmap.Offer{}, mindmap.ErrOfferNotFound
	}
	if err != nil {
		return mindmap.Offer{}, fmt.Errorf("sqlite: reading mind map offer: %w", err)
	}
	if offer.CreatedAt, err = parseChallengeTime(createdAt); err != nil {
		return mindmap.Offer{}, fmt.Errorf("sqlite: reading mind map offer: %w", err)
	}
	return offer, nil
}

// Delete removes the offer. An unknown id is not an error.
func (r *MindMapOfferRepository) Delete(ctx context.Context, id string) error {
	if _, err := execer(ctx, r.db).ExecContext(ctx, `DELETE FROM mind_map_offers WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: deleting mind map offer: %w", err)
	}
	return nil
}
