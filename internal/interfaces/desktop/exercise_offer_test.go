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
	offer   domainexercise.Offer
	found   bool
	view    applicationexercise.SessionExerciseView
	err     error
	pending string
	accept  string
	decline string
}

func (f *fakeOffers) GetPending(_ context.Context, sessionID string) (domainexercise.Offer, bool, error) {
	f.pending = sessionID
	return f.offer, f.found, f.err
}

func (f *fakeOffers) Accept(_ context.Context, offerID string) (applicationexercise.SessionExerciseView, error) {
	f.accept = offerID
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
		ChallengeID: "challenge-1", Title: "Design a cache", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced,
	}}

	// When the frontend accepts it
	got, err := newOfferApp(offers).AcceptOffer(" offer-1 ")

	// Then the challenge comes back
	require.NoError(t, err)
	assert.Equal(t, "offer-1", offers.accept)
	assert.Equal(t, SessionExerciseDTO{ChallengeID: "challenge-1", Title: "Design a cache", Topic: "caching", Difficulty: "ADVANCED"}, got)
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
			_, err := newOfferApp(&fakeOffers{err: tt.err}).AcceptOffer("offer-1")

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
	_, err := newOfferApp(&fakeOffers{}).AcceptOffer(" ")

	// Then it is invalid input
	var coded *ExerciseError
	require.ErrorAs(t, err, &coded)
	assert.Equal(t, exerciseCodeInvalidInput, coded.Code)
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
