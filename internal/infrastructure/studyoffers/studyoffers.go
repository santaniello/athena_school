// Package studyoffers composes the offers of the study chat: it puts the
// exercise offers and the quiz offers behind the one gate the chat knows, so
// the chat imports no lab or quiz code and the two contexts import each other
// not at all. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
package studyoffers

import (
	"context"
	"errors"
	"fmt"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

var (
	// ErrOfferPending is returned when an offer of one kind is created while
	// the session already has a pending offer of the other kind.
	ErrOfferPending = errors.New("studyoffers: the session already has a pending offer")
	// ErrUnsupportedKind is returned for an offer kind the gate has no side for.
	ErrUnsupportedKind = errors.New("studyoffers: unsupported offer kind")
)

// Offers is one kind of offer, as the gate needs it.
type Offers interface {
	HasPending(ctx context.Context, sessionID string) (bool, error)
	CreateOffer(ctx context.Context, sessionID, messageID string) error
}

// Gate implements the chat's offer gate over the exercise and the quiz offers.
// A session has at most one pending offer across both kinds.
type Gate struct {
	exercise Offers
	quiz     Offers
}

// NewGate creates a Gate. A nil quiz leaves the quiz side unwired: its offers
// are refused with ErrUnsupportedKind and it never counts as pending.
func NewGate(exercise, quiz Offers) *Gate {
	return &Gate{exercise: exercise, quiz: quiz}
}

// HasPending reports whether the session has a pending offer of either kind.
func (g *Gate) HasPending(ctx context.Context, sessionID string) (bool, error) {
	for _, side := range g.sides() {
		pending, err := side.HasPending(ctx, sessionID)
		if err != nil {
			return false, err
		}
		if pending {
			return true, nil
		}
	}
	return false, nil
}

// CreateOffer stores a pending offer of the given kind under the assistant
// message. It refuses when the session already has a pending offer of the
// other kind; a second offer of the same kind is refused by its own service.
func (g *Gate) CreateOffer(ctx context.Context, sessionID, messageID string, kind domainstudy.OfferKind) error {
	var target, other Offers
	switch {
	case kind == domainstudy.OfferKindExercise:
		target, other = g.exercise, g.quiz
	case kind == domainstudy.OfferKindQuiz && g.quiz != nil:
		target, other = g.quiz, g.exercise
	default:
		return fmt.Errorf("%w: %q", ErrUnsupportedKind, kind)
	}
	if other != nil {
		pending, err := other.HasPending(ctx, sessionID)
		if err != nil {
			return err
		}
		if pending {
			return ErrOfferPending
		}
	}
	return target.CreateOffer(ctx, sessionID, messageID)
}

// sides lists the wired kinds.
func (g *Gate) sides() []Offers {
	if g.quiz == nil {
		return []Offers{g.exercise}
	}
	return []Offers{g.exercise, g.quiz}
}
