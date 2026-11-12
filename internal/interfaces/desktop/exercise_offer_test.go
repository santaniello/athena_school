package desktop

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	applicationstudy "github.com/santaniello/athena/internal/application/study"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type fakeOffers struct {
	offer      domainexercise.Offer
	found      bool
	view       applicationexercise.SessionExerciseView
	err        error
	pending    string
	accept     string
	acceptSpec domainexercise.ChallengeSpec
	decline    string
}

func (f *fakeOffers) GetPending(_ context.Context, sessionID string) (domainexercise.Offer, bool, error) {
	f.pending = sessionID
	return f.offer, f.found, f.err
}

func (f *fakeOffers) Accept(_ context.Context, offerID string, spec domainexercise.ChallengeSpec) (applicationexercise.SessionExerciseView, error) {
	f.accept = offerID
	f.acceptSpec = spec
	return f.view, f.err
}

func (f *fakeOffers) Decline(_ context.Context, offerID string) error {
	f.decline = offerID
	return f.err
}

func newOfferApp(offers exerciseOffers) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithOfferService(offers))
	app.ctx = context.Background()
	return app
}

func TestApp_GetPendingOffer_returnsTheOffer(t *testing.T) {
	// Given a session with a pending offer
	offers := &fakeOffers{found: true, offer: domainexercise.Offer{ID: "offer-1", SessionID: "session-1", MessageID: "message-1"}}

	// When the frontend asks with a padded id
	got, err := newOfferApp(offers).GetPendingOffer(" session-1 ")

	// Then the offer comes back
	require.NoError(t, err)
	assert.Equal(t, "session-1", offers.pending)
	assert.Equal(t, &ExerciseOfferDTO{ID: "offer-1", SessionID: "session-1", MessageID: "message-1"}, got)
}

func TestApp_GetPendingOffer_returnsNullWhenThereIsNone(t *testing.T) {
	// Given a session with no offer
	// When the frontend asks
	got, err := newOfferApp(&fakeOffers{}).GetPendingOffer("session-1")

	// Then it gets null, not an error
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestApp_GetPendingOffer_rejectsABlankSession(t *testing.T) {
	// Given a blank session id
	// When the frontend asks
	_, err := newOfferApp(&fakeOffers{}).GetPendingOffer("  ")

	// Then it is invalid input
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeInvalidInput, coded.Code)
}

func TestApp_GetPendingOffer_mapsFailures(t *testing.T) {
	// Given a failing store
	// When the frontend asks
	_, err := newOfferApp(&fakeOffers{err: errors.New("boom")}).GetPendingOffer("session-1")

	// Then it is a coded error
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeInternal, coded.Code)
}

func TestApp_AcceptOffer_returnsTheLinkedChallenge(t *testing.T) {
	// Given an offer that links a challenge
	offers := &fakeOffers{view: applicationexercise.SessionExerciseView{
		ChallengeID: "challenge-1", Kind: domainexercise.ChallengeKindInterview, Title: "Design a cache", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced, TimeLimitMinutes: 45,
	}}

	// When the frontend accepts it
	got, err := newOfferApp(offers).AcceptOffer(" offer-1 ", " BEGINNER ", "", "")

	// Then the challenge comes back
	require.NoError(t, err)
	assert.Equal(t, "offer-1", offers.accept)
	assert.Equal(t, domainexercise.ChallengeSpec{Difficulty: domainexercise.DifficultyBeginner, Kind: domainexercise.ChallengeKindInterview}, offers.acceptSpec)
	assert.Equal(t, SessionExerciseDTO{ChallengeID: "challenge-1", Kind: "INTERVIEW", Title: "Design a cache", Topic: "caching", Difficulty: "ADVANCED", TimeLimitMinutes: 45}, got)
}

func TestApp_AcceptOffer_mapsErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code string
	}{
		{"unknown offer", domainexercise.ErrOfferNotFound, exerciseCodeOfferNotFound},
		{"invalid generation", fmt.Errorf("x: %w", applicationexercise.ErrInvalidGeneration), exerciseCodeInvalidGeneration},
		{"provider failure", errors.New("provider down"), exerciseCodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a failing accept
			// When the frontend accepts
			_, err := newOfferApp(&fakeOffers{err: tt.err}).AcceptOffer("offer-1", "ADVANCED", "", "")

			// Then the code is stable
			var coded *ExerciseError
			require.ErrorAs(t, err, &coded)
			assert.Equal(t, tt.code, coded.Code)
		})
	}
}

func TestApp_AcceptOffer_rejectsABlankId(t *testing.T) {
	// Given a blank offer id
	// When the frontend accepts
	_, err := newOfferApp(&fakeOffers{}).AcceptOffer(" ", "ADVANCED", "", "")

	// Then it is invalid input
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeInvalidInput, coded.Code)
}

