package mindmap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

// Generator writes a mind map from a session and ties it to the study-chat
// message whose offer asked for it. GenerationService implements it; the offer
// service depends on this port so it can be tested without a model.
type Generator interface {
	// GenerateForMessage generates and stores a mind map for the session,
	// linked to the message. A failure stores nothing.
	GenerateForMessage(ctx context.Context, sessionID, messageID string) (domainmindmap.MindMap, error)
}

var _ Generator = (*GenerationService)(nil)

// OfferService runs the mind map offer of the study chat: the tutor's answer to
// the user's request for a mind map, which the user accepts or declines.
// Accepting generates a mind map for the session and links it to the offer's
// message; declining deletes the offer. See
// specs/phases/phase-03-architecture-lab/24-mind-map.md.
type OfferService struct {
	offers    domainmindmap.OfferRepository
	mindMaps  domainmindmap.Repository
	generator Generator
	locks     sessionLocks
	now       func() time.Time
	newID     func() string
}

// NewOfferService creates an OfferService backed by the given ports.
func NewOfferService(offers domainmindmap.OfferRepository, mindMaps domainmindmap.Repository, generator Generator) *OfferService {
	return &OfferService{
		offers:    offers,
		mindMaps:  mindMaps,
		generator: generator,
		locks:     sessionLocks{held: map[string]*sessionLock{}},
		now:       func() time.Time { return time.Now().UTC() },
		newID:     uuid.NewString,
	}
}

// CreateOffer stores a pending offer under the assistant message. It returns
// domainmindmap.ErrOfferPending when the session already has one.
func (s *OfferService) CreateOffer(ctx context.Context, sessionID, messageID string) (domainmindmap.Offer, error) {
	offer := domainmindmap.Offer{ID: s.newID(), SessionID: sessionID, MessageID: messageID, CreatedAt: s.now()}
	if err := s.offers.Create(ctx, offer); err != nil {
		return domainmindmap.Offer{}, fmt.Errorf("mindmap: creating offer: %w", err)
	}
	return offer, nil
}

// GetPending returns the session's pending offer, and false when there is none.
func (s *OfferService) GetPending(ctx context.Context, sessionID string) (domainmindmap.Offer, bool, error) {
	offer, err := s.offers.GetBySession(ctx, sessionID)
	if errors.Is(err, domainmindmap.ErrOfferNotFound) {
		return domainmindmap.Offer{}, false, nil
	}
	if err != nil {
		return domainmindmap.Offer{}, false, fmt.Errorf("mindmap: reading pending offer: %w", err)
	}
	return offer, true, nil
}

// Decline deletes the offer and nothing else. An offer that is already gone
// when the call arrives is not an error. It takes the session lock, so it never
// interleaves with an accept of the same offer; if that accept resolved the
// offer while this call waited, the offer is reported as not found, because
// nothing was declined and a mind map now exists.
func (s *OfferService) Decline(ctx context.Context, offerID string) error {
	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainmindmap.ErrOfferNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("mindmap: reading offer: %w", err)
	}
	unlock := s.locks.lock(offer.SessionID)
	defer unlock()
	if _, err := s.offers.Get(ctx, offerID); err != nil {
		if errors.Is(err, domainmindmap.ErrOfferNotFound) {
			return err
		}
		return fmt.Errorf("mindmap: reading offer: %w", err)
	}
	if err := s.offers.Delete(ctx, offerID); err != nil {
		return fmt.Errorf("mindmap: declining offer: %w", err)
	}
	return nil
}

// Accept generates a mind map for the offer's session, links it to the offer's
// message and deletes the offer. It serializes per session, so two concurrent
// accepts never both generate: an accept that waited for the lock and finds the
// offer resolved returns the map the first one made. An accept that arrives
// after the offer was resolved finds no offer at all and fails with
// domainmindmap.ErrOfferNotFound, since the offer alone no longer says which
// message to look under. Before generating it looks for a map already linked to
// the message, which a previous accept left behind if the app closed before the
// offer was deleted, so no duplicate appears. A failure leaves the offer
// pending.
func (s *OfferService) Accept(ctx context.Context, offerID string) (domainmindmap.Summary, error) {
	first, err := s.offers.Get(ctx, offerID)
	if err != nil {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: reading offer: %w", err)
	}
	unlock := s.locks.lock(first.SessionID)
	defer unlock()

	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainmindmap.ErrOfferNotFound) {
		return s.existingMindMap(ctx, first)
	}
	if err != nil {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: reading offer: %w", err)
	}

	existing, err := s.mindMaps.FindByMessage(ctx, offer.MessageID)
	if err == nil {
		return s.resolve(ctx, offer, existing)
	}
	if !errors.Is(err, domainmindmap.ErrMindMapNotFound) {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: finding mind map of the offer: %w", err)
	}

	generated, err := s.generator.GenerateForMessage(ctx, offer.SessionID, offer.MessageID)
	if err != nil {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: generating mind map for offer: %w", err)
	}
	return s.resolve(ctx, offer, summaryOf(generated))
}

// resolve deletes the accepted offer and returns the mind map that answers it.
// The map already exists at this point, so the delete is detached from ctx: a
// cancel that lands as generation finishes must not leave the offer pending
// and report a failure for a map that was created.
func (s *OfferService) resolve(ctx context.Context, offer domainmindmap.Offer, mindMap domainmindmap.Summary) (domainmindmap.Summary, error) {
	if err := s.offers.Delete(context.WithoutCancel(ctx), offer.ID); err != nil {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: resolving offer: %w", err)
	}
	return mindMap, nil
}

// existingMindMap answers an accept that lost the race: the offer is gone, so
// it was either accepted (a mind map under the offer's message exists) or
// declined (none does, and the accept reports the offer as not found).
func (s *OfferService) existingMindMap(ctx context.Context, offer domainmindmap.Offer) (domainmindmap.Summary, error) {
	existing, err := s.mindMaps.FindByMessage(ctx, offer.MessageID)
	if errors.Is(err, domainmindmap.ErrMindMapNotFound) {
		return domainmindmap.Summary{}, domainmindmap.ErrOfferNotFound
	}
	if err != nil {
		return domainmindmap.Summary{}, fmt.Errorf("mindmap: finding mind map of the offer: %w", err)
	}
	return existing, nil
}

// summaryOf is the card a freshly generated mind map shows.
func summaryOf(mindMap domainmindmap.MindMap) domainmindmap.Summary {
	return domainmindmap.Summary{
		ID: mindMap.ID, SessionID: mindMap.SessionID, SessionTitle: mindMap.SessionTitle, Title: mindMap.Title,
		CreatedAt: mindMap.CreatedAt, UpdatedAt: mindMap.UpdatedAt, MessageID: mindMap.MessageID,
	}
}
