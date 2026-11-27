package exercise

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainmocks "github.com/santaniello/athena/internal/domain/exercise/mocks"
)

type offerFixture struct {
	service    *OfferService
	offers     *domainmocks.MockOfferRepository
	challenges *domainmocks.MockChallengeRepository
	links      *domainmocks.MockSessionExerciseRepository
	generator  *exercisemocks.MockChallengeGenerator
	profiles   *exercisemocks.MockProfileReader
	sessions   *exercisemocks.MockSessionTopicReader
	tx         *exercisemocks.MockTransactor
}

// newOfferFixture builds a fixture whose session sits in a folder with the
// Architecture Lab enabled; the session read is optional, since a failure
// before the check never reaches it.
func newOfferFixture(t *testing.T) *offerFixture {
	t.Helper()
	f := newOfferFixtureWithoutSession(t)
	f.sessions.EXPECT().ReadSession(anyContext(), "session-1").
		Return(domainexercise.SourceSession{Topic: "caching", ArchLabEnabled: true}, nil).Maybe()
	return f
}

func anyContext() any {
	return mock.MatchedBy(func(context.Context) bool { return true })
}

func newOfferFixtureWithoutSession(t *testing.T) *offerFixture {
	t.Helper()
	f := &offerFixture{
		offers:     domainmocks.NewMockOfferRepository(t),
		challenges: domainmocks.NewMockChallengeRepository(t),
		links:      domainmocks.NewMockSessionExerciseRepository(t),
		generator:  exercisemocks.NewMockChallengeGenerator(t),
		profiles:   exercisemocks.NewMockProfileReader(t),
		sessions:   exercisemocks.NewMockSessionTopicReader(t),
		tx:         exercisemocks.NewMockTransactor(t),
	}
	f.service = NewOfferService(f.offers, f.challenges, f.links, f.generator, f.profiles, f.sessions, f.tx)
	f.service.now = func() time.Time { return fixedNow }
	f.service.newID = func() string { return "generated-id" }
	return f
}

func (f *offerFixture) runTx(ctx context.Context) {
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(context.WithValue(ctx, txMarker{}, true))
		}).Once()
}

var testOffer = domainexercise.Offer{ID: "offer-1", SessionID: "session-1", MessageID: "message-1", CreatedAt: fixedNow}

var testOfferChallenge = domainexercise.Challenge{
	ID: "challenge-1", Kind: domainexercise.ChallengeKindInterview, Title: "Design a cache", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced, TimeLimitMinutes: 45,
}

func (f *offerFixture) expectOffer(ctx context.Context) {
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil)
}

func offerView() SessionExerciseView {
	return SessionExerciseView{SessionID: "session-1", ChallengeID: "challenge-1", Kind: domainexercise.ChallengeKindInterview, Title: "Design a cache", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced, TimeLimitMinutes: 45}
}

func TestOfferService_CreateOffer_storesAPendingOfferForTheMessage(t *testing.T) {
	// Given a session with no pending offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Create(ctx, domainexercise.Offer{ID: "generated-id", SessionID: "session-1", MessageID: "message-1", CreatedAt: fixedNow}).Return(nil)

	// When creating an offer for an assistant message
	offer, err := f.service.CreateOffer(ctx, "session-1", "message-1")

	// Then the stored offer is returned
	require.NoError(t, err)
	assert.Equal(t, domainexercise.Offer{ID: "generated-id", SessionID: "session-1", MessageID: "message-1", CreatedAt: fixedNow}, offer)
}

func TestOfferService_CreateOffer_refusesASecondPendingOffer(t *testing.T) {
	// Given a session that already has a pending offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Create(ctx, domainexercise.Offer{ID: "generated-id", SessionID: "session-1", MessageID: "message-2", CreatedAt: fixedNow}).
		Return(domainexercise.ErrOfferPending)

	// When creating another
	_, err := f.service.CreateOffer(ctx, "session-1", "message-2")

	// Then ErrOfferPending is returned
	assert.ErrorIs(t, err, domainexercise.ErrOfferPending)
}

