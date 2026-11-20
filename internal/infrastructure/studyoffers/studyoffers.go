// Package studyoffers composes the offers of the study chat: it puts the
// exercise, quiz and mind map offers behind the one gate the chat knows, so
// the chat imports no lab, quiz or mind map code and the contexts import each
// other not at all. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
package studyoffers

import (
	"context"
	"errors"
	"fmt"

	"github.com/santaniello/athena/internal/application/sessionlock"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

var (
	// ErrOfferPending is returned when an offer of one kind is created while
	// the session already has a pending offer of another kind.
	ErrOfferPending = errors.New("studyoffers: the session already has a pending offer")
	// ErrUnsupportedKind is returned for an offer kind the gate has no side for.
	ErrUnsupportedKind = errors.New("studyoffers: unsupported offer kind")
)

// Offers is one kind of offer, as the gate needs it.
type Offers interface {
	HasPending(ctx context.Context, sessionID string) (bool, error)
	CreateOffer(ctx context.Context, sessionID, messageID string) error
}

// side is one wired kind of offer.
type side struct {
	kind   domainstudy.OfferKind
	offers Offers
}

// Gate implements the chat's offer gate over the exercise, the quiz and the
// mind map offers. A session has at most one pending offer across all kinds.
type Gate struct {
	sides []side
	// locks makes the check of the other kinds and the create one step per
	// session; the kinds store their offers in separate tables, so nothing
	// else ties them together.
	locks sessionlock.Locks
}

// Option configures a Gate.
type Option func(*Gate)

// WithMindMapOffers wires the mind map side. A nil offers leaves it unwired.
func WithMindMapOffers(offers Offers) Option {
	return func(g *Gate) {
		if offers != nil {
			g.sides = append(g.sides, side{kind: domainstudy.OfferKindMindMap, offers: offers})
		}
	}
}

// NewGate creates a Gate over the exercise offers, the quiz offers and any
// optional side. A nil quiz leaves the quiz side unwired: its offers are
// refused with ErrUnsupportedKind and it never counts as pending.
func NewGate(exercise, quiz Offers, options ...Option) *Gate {
	g := &Gate{sides: []side{{kind: domainstudy.OfferKindExercise, offers: exercise}}}
	if quiz != nil {
		g.sides = append(g.sides, side{kind: domainstudy.OfferKindQuiz, offers: quiz})
	}
	for _, option := range options {
		option(g)
	}
	return g
}

// HasPending reports whether the session has a pending offer of any kind.
func (g *Gate) HasPending(ctx context.Context, sessionID string) (bool, error) {
	for _, s := range g.sides {
		pending, err := s.offers.HasPending(ctx, sessionID)
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
// message. It refuses when the session already has a pending offer of another
// kind; a second offer of the same kind is refused by its own service. The
// check and the create run under the session lock, so two offers of different
// kinds created together cannot both pass.
func (g *Gate) CreateOffer(ctx context.Context, sessionID, messageID string, kind domainstudy.OfferKind) error {
	var target Offers
	for _, s := range g.sides {
		if s.kind == kind {
			target = s.offers
		}
	}
	if target == nil {
		return fmt.Errorf("%w: %q", ErrUnsupportedKind, kind)
	}
	unlock, err := g.locks.Lock(ctx, sessionID)
	if err != nil {
		return err
	}
	defer unlock()
	for _, s := range g.sides {
		if s.kind == kind {
			continue
		}
		pending, err := s.offers.HasPending(ctx, sessionID)
		if err != nil {
			return err
		}
		if pending {
			return ErrOfferPending
		}
	}
	return target.CreateOffer(ctx, sessionID, messageID)
}
