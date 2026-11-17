package mindmap

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mocks "github.com/santaniello/athena/internal/application/mindmap/mocks"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
	domainmocks "github.com/santaniello/athena/internal/domain/mindmap/mocks"
)

type offerFixture struct {
	service   *OfferService
	offers    *domainmocks.MockOfferRepository
	mindMaps  *domainmocks.MockRepository
	generator *mocks.MockGenerator
}

func newOfferFixture(t *testing.T) *offerFixture {
	t.Helper()
	f := &offerFixture{
		offers:    domainmocks.NewMockOfferRepository(t),
		mindMaps:  domainmocks.NewMockRepository(t),
		generator: mocks.NewMockGenerator(t),
	}
	f.service = NewOfferService(f.offers, f.mindMaps, f.generator)
	f.service.now = func() time.Time { return fixedNow }
	f.service.newID = func() string { return "offer-1" }
	return f
}

func pendingOffer() domainmindmap.Offer {
	return domainmindmap.Offer{ID: "offer-1", SessionID: "session-1", MessageID: "message-1", CreatedAt: fixedNow}
}

func offeredMindMap() domainmindmap.MindMap {
	return domainmindmap.MindMap{
		ID: "map-1", SessionID: "session-1", SessionTitle: "Distributed Caching", Title: "Caching",
		Tree: validTree, DiagramXML: "<mxGraphModel/>", CreatedAt: fixedNow, UpdatedAt: fixedNow, MessageID: "message-1",
	}
}

func offeredSummary() domainmindmap.Summary {
	return domainmindmap.Summary{
		ID: "map-1", SessionID: "session-1", SessionTitle: "Distributed Caching", Title: "Caching",
		CreatedAt: fixedNow, UpdatedAt: fixedNow, MessageID: "message-1",
	}
}

func TestOfferService_CreateOffer_storesAPendingOfferUnderTheMessage(t *testing.T) {
	// Given a repository that accepts the offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Create(ctx, pendingOffer()).Return(nil).Once()

	// When creating an offer
	got, err := f.service.CreateOffer(ctx, "session-1", "message-1")

	// Then the stored offer is returned
	require.NoError(t, err)
	assert.Equal(t, pendingOffer(), got)
}

func TestOfferService_CreateOffer_keepsErrOfferPending(t *testing.T) {
	// Given a session that already has an offer
	f := newOfferFixture(t)
	f.offers.EXPECT().Create(context.Background(), pendingOffer()).Return(domainmindmap.ErrOfferPending).Once()

	// When creating another one
	_, err := f.service.CreateOffer(context.Background(), "session-1", "message-1")

	// Then ErrOfferPending is kept in the chain
	require.ErrorIs(t, err, domainmindmap.ErrOfferPending)
}

func TestOfferService_GetPending(t *testing.T) {
	boom := errors.New("db down")
	tests := []struct {
		name      string
		offer     domainmindmap.Offer
		repoErr   error
		wantFound bool
		wantErr   error
	}{
		{"pending", pendingOffer(), nil, true, nil},
		{"none", domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound, false, nil},
		{"failure", domainmindmap.Offer{}, boom, false, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a repository answer
			f := newOfferFixture(t)
			f.offers.EXPECT().GetBySession(context.Background(), "session-1").Return(tt.offer, tt.repoErr).Once()

			// When asking for the session's pending offer
			got, found, err := f.service.GetPending(context.Background(), "session-1")

			// Then a missing offer is not an error, and a failure is
			assert.Equal(t, tt.wantFound, found)
			assert.ErrorIs(t, err, tt.wantErr)
			if tt.wantFound {
				assert.Equal(t, tt.offer, got)
			}
		})
	}
}

func TestOfferService_Decline_deletesTheOffer(t *testing.T) {
	// Given a pending offer
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(nil).Once()

	// When declining it
	err := f.service.Decline(ctx, "offer-1")

	// Then it is deleted and nothing else is touched
	require.NoError(t, err)
}