func TestOfferService_GetPending(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the pending offer", func(t *testing.T) {
		// Given a pending offer
		f := newOfferFixture(t)
		f.offers.EXPECT().GetBySession(ctx, "session-1").Return(testOffer, nil)

		// When asking for it
		offer, found, err := f.service.GetPending(ctx, "session-1")

		// Then it is found
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, testOffer, offer)
	})

	t.Run("reports none without an error", func(t *testing.T) {
		// Given no pending offer
		f := newOfferFixture(t)
		f.offers.EXPECT().GetBySession(ctx, "session-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound)

		// When asking for it
		_, found, err := f.service.GetPending(ctx, "session-1")

		// Then nothing is found and nothing fails
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("fails when the store fails", func(t *testing.T) {
		// Given a store failure
		f := newOfferFixture(t)
		boom := errors.New("boom")
		f.offers.EXPECT().GetBySession(ctx, "session-1").Return(domainexercise.Offer{}, boom)

		// When asking for the offer
		_, found, err := f.service.GetPending(ctx, "session-1")

		// Then the error comes back
		assert.ErrorIs(t, err, boom)
		assert.False(t, found)
	})
}

func TestOfferService_AcceptAndDecline_giveUpWaitingForTheSessionWhenCancelled(t *testing.T) {
	// Given a session busy with another caller, and a caller whose context is cancelled
	f := newOfferFixture(t)
	release, err := f.service.locks.Lock(context.Background(), "session-1")
	require.NoError(t, err)
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Twice()

	// When it tries to accept or decline
	_, acceptErr := f.service.Accept(ctx, "offer-1", practiceChoice())
	declineErr := f.service.Decline(ctx, "offer-1")

	// Then both return the cancellation instead of waiting, and change nothing
	require.ErrorIs(t, acceptErr, context.Canceled)
	require.ErrorIs(t, declineErr, context.Canceled)
}

func TestOfferService_Decline_deletesTheOffer(t *testing.T) {
	// Given a pending offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(nil)

	// When declining it
	err := f.service.Decline(ctx, "offer-1")

	// Then it is deleted and nothing else is touched
	assert.NoError(t, err)
}

func TestOfferService_Decline_isANoOp_whenTheOfferIsAlreadyGone(t *testing.T) {
	// Given an offer another call already resolved
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound)

	// When declining it
	err := f.service.Decline(ctx, "offer-1")

	// Then nothing is deleted (strict mocks) and there is no error
	assert.NoError(t, err)
}

func TestOfferService_Decline_failsWhenTheOfferCannotBeRead(t *testing.T) {
	// Given a read failure
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, boom)

	// When declining
	err := f.service.Decline(ctx, "offer-1")

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Decline_failsWhenTheStoreFails(t *testing.T) {
	// Given a store failure
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.expectOffer(ctx)
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(boom)

	// When declining
	err := f.service.Decline(ctx, "offer-1")

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_refusesAndKeepsTheOffer_whenTheSessionsFolderHasTheLabDisabled(t *testing.T) {
	// Given a pending offer whose session's folder was switched off, and a
	// latest challenge that would otherwise be reused without a model call
	f := newOfferFixtureWithoutSession(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{Topic: "caching", ArchLabEnabled: false}, nil)

	// When the offer is accepted
	_, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyAdvanced))

	// Then it fails with ErrArchLabNotEnabled; nothing is read, generated, linked or deleted
	require.ErrorIs(t, err, domainexercise.ErrArchLabNotEnabled)
}

func TestOfferService_Accept_failsWhenTheSessionCannotBeRead(t *testing.T) {
	// Given a pending offer and a failing session read
	f := newOfferFixtureWithoutSession(t)
	ctx := context.Background()
	boom := errors.New("db down")
	f.expectOffer(ctx)
	f.sessions.EXPECT().ReadSession(ctx, "session-1").Return(domainexercise.SourceSession{}, boom)

	// When the offer is accepted
	_, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyAdvanced))

	// Then the failure surfaces and nothing is created
	require.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_generatesWithTheProfilesDifficulty_whenNoDifficultyIsChosen(t *testing.T) {
	tests := map[string]domainexercise.Difficulty{
		"beginner":     domainexercise.DifficultyBeginner,
		"intermediate": domainexercise.DifficultyIntermediate,
		"advanced":     domainexercise.DifficultyAdvanced,
		"":             domainexercise.DifficultyIntermediate,
	}
	for level, want := range tests {
		t.Run("level "+level, func(t *testing.T) {
			// Given a session without a challenge and a profile of that level
			f := newOfferFixture(t)
			ctx := context.Background()
			f.expectOffer(ctx)
			f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{ExperienceLevel: level}, nil)
			f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(want)).Return(testOfferChallenge, nil)
			f.runTx(ctx)
			f.links.EXPECT().Create(insideTx(), mock.Anything).Return(nil)
			f.offers.EXPECT().Delete(insideTx(), "offer-1").Return(nil)

			// When accepting
			view, err := f.service.Accept(ctx, "offer-1", practiceChoice())

			// Then the challenge is generated at that difficulty, outside any transaction, and linked
			require.NoError(t, err)
			assert.Equal(t, offerView(), view)
		})
	}
}