func TestApp_AcceptOffer_withoutADifficultyLeavesTheChoiceToTheProfile(t *testing.T) {
	// Given an offer accepted with no difficulty
	offers := &fakeOffers{}

	// When the frontend accepts
	_, err := newOfferApp(offers).AcceptOffer("offer-1", " ", "", "")

	// Then the service gets an empty difficulty
	require.NoError(t, err)
	assert.Empty(t, offers.acceptSpec.Difficulty)
}

func TestApp_AcceptOffer_rejectsAnUnknownDifficulty(t *testing.T) {
	// Given an unknown difficulty
	offers := &fakeOffers{}

	// When the frontend accepts
	_, err := newOfferApp(offers).AcceptOffer("offer-1", "EXPERT", "", "")

	// Then it is invalid input and the offer is untouched
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeInvalidInput, coded.Code)
	assert.Empty(t, offers.accept)
}

func TestApp_DeclineOffer_declinesIt(t *testing.T) {
	// Given a pending offer
	offers := &fakeOffers{}

	// When the frontend declines it
	err := newOfferApp(offers).DeclineOffer(" offer-1 ")

	// Then the service declined that id
	require.NoError(t, err)
	assert.Equal(t, "offer-1", offers.decline)
}

func TestApp_DeclineOffer_rejectsABlankIdAndMapsFailures(t *testing.T) {
	// Given a blank id, then a failing store
	var blank, failed *ExerciseError
	errBlank := newOfferApp(&fakeOffers{}).DeclineOffer("")
	errFailed := newOfferApp(&fakeOffers{err: errors.New("boom")}).DeclineOffer("offer-1")

	// Then both are coded errors
	require.ErrorAs(t, errBlank, &blank)
	require.ErrorAs(t, errFailed, &failed)
	assert.Equal(t, exerciseCodeInvalidInput, blank.Code)
	assert.Equal(t, exerciseCodeInternal, failed.Code)
}

func TestToExerciseError_mapsAPendingOffer(t *testing.T) {
	// Given a second offer for the session
	// When mapping the error
	coded := toExerciseError(fmt.Errorf("x: %w", domainexercise.ErrOfferPending))

	// Then it has its own code
	assert.Equal(t, exerciseCodeOfferPending, coded.Code)
}

func TestErrorCode_mapsAPendingOfferToItsWireCode(t *testing.T) {
	// Given SendMessage refused while an offer is pending
	// When mapping the error
	// Then the chat gets a stable code
	assert.Equal(t, "offer_pending", errorCode(fmt.Errorf("x: %w", applicationstudy.ErrOfferPending)))
}

func TestApp_AcceptOffer_isRefusedWhileAChallengeIsBeingGenerated(t *testing.T) {
	// Given a Lab generation that is still running
	generator := &blockingGenerator{started: make(chan struct{})}
	offers := &fakeOffers{}
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithOfferService(offers), WithGenerationService(generator))
	app.ctx = context.Background()
	first := make(chan error, 1)
	go func() {
		_, err := app.GenerateChallenge("session-1", "BEGINNER", "", "")
		first <- err
	}()
	<-generator.started

	// When the chat accepts an offer
	_, err := app.AcceptOffer("offer-1", "ADVANCED", "", "")

	// Then it is refused without touching the offer, and the generation is undisturbed
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeGenerationInProgress, coded.Code)
	assert.Empty(t, offers.accept)
	require.NoError(t, app.CancelChallengeGeneration())
	<-first
}

type fakeSessionExercises struct {
	cards   []applicationexercise.SessionExerciseCard
	err     error
	session string
}

func (f *fakeSessionExercises) ListSessionExercises(_ context.Context, sessionID string) ([]applicationexercise.SessionExerciseCard, error) {
	f.session = sessionID
	return f.cards, f.err
}

func newSessionExercisesApp(cards sessionExerciseCards) *App {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, WithSessionExerciseService(cards))
	app.ctx = context.Background()
	return app
}