func TestOfferService_Decline_anOfferThatIsGoneIsNotAnError(t *testing.T) {
	// Given an offer that no longer exists; no delete is expected
	f := newOfferFixture(t)
	f.offers.EXPECT().Get(context.Background(), "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()

	// When declining it
	err := f.service.Decline(context.Background(), "offer-1")

	// Then it succeeds
	require.NoError(t, err)
}

func TestOfferService_Decline_anOfferResolvedWhileWaitingIsNotFound(t *testing.T) {
	// Given an offer that an accept resolved while the decline waited for the session lock
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()

	// When declining it
	err := f.service.Decline(ctx, "offer-1")

	// Then nothing is deleted and the caller is told the offer was not pending
	require.ErrorIs(t, err, domainmindmap.ErrOfferNotFound)
}

func TestOfferService_Decline_failures(t *testing.T) {
	boom := errors.New("db down")
	t.Run("re-reading the offer under the lock", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(context.Background(), "offer-1").Return(pendingOffer(), nil).Once()
		f.offers.EXPECT().Get(context.Background(), "offer-1").Return(domainmindmap.Offer{}, boom).Once()

		err := f.service.Decline(context.Background(), "offer-1")

		require.ErrorIs(t, err, boom)
		assert.NotErrorIs(t, err, domainmindmap.ErrOfferNotFound)
	})
	t.Run("reading the offer", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(context.Background(), "offer-1").Return(domainmindmap.Offer{}, boom).Once()

		err := f.service.Decline(context.Background(), "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("deleting the offer", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(context.Background(), "offer-1").Return(pendingOffer(), nil).Twice()
		f.offers.EXPECT().Delete(context.Background(), "offer-1").Return(boom).Once()

		err := f.service.Decline(context.Background(), "offer-1")

		require.ErrorIs(t, err, boom)
	})
}

func TestOfferService_Accept_generatesTheMindMapLinksItAndDeletesTheOffer(t *testing.T) {
	// Given a pending offer whose message has no mind map yet
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
	f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, domainmindmap.ErrMindMapNotFound).Once()
	f.generator.EXPECT().GenerateForMessage(ctx, "session-1", "message-1").Return(offeredMindMap(), nil).Once()
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(nil).Once()

	// When accepting it with the user's choice
	got, err := f.service.Accept(ctx, "offer-1")

	// Then the new mind map is returned as a card and the offer is gone
	require.NoError(t, err)
	assert.Equal(t, offeredSummary(), got)
}

func TestOfferService_Accept_reusesTheMindMapAnEarlierAcceptLeftBehind(t *testing.T) {
	// Given an offer whose message already has a mind map (the app closed before the offer was deleted)
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
	f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(offeredSummary(), nil).Once()
	f.offers.EXPECT().Delete(ctx, "offer-1").Return(nil).Once()

	// When accepting it; the generator has no expectation, so a call would fail the test
	got, err := f.service.Accept(ctx, "offer-1")

	// Then the existing mind map is returned and the offer is deleted
	require.NoError(t, err)
	assert.Equal(t, offeredSummary(), got)
}

func TestOfferService_Accept_aLostRaceReturnsTheExistingMindMap(t *testing.T) {
	// Given an offer that a concurrent accept resolved while this one waited for the lock
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()
	f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(offeredSummary(), nil).Once()

	// When accepting it
	got, err := f.service.Accept(ctx, "offer-1")

	// Then the mind map the other accept made is returned and nothing is generated or deleted
	require.NoError(t, err)
	assert.Equal(t, offeredSummary(), got)
}

func TestOfferService_Accept_aDeclinedOfferIsNotFound(t *testing.T) {
	// Given an offer that was declined: it is gone and left no mind map
	f := newOfferFixture(t)
	ctx := context.Background()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Once()
	f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()
	f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, domainmindmap.ErrMindMapNotFound).Once()

	// When accepting it
	_, err := f.service.Accept(ctx, "offer-1")

	// Then it is reported as not found
	require.ErrorIs(t, err, domainmindmap.ErrOfferNotFound)
}

func TestOfferService_Accept_aMissingOfferIsNotFound(t *testing.T) {
	// Given no such offer
	f := newOfferFixture(t)
	f.offers.EXPECT().Get(context.Background(), "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()

	// When accepting it
	_, err := f.service.Accept(context.Background(), "offer-1")

	// Then it is reported as not found
	require.ErrorIs(t, err, domainmindmap.ErrOfferNotFound)
}

func TestOfferService_Accept_aFailedGenerationLeavesTheOfferPending(t *testing.T) {
	// Given a generator that fails; no delete is expected
	f := newOfferFixture(t)
	ctx := context.Background()
	boom := errors.New("model down")
	f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
	f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, domainmindmap.ErrMindMapNotFound).Once()
	f.generator.EXPECT().GenerateForMessage(ctx, "session-1", "message-1").Return(domainmindmap.MindMap{}, boom).Once()

	// When accepting it
	_, err := f.service.Accept(ctx, "offer-1")

	// Then the failure comes back and the offer stays
	require.ErrorIs(t, err, boom)
}