func TestOfferService_Accept_generatesAtTheChosenDifficulty_withoutReadingTheProfile(t *testing.T) {
	// Given a session without a challenge
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(domainexercise.DifficultyBeginner)).Return(testOfferChallenge, nil)
	f.runTx(ctx)
	f.links.EXPECT().Create(insideTx(), domainexercise.SessionExercise{
		ID: "generated-id", SessionID: "session-1", ChallengeID: "challenge-1", MessageID: "message-1", CreatedAt: fixedNow,
	}).Return(nil)
	f.offers.EXPECT().Delete(insideTx(), "offer-1").Return(nil)

	// When accepting with a chosen difficulty (no profile call is expected)
	view, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyBeginner))

	// Then the challenge is generated at the chosen difficulty and linked
	require.NoError(t, err)
	assert.Equal(t, offerView(), view)
}

func TestOfferService_Accept_keepsTheOfferAndCreatesNothing_whenGenerationFails(t *testing.T) {
	// Given a generation that fails
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	boom := errors.New("provider down")
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{ExperienceLevel: "advanced"}, nil)
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(domainexercise.DifficultyAdvanced)).Return(domainexercise.Challenge{}, boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then the error comes back; no link is created and the offer is not deleted (strict mocks)
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_failsWhenTheProfileCannotBeRead(t *testing.T) {
	// Given a profile that cannot be read
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	boom := errors.New("no profile")
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{}, boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then nothing is generated and the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_returnsNotFoundForAnUnknownOffer(t *testing.T) {
	// Given no offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyAdvanced))

	// Then ErrOfferNotFound is returned
	assert.ErrorIs(t, err, domainexercise.ErrOfferNotFound)
}

func TestOfferService_Accept_failsAndKeepsTheOffer_whenTheLinkCannotBeCreated(t *testing.T) {
	// Given a link store failure inside the transaction
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	boom := errors.New("db down")
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(domainexercise.DifficultyAdvanced)).Return(testOfferChallenge, nil)
	f.runTx(ctx)
	f.links.EXPECT().Create(insideTx(), mock.Anything).Return(boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyAdvanced))

	// Then the error comes back and the offer is not deleted (strict mocks)
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_failsWhenTheOfferCannotBeDeleted(t *testing.T) {
	// Given a delete failure inside the transaction
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	boom := errors.New("db down")
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(domainexercise.DifficultyAdvanced)).Return(testOfferChallenge, nil)
	f.runTx(ctx)
	f.links.EXPECT().Create(insideTx(), mock.Anything).Return(nil)
	f.offers.EXPECT().Delete(insideTx(), "offer-1").Return(boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceAt(domainexercise.DifficultyAdvanced))

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_returnsTheExistingLink_whenAnotherAcceptResolvedTheOfferFirst(t *testing.T) {
	// Given an offer that is gone by the time the session lock is taken, with
	// the link its resolution created under the offer's message
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound).Once()
	f.links.EXPECT().ListBySession(ctx, "session-1").Return([]domainexercise.SessionExercise{
		{ID: "link-0", ChallengeID: "challenge-0", MessageID: "message-0"},
		{ID: "link-1", ChallengeID: "challenge-1", MessageID: "message-1"},
	}, nil)
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(testOfferChallenge, nil)

	// When accepting
	view, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then the link of this offer is returned and nothing is generated or created
	require.NoError(t, err)
	assert.Equal(t, offerView(), view)
}

func TestOfferService_Accept_returnsNotFound_whenTheOfferWasDeclinedAndOnlyOlderLinksExist(t *testing.T) {
	// Given an offer that vanished (declined) while the session has a link
	// that belongs to an earlier message
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound).Once()
	f.links.EXPECT().ListBySession(ctx, "session-1").Return([]domainexercise.SessionExercise{
		{ID: "link-0", ChallengeID: "challenge-0", MessageID: "message-0"},
	}, nil)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then ErrOfferNotFound is returned, not the old exercise
	assert.ErrorIs(t, err, domainexercise.ErrOfferNotFound)
}

func TestOfferService_Accept_returnsNotFound_whenTheOfferIsGoneAndTheSessionHasNoLinks(t *testing.T) {
	// Given an offer that vanished and a session without links
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound).Once()
	f.links.EXPECT().ListBySession(ctx, "session-1").Return(nil, nil)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then ErrOfferNotFound is returned
	assert.ErrorIs(t, err, domainexercise.ErrOfferNotFound)
}