func TestApp_ListSessionExercises_mapsEachCardWithAndWithoutALatestResult(t *testing.T) {
	// Given a session with a card that has a latest result and one that has not
	cards := &fakeSessionExercises{cards: []applicationexercise.SessionExerciseCard{
		{
			SessionExerciseID: "link-1", MessageID: "message-1", ChallengeID: "challenge-1", Title: "Design a feed",
			Difficulty: domainexercise.DifficultyAdvanced, TimeLimitMinutes: 60, Status: domainexercise.ProgressCompleted, Attempts: 2,
			Latest: &applicationexercise.LatestResult{Outcome: domainexercise.ResultFailed, DiagramScore: 40, UnderstandingScore: 55},
		},
		{SessionExerciseID: "link-2", Status: domainexercise.ProgressNotStarted},
	}}

	// When listing the session's exercises
	got, err := newSessionExercisesApp(cards).ListSessionExercises(" session-1 ")

	// Then the session is trimmed and both cards are mapped
	require.NoError(t, err)
	assert.Equal(t, "session-1", cards.session)
	assert.Equal(t, []SessionExerciseCardDTO{
		{
			SessionExerciseID: "link-1", MessageID: "message-1", ChallengeID: "challenge-1", Title: "Design a feed",
			Difficulty: "ADVANCED", TimeLimitMinutes: 60, Status: "COMPLETED", Attempts: 2,
			Latest: &LatestResultDTO{Outcome: "FAILED", DiagramScore: 40, UnderstandingScore: 55},
		},
		{SessionExerciseID: "link-2", Status: "NOT_STARTED"},
	}, got)
}

func TestApp_ListSessionExercises_returnsAnEmptyListWhenThereAreNone(t *testing.T) {
	// Given a session without cards
	got, err := newSessionExercisesApp(&fakeSessionExercises{}).ListSessionExercises("session-1")

	// Then the list is empty and not nil, so it serializes as []
	require.NoError(t, err)
	assert.Equal(t, []SessionExerciseCardDTO{}, got)
}

func TestApp_ListSessionExercises_rejectsABlankSession(t *testing.T) {
	// When the session id is blank
	_, err := newSessionExercisesApp(&fakeSessionExercises{}).ListSessionExercises("  ")

	// Then it is invalid input
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeInvalidInput, exerciseErr.Code)
}

func TestApp_ListSessionExercises_mapsFailures(t *testing.T) {
	// When the service fails
	_, err := newSessionExercisesApp(&fakeSessionExercises{err: errors.New("boom")}).ListSessionExercises("session-1")

	// Then an exercise error comes back
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
}

func TestApp_ListSessionExercises_failsWithATypedErrorWhenTheServiceIsNotWired(t *testing.T) {
	// Given an app built without the session exercise service
	app := NewApp(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app.ctx = context.Background()

	// When listing the session's exercises
	_, err := app.ListSessionExercises("session-1")

	// Then a typed error comes back instead of a panic
	var exerciseErr *ExerciseError
	require.ErrorAs(t, err, &exerciseErr)
	assert.Equal(t, exerciseCodeUnavailable, exerciseErr.Code)
}

func TestApp_AcceptOffer_practiceSendsTheKindAndTrimmedGoal(t *testing.T) {
	// Given an offer that links an untimed practice
	offers := &fakeOffers{view: applicationexercise.SessionExerciseView{
		ChallengeID: "challenge-2", Kind: domainexercise.ChallengeKindPractice, Title: "Slow price API", Topic: "caching", Difficulty: domainexercise.DifficultyBeginner,
	}}

	// When the frontend accepts it as a practice
	got, err := newOfferApp(offers).AcceptOffer("offer-1", "BEGINNER", "PRACTICE", " cache aside ")

	// Then the service gets the practice spec and the card says practice, with no time
	require.NoError(t, err)
	assert.Equal(t, domainexercise.ChallengeSpec{
		Difficulty: domainexercise.DifficultyBeginner, Kind: domainexercise.ChallengeKindPractice, PracticeGoal: "cache aside",
	}, offers.acceptSpec)
	assert.Equal(t, "PRACTICE", got.Kind)
	assert.Equal(t, 0, got.TimeLimitMinutes)
}

func TestApp_AcceptOffer_practiceWithoutADifficultyStillNeedsAGoal(t *testing.T) {
	// Given a practice accepted with no difficulty and no goal
	offers := &fakeOffers{}

	// When the frontend accepts
	_, err := newOfferApp(offers).AcceptOffer("offer-1", "", "PRACTICE", " ")

	// Then it is invalid input and the offer is untouched
	assert.Equal(t, exerciseCodeInvalidInput, codeOf(t, err).Code)
	assert.Empty(t, offers.accept)
}

func TestApp_AcceptOffer_rejectsAnUnknownKind(t *testing.T) {
	// Given an unknown kind
	offers := &fakeOffers{}

	// When the frontend accepts
	_, err := newOfferApp(offers).AcceptOffer("offer-1", "BEGINNER", "QUIZ", "")

	// Then it is invalid input and the offer is untouched
	assert.Equal(t, exerciseCodeInvalidInput, codeOf(t, err).Code)
	assert.Empty(t, offers.accept)
}