func TestOfferService_Accept_otherFailures(t *testing.T) {
	boom := errors.New("db down")
	ctx := context.Background()
	t.Run("reading the offer", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("reading the offer again under the lock", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Once()
		f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("looking for the mind map of the message", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
		f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("looking for the mind map of a message after a lost race", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Once()
		f.offers.EXPECT().Get(ctx, "offer-1").Return(domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound).Once()
		f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("deleting the offer of a mind map that already exists", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
		f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(offeredSummary(), nil).Once()
		f.offers.EXPECT().Delete(ctx, "offer-1").Return(boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
	t.Run("deleting the offer after generating", func(t *testing.T) {
		f := newOfferFixture(t)
		f.offers.EXPECT().Get(ctx, "offer-1").Return(pendingOffer(), nil).Twice()
		f.mindMaps.EXPECT().FindByMessage(ctx, "message-1").Return(domainmindmap.Summary{}, domainmindmap.ErrMindMapNotFound).Once()
		f.generator.EXPECT().GenerateForMessage(ctx, "session-1", "message-1").Return(offeredMindMap(), nil).Once()
		f.offers.EXPECT().Delete(ctx, "offer-1").Return(boom).Once()

		_, err := f.service.Accept(ctx, "offer-1")

		require.ErrorIs(t, err, boom)
	})
}

// memoryOffers is a small in-memory store for the concurrency test, where the
// answers must change as the accepts run.
type memoryOffers struct {
	domainmindmap.OfferRepository
	mu     sync.Mutex
	offers map[string]domainmindmap.Offer
}

func (m *memoryOffers) Get(_ context.Context, id string) (domainmindmap.Offer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	offer, ok := m.offers[id]
	if !ok {
		return domainmindmap.Offer{}, domainmindmap.ErrOfferNotFound
	}
	return offer, nil
}

func (m *memoryOffers) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.offers, id)
	return nil
}

type memoryMindMaps struct {
	domainmindmap.Repository
	mu        sync.Mutex
	byMessage map[string]domainmindmap.Summary
}

func (m *memoryMindMaps) FindByMessage(_ context.Context, messageID string) (domainmindmap.Summary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	summary, ok := m.byMessage[messageID]
	if !ok {
		return domainmindmap.Summary{}, domainmindmap.ErrMindMapNotFound
	}
	return summary, nil
}

type countingGenerator struct {
	mindMaps *memoryMindMaps
	mu       sync.Mutex
	calls    int
}

func (g *countingGenerator) GenerateForMessage(_ context.Context, _, messageID string) (domainmindmap.MindMap, error) {
	g.mu.Lock()
	g.calls++
	g.mu.Unlock()
	time.Sleep(20 * time.Millisecond) // a slow model, so the second accept arrives while this one runs
	q := offeredMindMap()
	g.mindMaps.mu.Lock()
	g.mindMaps.byMessage[messageID] = offeredSummary()
	g.mindMaps.mu.Unlock()
	return q, nil
}

func TestOfferService_Accept_twoConcurrentAcceptsGenerateOnce(t *testing.T) {
	// Given one pending offer and a slow generator
	offers := &memoryOffers{offers: map[string]domainmindmap.Offer{"offer-1": pendingOffer()}}
	mindMaps := &memoryMindMaps{byMessage: map[string]domainmindmap.Summary{}}
	generator := &countingGenerator{mindMaps: mindMaps}
	service := NewOfferService(offers, mindMaps, generator)

	// When two accepts of the same offer run at once
	results := make([]domainmindmap.Summary, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = service.Accept(context.Background(), "offer-1")
		}()
	}
	wg.Wait()

	// Then the model was called once and both got that mind map
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, 1, generator.calls)
	assert.Equal(t, offeredSummary(), results[0])
	assert.Equal(t, offeredSummary(), results[1])
}

func TestOfferService_Accept_offersOfDifferentSessionsDoNotWaitForEachOther(t *testing.T) {
	// Given two offers in two sessions and a generator that blocks until both have started
	started := make(chan string, 2)
	release := make(chan struct{})
	offers := &memoryOffers{offers: map[string]domainmindmap.Offer{
		"offer-1": pendingOffer(),
		"offer-2": {ID: "offer-2", SessionID: "session-2", MessageID: "message-2", CreatedAt: fixedNow},
	}}
	mindMaps := &memoryMindMaps{byMessage: map[string]domainmindmap.Summary{}}
	service := NewOfferService(offers, mindMaps, blockingGenerator{started: started, release: release})

	// When both are accepted at once
	var wg sync.WaitGroup
	for _, id := range []string{"offer-1", "offer-2"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = service.Accept(context.Background(), id)
		}()
	}

	// Then both reach the generator without either one finishing first
	for range 2 {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("the second session waited for the first one's lock")
		}
	}
	close(release)
	wg.Wait()
}

type blockingGenerator struct {
	started chan string
	release chan struct{}
}

func (g blockingGenerator) GenerateForMessage(_ context.Context, sessionID, _ string) (domainmindmap.MindMap, error) {
	g.started <- sessionID
	<-g.release
	return offeredMindMap(), nil
}

func TestNewOfferService_defaultsAreRealImplementations(t *testing.T) {
	// Given a service built with the constructor
	service := NewOfferService(nil, nil, nil)

	// Then the clock and the id source are real
	assert.WithinDuration(t, time.Now(), service.now(), time.Minute)
	assert.NotEqual(t, service.newID(), service.newID())
}
