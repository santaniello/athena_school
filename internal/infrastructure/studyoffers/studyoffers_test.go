package studyoffers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
)

type fakeOffers struct {
	pending      bool
	pendingErr   error
	createErr    error
	askedSession string
	created      []string // "session/message"
}

func (f *fakeOffers) HasPending(_ context.Context, sessionID string) (bool, error) {
	f.askedSession = sessionID
	return f.pending, f.pendingErr
}

func (f *fakeOffers) CreateOffer(_ context.Context, sessionID, messageID string) error {
	f.created = append(f.created, sessionID+"/"+messageID)
	return f.createErr
}

func TestGate_HasPending(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name     string
		exercise *fakeOffers
		quiz     *fakeOffers
		want     bool
		wantErr  error
	}{
		{"none", &fakeOffers{}, &fakeOffers{}, false, nil},
		{"exercise pending", &fakeOffers{pending: true}, &fakeOffers{}, true, nil},
		{"quiz pending", &fakeOffers{}, &fakeOffers{pending: true}, true, nil},
		{"both pending", &fakeOffers{pending: true}, &fakeOffers{pending: true}, true, nil},
		{"exercise check fails", &fakeOffers{pendingErr: boom}, &fakeOffers{}, false, boom},
		{"quiz check fails", &fakeOffers{}, &fakeOffers{pendingErr: boom}, false, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a gate over both offer kinds
			gate := NewGate(tt.exercise, tt.quiz)

			// When asking whether the session has a pending offer
			got, err := gate.HasPending(context.Background(), "session-1")

			// Then it is true when either kind is pending, and a failure is not hidden
			assert.Equal(t, tt.want, got)
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestGate_HasPending_asksBothKindsAboutTheSameSession(t *testing.T) {
	// Given two offer services that both answer no
	exercise, quiz := &fakeOffers{}, &fakeOffers{}

	// When asking about a session
	_, err := NewGate(exercise, quiz).HasPending(context.Background(), "session-1")

	// Then each one was asked about that session
	require.NoError(t, err)
	assert.Equal(t, "session-1", exercise.askedSession)
	assert.Equal(t, "session-1", quiz.askedSession)
}

func TestGate_CreateOffer_routesByKind(t *testing.T) {
	tests := []struct {
		name         string
		kind         domainstudy.OfferKind
		wantExercise []string
		wantQuiz     []string
	}{
		{"exercise", domainstudy.OfferKindExercise, []string{"session-1/message-1"}, nil},
		{"quiz", domainstudy.OfferKindQuiz, nil, []string{"session-1/message-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a gate with nothing pending
			exercise, quiz := &fakeOffers{}, &fakeOffers{}

			// When creating an offer of a kind
			err := NewGate(exercise, quiz).CreateOffer(context.Background(), "session-1", "message-1", tt.kind)

			// Then only that kind's service received it
			require.NoError(t, err)
			assert.Equal(t, tt.wantExercise, exercise.created)
			assert.Equal(t, tt.wantQuiz, quiz.created)
		})
	}
}

func TestGate_CreateOffer_refusesWhileTheOtherKindIsPending(t *testing.T) {
	tests := []struct {
		name     string
		kind     domainstudy.OfferKind
		exercise *fakeOffers
		quiz     *fakeOffers
	}{
		{"quiz while an exercise is pending", domainstudy.OfferKindQuiz, &fakeOffers{pending: true}, &fakeOffers{}},
		{"exercise while a quiz is pending", domainstudy.OfferKindExercise, &fakeOffers{}, &fakeOffers{pending: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When creating an offer while the other kind is pending
			err := NewGate(tt.exercise, tt.quiz).CreateOffer(context.Background(), "session-1", "message-1", tt.kind)

			// Then it is refused and nothing was created
			require.ErrorIs(t, err, ErrOfferPending)
			assert.Empty(t, tt.exercise.created)
			assert.Empty(t, tt.quiz.created)
		})
	}
}

func TestGate_CreateOffer_failsWhenTheOtherKindCannotBeChecked(t *testing.T) {
	// Given a quiz service whose check fails
	boom := errors.New("boom")
	exercise, quiz := &fakeOffers{}, &fakeOffers{pendingErr: boom}

	// When creating an exercise offer
	err := NewGate(exercise, quiz).CreateOffer(context.Background(), "session-1", "message-1", domainstudy.OfferKindExercise)

	// Then the failure comes back and nothing was created
	require.ErrorIs(t, err, boom)
	assert.Empty(t, exercise.created)
}

func TestGate_CreateOffer_passesCreationFailuresThrough(t *testing.T) {
	// Given a service that refuses the offer
	boom := errors.New("boom")
	exercise, quiz := &fakeOffers{createErr: boom}, &fakeOffers{}

	// When creating an exercise offer
	err := NewGate(exercise, quiz).CreateOffer(context.Background(), "session-1", "message-1", domainstudy.OfferKindExercise)

	// Then the error comes back
	require.ErrorIs(t, err, boom)
}

func TestGate_CreateOffer_refusesAnUnknownKind(t *testing.T) {
	// Given a gate
	exercise, quiz := &fakeOffers{}, &fakeOffers{}

	// When creating an offer of no kind
	err := NewGate(exercise, quiz).CreateOffer(context.Background(), "session-1", "message-1", domainstudy.OfferKindNone)

	// Then it is refused and nothing was created
	require.ErrorIs(t, err, ErrUnsupportedKind)
	assert.Empty(t, exercise.created)
	assert.Empty(t, quiz.created)
}

func TestGate_withoutQuizOffers_hasNoQuizSide(t *testing.T) {
	// Given a gate with no quiz offers wired
	exercise := &fakeOffers{}
	gate := NewGate(exercise, nil)

	// When asking about pending offers and creating each kind
	pending, err := gate.HasPending(context.Background(), "session-1")
	require.NoError(t, err)
	exerciseErr := gate.CreateOffer(context.Background(), "session-1", "message-1", domainstudy.OfferKindExercise)
	quizErr := gate.CreateOffer(context.Background(), "session-1", "message-2", domainstudy.OfferKindQuiz)

	// Then only the exercise side works
	assert.False(t, pending)
	require.NoError(t, exerciseErr)
	require.ErrorIs(t, quizErr, ErrUnsupportedKind)
	assert.Equal(t, []string{"session-1/message-1"}, exercise.created)
}
