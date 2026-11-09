package exercise

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

// SessionExerciseView is a challenge linked to a study session: what the chat
// card shows.
type SessionExerciseView struct {
	ChallengeID string
	Title       string
	Topic       string
	Difficulty  domainexercise.Difficulty
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
	tx         Transactor
	locks      sessionLocks
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
	tx Transactor,
) *OfferService {
	return &OfferService{
		offers:     offers,
		challenges: challenges,
		links:      links,
		generator:  generator,
		profiles:   profiles,
		tx:         tx,
		locks:      sessionLocks{held: map[string]*sync.Mutex{}},
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

// Decline deletes the offer. Nothing else is created or touched.
func (s *OfferService) Decline(ctx context.Context, offerID string) error {
	if err := s.offers.Delete(ctx, offerID); err != nil {
		return fmt.Errorf("exercise: declining offer: %w", err)
	}
	return nil
}

// Accept links a challenge to the offer's session and deletes the offer. It
// reuses the session's latest challenge when there is one, else generates one
// at the profile's difficulty. It serializes per session, so two concurrent
// accepts never both generate: the second one finds the offer resolved and
// returns the existing link. A failure leaves the offer pending.
func (s *OfferService) Accept(ctx context.Context, offerID string) (SessionExerciseView, error) {
	first, err := s.offers.Get(ctx, offerID)
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: reading offer: %w", err)
	}
	unlock := s.locks.lock(first.SessionID)
	defer unlock()

	offer, err := s.offers.Get(ctx, offerID)
	if errors.Is(err, domainexercise.ErrOfferNotFound) {
		return s.existingLink(ctx, first.SessionID)
	}
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: reading offer: %w", err)
	}

	challenge, err := s.challengeFor(ctx, offer.SessionID)
	if err != nil {
		return SessionExerciseView{}, err
	}
	if err := s.link(ctx, offer, challenge); err != nil {
		return SessionExerciseView{}, err
	}
	return sessionExerciseViewOf(challenge), nil
}

// existingLink answers an accept that lost the race: the offer is gone, so
// the winner left a challenge and a link for the session.
func (s *OfferService) existingLink(ctx context.Context, sessionID string) (SessionExerciseView, error) {
	challenge, err := s.challenges.FindLatestBySourceSession(ctx, sessionID)
	if errors.Is(err, domainexercise.ErrChallengeNotFound) {
		return SessionExerciseView{}, domainexercise.ErrOfferNotFound
	}
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: finding challenge: %w", err)
	}
	_, err = s.links.GetBySessionAndChallenge(ctx, sessionID, challenge.ID)
	if errors.Is(err, domainexercise.ErrSessionExerciseNotFound) {
		return SessionExerciseView{}, domainexercise.ErrOfferNotFound
	}
	if err != nil {
		return SessionExerciseView{}, fmt.Errorf("exercise: finding link: %w", err)
	}
	return sessionExerciseViewOf(challenge), nil
}

// challengeFor reuses the session's latest challenge, else generates one with
// no transaction open.
func (s *OfferService) challengeFor(ctx context.Context, sessionID string) (domainexercise.Challenge, error) {
	challenge, err := s.challenges.FindLatestBySourceSession(ctx, sessionID)
	if err == nil {
		return challenge, nil
	}
	if !errors.Is(err, domainexercise.ErrChallengeNotFound) {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: finding challenge: %w", err)
	}
	profile, err := s.profiles.ReadProfile(ctx)
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: reading profile: %w", err)
	}
	challenge, err = s.generator.GenerateChallenge(ctx, sessionID, difficultyFor(profile.ExperienceLevel))
	if err != nil {
		return domainexercise.Challenge{}, fmt.Errorf("exercise: generating challenge for offer: %w", err)
	}
	return challenge, nil
}

// link creates the session exercise under the offer's message and deletes the
// offer in one transaction. An existing link is accepted as is.
func (s *OfferService) link(ctx context.Context, offer domainexercise.Offer, challenge domainexercise.Challenge) error {
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		err := s.links.Create(ctx, domainexercise.SessionExercise{
			ID: s.newID(), SessionID: offer.SessionID, ChallengeID: challenge.ID, MessageID: offer.MessageID, CreatedAt: s.now(),
		})
		if err != nil && !errors.Is(err, domainexercise.ErrSessionExerciseExists) {
			return err
		}
		return s.offers.Delete(ctx, offer.ID)
	})
	if err != nil {
		return fmt.Errorf("exercise: linking challenge to session: %w", err)
	}
	return nil
}

func sessionExerciseViewOf(challenge domainexercise.Challenge) SessionExerciseView {
	return SessionExerciseView{
		ChallengeID: challenge.ID, Title: challenge.Title, Topic: challenge.Topic, Difficulty: challenge.Difficulty,
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

// sessionLocks hands out one mutex per session, in process: the desktop app
// is a single process.
type sessionLocks struct {
	mu   sync.Mutex
	held map[string]*sync.Mutex
}

// lock takes the session's mutex and returns the function that releases it.
func (l *sessionLocks) lock(sessionID string) func() {
	l.mu.Lock()
	m, ok := l.held[sessionID]
	if !ok {
		m = &sync.Mutex{}
		l.held[sessionID] = m
	}
	l.mu.Unlock()
	m.Lock()
	return m.Unlock
}
