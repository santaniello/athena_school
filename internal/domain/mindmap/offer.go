package mindmap

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrOfferNotFound is returned when no mind map offer matches.
	ErrOfferNotFound = errors.New("mind map offer not found")
	// ErrOfferPending is returned when the session already has a pending mind map offer.
	ErrOfferPending = errors.New("mind map offer is already pending")
)

// Offer is the tutor's offer of a mind map in the study chat, waiting for the
// user to accept or decline. It hangs under the assistant message that carries
// it. A resolved offer is deleted, so every stored offer is pending.
type Offer struct {
	ID        string
	SessionID string
	MessageID string
	CreatedAt time.Time
}

// OfferRepository stores the pending mind map offers, at most one per session.
type OfferRepository interface {
	// Create stores the offer, or returns ErrOfferPending when the session has one.
	Create(ctx context.Context, offer Offer) error
	// Get returns the offer, or ErrOfferNotFound.
	Get(ctx context.Context, id string) (Offer, error)
	// GetBySession returns the session's pending offer, or ErrOfferNotFound.
	GetBySession(ctx context.Context, sessionID string) (Offer, error)
	// Delete removes the offer. An unknown id is not an error.
	Delete(ctx context.Context, id string) error
}
