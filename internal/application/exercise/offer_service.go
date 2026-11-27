package exercise

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/santaniello/athena/internal/application/sessionlock"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// SessionExerciseView is a challenge linked to a study session: what the chat
// card shows.
type SessionExerciseView struct {
	// SessionID is the session of the offer, not of the challenge: a challenge
	// can come from the Lab and not belong to any session.
	SessionID   string
	ChallengeID string
	Kind        domainexercise.ChallengeKind
	Title       string
	Topic       string
	Difficulty  domainexercise.Difficulty
	// TimeLimitMinutes is the exam time, shown in the rules before it starts.
	// It is 0 for an untimed practice.
	TimeLimitMinutes int
}

// OfferService runs the exercise offer of the study chat: the tutor's answer
// to the user's request for an exercise, which the user accepts or declines.
// Accepting links a challenge to the session (generated now or reused);
// declining deletes the offer. See
// specs/phases/phase-03-architecture-lab/14-ai-challenge-generation.md.
type OfferService struct {
	offers     domainexercise.OfferRepository
	challenges domainexercise.ChallengeRepository
	links      domainexercise.SessionExerciseRepository
	generator  ChallengeGenerator
	profiles   ProfileReader
	sessions   SessionTopicReader
	tx         Transactor
	locks      sessionlock.Locks
	now        func() time.Time
	newID      func() string
}

// NewOfferService creates an OfferService backed by the given ports.
func NewOfferService(
	offers domainexercise.OfferRepository,
	challenges domainexercise.ChallengeRepository,
	links domainexercise.SessionExerciseRepository,
	generator ChallengeGenerator,
	profiles ProfileReader,
	sessions SessionTopicReader,
	tx Transactor,
) *OfferService {
	return &OfferService{
		offers:     offers,
		challenges: challenges,
		links:      links,
		generator:  generator,
		profiles:   profiles,
		sessions:   sessions,
		tx:         tx,
		now:        func() time.Time { return time.Now().UTC() },
		newID:      uuid.NewString,
	}
}

// CreateOffer stores a pending offer under the assistant message. It returns
// domainexercise.ErrOfferPending when the session already has one.
func (s *OfferService) CreateOffer(ctx context.Context, sessionID, messageID string) (domainexercise.Offer, error) {
	offer := domainexercise.Offer{ID: s.newID(), SessionID: sessionID, MessageID: messageID, CreatedAt: s.now()}
	if err := s.offers.Create(ctx, offer); err != nil {
		return domainexercise.Offer{}, fmt.Errorf("exercise: creating offer: %w", err)
	}
	return offer, nil
}

// GetPending returns the session's pending offer, and false when there is none.
func (s *OfferService) GetPending(ctx context.Context, sessionID string) (domainexercise.Offer, bool, error) {
	offer, err := s.offers.GetBySession(ctx, sessionID)
	if errors.Is(err, domainexercise.ErrOfferNotFound) {
		return domainexercise.Offer{}, false, nil
	}
	if err != nil {
		return domainexercise.Offer{}, false, fmt.Errorf("exercise: reading pending offer: %w", err)
	}
	return offer, true, nil
}

// Decline deletes the offer. Nothing else is created or touched. It takes the
// session lock, so it never interleaves with an accept of the same offer.
func (s *OfferService) Decline(ctx context.Context, offerID string) error {
	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainexercise.ErrOfferNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("exercise: reading offer: %w", err)
	}
	unlock, err := s.locks.Lock(ctx, offer.SessionID)
	if err != nil {
		return fmt.Errorf("exercise: waiting to decline the offer: %w", err)
	}
	defer unlock()
	if err := s.offers.Delete(ctx, offerID); err != nil {
		return fmt.Errorf("exercise: declining offer: %w", err)
	}
	return nil
}

// Accept links a practice challenge to the offer's session and deletes the
// offer. The spec is the user's choice on the offer: an empty difficulty means
// no choice and the profile's difficulty is used, and the goal says what to
// practice. It always generates a new challenge. It serializes per session, so two concurrent accepts never both
// generate: the second one finds the offer resolved and returns the existing
// link. A failure leaves the offer pending, and so does ErrArchLabNotEnabled,
// returned first when the session's folder has the lab switched off: the user
// can still decline.
func (s *OfferService) Accept(ctx context.Context, offerID string, spec domainexercise.ChallengeSpec) (SessionExerciseView, error) {
	first, err := s.offers.Get(ctx, offerID)
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: reading offer: %w", err)
	}
	unlock, err := s.locks.Lock(ctx, first.SessionID)
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: waiting to accept the offer: %w", err)
	}
	defer unlock()

	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainexercise.ErrOfferNotFound) {
		return s.existingLink(ctx, first)
	}
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: reading offer: %w", err)
	}

	if err := s.requireArchLab(ctx, offer.SessionID); err != nil {
		return SessionExerciseView{}, err
	}
	challenge, err := s.challengeFor(ctx, offer.SessionID, spec)
	if err != nil {
		return SessionExerciseView{}, err
	}
	if err := s.link(ctx, offer, challenge); err != nil {
		return SessionExerciseView{}, err
	}
	return sessionExerciseViewOf(offer.SessionID, challenge), nil
}

