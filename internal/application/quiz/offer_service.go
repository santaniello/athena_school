package quiz

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/santaniello/athena/internal/application/sessionlock"
	domainquiz "github.com/santaniello/athena/internal/domain/quiz"
)

// OfferService runs the quiz offer of the study chat: the tutor's answer to the
// user's request for a quiz, which the user accepts or declines. Accepting
// generates a quiz for the session and links it to the offer's message;
// declining deletes the offer. See
// specs/phases/phase-03-architecture-lab/23-interactive-quiz-chat-offer.md.
type OfferService struct {
	offers    domainquiz.OfferRepository
	quizzes   domainquiz.Repository
	generator Generator
	locks     sessionlock.Locks
	now       func() time.Time
	newID     func() string
}

// NewOfferService creates an OfferService backed by the given ports.
func NewOfferService(offers domainquiz.OfferRepository, quizzes domainquiz.Repository, generator Generator) *OfferService {
	return &OfferService{
		offers:    offers,
		quizzes:   quizzes,
		generator: generator,
		now:       func() time.Time { return time.Now().UTC() },
		newID:     uuid.NewString,
	}
}

// CreateOffer stores a pending offer under the assistant message. It returns
// domainquiz.ErrOfferPending when the session already has one.
func (s *OfferService) CreateOffer(ctx context.Context, sessionID, messageID string) (domainquiz.Offer, error) {
	offer := domainquiz.Offer{ID: s.newID(), SessionID: sessionID, MessageID: messageID, CreatedAt: s.now()}
	if err := s.offers.Create(ctx, offer); err != nil {
		return domainquiz.Offer{}, fmt.Errorf("quiz: creating offer: %w", err)
	}
	return offer, nil
}

// GetPending returns the session's pending offer, and false when there is none.
func (s *OfferService) GetPending(ctx context.Context, sessionID string) (domainquiz.Offer, bool, error) {
	offer, err := s.offers.GetBySession(ctx, sessionID)
	if errors.Is(err, domainquiz.ErrOfferNotFound) {
		return domainquiz.Offer{}, false, nil
	}
	if err != nil {
		return domainquiz.Offer{}, false, fmt.Errorf("quiz: reading pending offer: %w", err)
	}
	return offer, true, nil
}

// Decline deletes the offer and nothing else. An offer that is already gone
// when the call arrives is not an error. It takes the session lock, so it never
// interleaves with an accept of the same offer; if that accept resolved the
// offer while this call waited, the offer is reported as not found, because
// nothing was declined and a quiz now exists.
func (s *OfferService) Decline(ctx context.Context, offerID string) error {
	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainquiz.ErrOfferNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("quiz: reading offer: %w", err)
	}
	unlock, err := s.locks.Lock(ctx, offer.SessionID)
	if err != nil {
		return fmt.Errorf("quiz: waiting to decline the offer: %w", err)
	}
	defer unlock()
	if _, err := s.offers.Get(ctx, offerID); err != nil {
		if errors.Is(err, domainquiz.ErrOfferNotFound) {
			return err
		}
		return fmt.Errorf("quiz: reading offer: %w", err)
	}
	if err := s.offers.Delete(ctx, offerID); err != nil {
		return fmt.Errorf("quiz: declining offer: %w", err)
	}
	return nil
}

// Accept generates a quiz of the user's chosen size and difficulty for the
// offer's session, links it to the offer's message and deletes the offer. It
// serializes per session, so two concurrent accepts never both generate: an
// accept that waited for the lock and finds the offer resolved returns the quiz
// the first one made. An accept that arrives after the offer was resolved finds
// no offer at all and fails with domainquiz.ErrOfferNotFound, since the offer
// alone no longer says which message to look under. Before generating it looks
// for a quiz already linked to the message, which a previous accept left
// behind if the app closed before the offer was deleted, so no duplicate
// appears. A failure leaves the offer pending.
func (s *OfferService) Accept(ctx context.Context, offerID string, spec domainquiz.Spec) (domainquiz.Summary, error) {
	first, err := s.offers.Get(ctx, offerID)
	if err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: reading offer: %w", err)
	}
	unlock, err := s.locks.Lock(ctx, first.SessionID)
	if err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: waiting to accept the offer: %w", err)
	}
	defer unlock()

	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainquiz.ErrOfferNotFound) {
		return s.existingQuiz(ctx, first)
	}
	if err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: reading offer: %w", err)
	}

	existing, err := s.quizzes.FindByMessage(ctx, offer.MessageID)
	if err == nil {
		return s.resolve(ctx, offer, existing)
	}
	if !errors.Is(err, domainquiz.ErrQuizNotFound) {
		return domainquiz.Summary{}, fmt.Errorf("quiz: finding quiz of the offer: %w", err)
	}

	generated, err := s.generator.GenerateForMessage(ctx, offer.SessionID, offer.MessageID, spec)
	if err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: generating quiz for offer: %w", err)
	}
	return s.resolve(ctx, offer, summaryOf(generated))
}

// resolve deletes the accepted offer and returns the quiz that answers it.
func (s *OfferService) resolve(ctx context.Context, offer domainquiz.Offer, quiz domainquiz.Summary) (domainquiz.Summary, error) {
	if err := s.offers.Delete(ctx, offer.ID); err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: resolving offer: %w", err)
	}
	return quiz, nil
}

// existingQuiz answers an accept that lost the race: the offer is gone, so it
// was either accepted (a quiz under the offer's message exists) or declined
// (none does, and the accept reports the offer as not found).
func (s *OfferService) existingQuiz(ctx context.Context, offer domainquiz.Offer) (domainquiz.Summary, error) {
	existing, err := s.quizzes.FindByMessage(ctx, offer.MessageID)
	if errors.Is(err, domainquiz.ErrQuizNotFound) {
		return domainquiz.Summary{}, domainquiz.ErrOfferNotFound
	}
	if err != nil {
		return domainquiz.Summary{}, fmt.Errorf("quiz: finding quiz of the offer: %w", err)
	}
	return existing, nil
}

// summaryOf is the card a freshly generated quiz shows: it has no attempts.
func summaryOf(quiz domainquiz.Quiz) domainquiz.Summary {
	return domainquiz.Summary{
		ID: quiz.ID, SessionID: quiz.SessionID, SessionTitle: quiz.SessionTitle, Difficulty: quiz.Difficulty,
		QuestionCount: len(quiz.Questions), CreatedAt: quiz.CreatedAt, MessageID: quiz.MessageID,
	}
}