func TestOfferService_Accept_failsWhenTheLinksCannotBeListedAfterALostRace(t *testing.T) {
	// Given an offer that vanished and a failing link store
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, domainexercise.ErrOfferNotFound).Once()
	f.links.EXPECT().ListBySession(ctx, "session-1").Return(nil, boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_failsWhenTheOfferReadFailsInsideTheLock(t *testing.T) {
	// Given a second read of the offer that fails for another reason
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("db down")
	f.offers.EXPECT().Get(ctx, "offer-1").Return(testOffer, nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, boom).Once()

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_failsWhenTheFirstOfferReadFails(t *testing.T) {
	// Given an offer read that fails
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("db down")
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainexercise.Offer{}, boom)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", practiceChoice())

	// Then the error comes back
	assert.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_serializesConcurrentAcceptsOfTheSameSession(t *testing.T) {
	// Given a session without a challenge whose generation takes a moment
	f := newOfferFixture(t)
	ctx := context.Background()
	var mu sync.Mutex
	generated := false

	f.offers.EXPECT().Get(ctx, "offer-1").RunAndReturn(func(context.Context, string) (domainexercise.Offer, error) {
		mu.Lock()
		defer mu.Unlock()
		if generated {
			return domainexercise.Offer{}, domainexercise.ErrOfferNotFound
		}
		return testOffer, nil
	})
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{ExperienceLevel: "advanced"}, nil).Maybe()
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", practiceAt(domainexercise.DifficultyAdvanced)).
		RunAndReturn(func(context.Context, string, domainexercise.ChallengeSpec) (domainexercise.Challenge, error) {
			time.Sleep(20 * time.Millisecond)
			return testOfferChallenge, nil
		}).Once()
	f.tx.EXPECT().WithinTx(ctx, mock.MatchedBy(func(func(context.Context) error) bool { return true })).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			mu.Lock()
			generated = true
			mu.Unlock()
			return err
		}).Once()
	f.links.EXPECT().Create(ctx, mock.Anything).Return(nil).Once()
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(nil).Once()
	f.links.EXPECT().ListBySession(ctx, "session-1").Return([]domainexercise.SessionExercise{
		{ID: "link-1", ChallengeID: "challenge-1", MessageID: "message-1"},
	}, nil).Maybe()
	f.challenges.EXPECT().Get(ctx, "challenge-1").Return(testOfferChallenge, nil).Maybe()

	// When two accepts run at once
	var wg sync.WaitGroup
	views := make([]SessionExerciseView, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			views[i], errs[i] = f.service.Accept(ctx, "offer-1", practiceChoice())
		}()
	}
	wg.Wait()

	// Then one generation happened and both callers got the same link
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, offerView(), views[0])
	assert.Equal(t, offerView(), views[1])
}

func TestOfferService_Accept_practiceNeverReusesAnExistingChallenge(t *testing.T) {
	// Given an unlinked interview challenge of the chosen difficulty and a user who chose a practice
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)
	spec := domainexercise.ChallengeSpec{Difficulty: domainexercise.DifficultyAdvanced, Kind: domainexercise.ChallengeKindPractice, PracticeGoal: "cache aside"}
	fresh := domainexercise.Challenge{ID: "challenge-2", Kind: domainexercise.ChallengeKindPractice, Title: "Slow price API", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced}
	f.generator.EXPECT().GenerateChallenge(ctx, "session-1", spec).Return(fresh, nil)
	f.runTx(ctx)
	f.links.EXPECT().Create(insideTx(), domainexercise.SessionExercise{
		ID: "generated-id", SessionID: "session-1", ChallengeID: "challenge-2", MessageID: "message-1", CreatedAt: fixedNow,
	}).Return(nil)
	f.offers.EXPECT().Delete(insideTx(), "offer-1").Return(nil)

	// When accepting
	view, err := f.service.Accept(ctx, "offer-1", spec)

	// Then a practice is generated without looking for a challenge to reuse (strict mocks)
	require.NoError(t, err)
	assert.Equal(t, "challenge-2", view.ChallengeID)
}

func TestOfferService_Accept_refusesAPracticeWithoutAGoal(t *testing.T) {
	// Given a practice chosen with no goal
	f := newOfferFixture(t)
	ctx := context.Background()
	f.expectOffer(ctx)

	// When accepting
	_, err := f.service.Accept(ctx, "offer-1", domainexercise.ChallengeSpec{Difficulty: domainexercise.DifficultyBeginner, Kind: domainexercise.ChallengeKindPractice})

	// Then it is refused and the offer stays pending (strict mocks)
	assert.ErrorIs(t, err, domainexercise.ErrPracticeGoalRequired)
}

// practiceChoice is what the user sends when accepting an offer without
// choosing a difficulty: a practice goal only.
func practiceChoice() domainexercise.ChallengeSpec {
	return domainexercise.ChallengeSpec{Kind: domainexercise.ChallengeKindPractice, PracticeGoal: "cache aside"}
}

func practiceAt(difficulty domainexercise.Difficulty) domainexercise.ChallengeSpec {
	return domainexercise.ChallengeSpec{Difficulty: difficulty, Kind: domainexercise.ChallengeKindPractice, PracticeGoal: "cache aside"}
}