// requireArchLab fails with ErrArchLabNotEnabled when the session's folder has
// the Architecture Lab off right now. It runs before the reuse path too, which
// calls no model: a switched-off folder must not gain an exercise either way.
func (s *OfferService) requireArchLab(ctx context.Context, sessionID string) error {
	session, err := s.sessions.ReadSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("exercise: reading offer session: %w", err)
	}
	if !session.ArchLabEnabled {
		return domainexercise.ErrArchLabNotEnabled
	}
	return nil
}

// existingLink answers an accept that lost the race: the offer is gone, so it
// was either accepted (a link under the offer's message exists) or declined
// (none does, and the accept reports the offer as not found).
func (s *OfferService) existingLink(ctx context.Context, offer domainexercise.Offer) (SessionExerciseView, error) {
	links, err := s.links.ListBySession(ctx, offer.SessionID)
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: finding link: %w", err)
	}
	for _, link := range links {
		if link.MessageID != offer.MessageID {
			continue
		}
		challenge, err := s.challenges.Get(ctx, link.ChallengeID)
		if err != nil {
			return SessionExerciseView{}, fmt.Errorf("exercise: finding challenge: %w", err)
		}
		return sessionExerciseViewOf(offer.SessionID, challenge), nil
	}
	return SessionExerciseView{}, domainexercise.ErrOfferNotFound
}

// challengeFor generates the practice challenge the offer asks for, with no
// transaction open. An empty difficulty means the profile's, resolved before
// anything else. A practice depends on the goal the user wrote, so it is never
// reused. An interview is refused by the generator: those come from the
// catalog's canonical problems.
func (s *OfferService) challengeFor(ctx context.Context, sessionID string, spec domainexercise.ChallengeSpec) (domainexercise.Challenge, error) {
	spec = spec.Normalized()
	if spec.Difficulty == "" {
		profile, err := s.profiles.ReadProfile(ctx)
		if err != nil {
			return domainexercise.Challenge{}, fmt.Errorf("exercise: reading profile: %w", err)
		}
		spec.Difficulty = difficultyFor(profile.ExperienceLevel)
	}
	if err := spec.Validate(); err != nil {
		return domainexercise.Challenge{}, err
	}
	return s.generate(ctx, sessionID, spec)
}

func (s *OfferService) generate(ctx context.Context, sessionID string, spec domainexercise.ChallengeSpec) (domainexercise.Challenge, error) {
	challenge, err := s.generator.GenerateChallenge(ctx, sessionID, spec)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: generating challenge for offer: %w", err)
	}
	return challenge, nil
}

// link creates the session exercise under the offer's message and deletes the
// offer in one transaction.
func (s *OfferService) link(ctx context.Context, offer domainexercise.Offer, challenge domainexercise.Challenge) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		err := s.links.Create(ctx, domainexercise.SessionExercise{
			ID: s.newID(), SessionID: offer.SessionID, ChallengeID: challenge.ID, MessageID: offer.MessageID, CreatedAt: s.now(),
		})
		if err != nil {
			return err
		}
		return s.offers.Delete(ctx, offer.ID)
	})
	if err != nil {
		return fmt.Errorf("exercise: linking challenge to session: %w", err)
	}
	return nil
}

func sessionExerciseViewOf(sessionID string, challenge domainexercise.Challenge) SessionExerciseView {
	return SessionExerciseView{
		SessionID: sessionID, ChallengeID: challenge.ID, Kind: challenge.Kind, Title: challenge.Title, Topic: challenge.Topic, Difficulty: challenge.Difficulty,
		TimeLimitMinutes: challenge.TimeLimitMinutes,
	}
}

// difficultyFor maps the profile's experience level to a challenge
// difficulty; an unknown level gets the intermediate one.
func difficultyFor(experienceLevel string) domainexercise.Difficulty {
	switch experienceLevel {
	case "beginner":
		return domainexercise.DifficultyBeginner
	case "advanced":
		return domainexercise.DifficultyAdvanced
	default:
		return domainexercise.DifficultyIntermediate
	}
}
