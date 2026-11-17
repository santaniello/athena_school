package mindmap

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mocks "github.com/santaniello/athena/internal/application/mindmap/mocks"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
)

type contextFixture struct {
	builder   generationContextBuilder
	summaries *mocks.MockSummaryReader
	profiles  *mocks.MockProfileReader
	notes     *mocks.MockNotesSearcher
}

func newContextFixture(t *testing.T) *contextFixture {
	t.Helper()
	f := &contextFixture{
		summaries: mocks.NewMockSummaryReader(t),
		profiles:  mocks.NewMockProfileReader(t),
		notes:     mocks.NewMockNotesSearcher(t),
	}
	f.builder = generationContextBuilder{summaries: f.summaries, profiles: f.profiles, notes: f.notes}
	return f
}

var (
	testSession = domainmindmap.SourceSession{Topic: "Distributed Caching", Goal: "Understand eviction"}
	testProfile = domainmindmap.GenerationProfile{Area: "backend", ExperienceLevel: "intermediate", Language: "pt"}
)

func TestContextBuilder_gathersSummaryProfileAndNotes(t *testing.T) {
	// Given a session with a summary and notes
	f := newContextFixture(t)
	ctx := context.Background()
	notes := []domainmindmap.NoteExcerpt{{Source: "cache.md", Content: "LRU."}}
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered LRU.", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(notes, nil).Once()

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession)

	// Then it holds everything the model is told
	require.NoError(t, err)
	assert.Equal(t, generationContext{
		Topic: "Distributed Caching", Goal: "Understand eviction", Summary: "We covered LRU.", Profile: testProfile, Notes: notes,
	}, got)
}

func TestContextBuilder_failsWhenTheSummaryCannotBeRead(t *testing.T) {
	// Given a summary reader that fails
	f := newContextFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", boom).Once()

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession)

	// Then the failure is returned
	assert.ErrorIs(t, err, boom)
}

func TestContextBuilder_failsWhenTheProfileCannotBeRead(t *testing.T) {
	// Given a profile reader that fails
	f := newContextFixture(t)
	ctx := context.Background()
	boom := errors.New("boom")
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainmindmap.GenerationProfile{}, boom).Once()

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession)

	// Then the failure is returned
	assert.ErrorIs(t, err, boom)
}

func TestContextBuilder_continuesWithoutNotesWhenTheSearchFails(t *testing.T) {
	// Given a notes index that is not ready
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, errors.New("index not ready")).Once()

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession)

	// Then it has no notes and no error
	require.NoError(t, err)
	assert.Empty(t, got.Notes)
}

func TestContextBuilder_failsWhenTheContextIsCancelledDuringTheNotesSearch(t *testing.T) {
	// Given a notes search that fails because the caller cancelled
	f := newContextFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil).Once()
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil).Once()
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").RunAndReturn(
		func(context.Context, string, string) ([]domainmindmap.NoteExcerpt, error) {
			cancel()
			return nil, context.Canceled
		}).Once()

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession)

	// Then the cancellation is reported instead of a context without notes
	assert.ErrorIs(t, err, context.Canceled)
}

func TestBoundNotes_keepsAtMostMaxNoteChunks(t *testing.T) {
	// Given more excerpts than the cap
	found := make([]domainmindmap.NoteExcerpt, maxNoteChunks+3)
	for i := range found {
		found[i] = domainmindmap.NoteExcerpt{Source: "n.md", Content: "x"}
	}

	// When they are bounded
	kept := boundNotes(found)

	// Then only the first maxNoteChunks stay
	assert.Len(t, kept, maxNoteChunks)
}

func TestBoundNotes_cutsTheLastExcerptToTheCharacterBudget(t *testing.T) {
	// Given two excerpts whose total passes the budget
	first := strings.Repeat("a", maxNoteChars-10)
	second := strings.Repeat("b", 50)

	// When they are bounded
	kept := boundNotes([]domainmindmap.NoteExcerpt{{Source: "1", Content: first}, {Source: "2", Content: second}})

	// Then the second is cut to what is left and the total is the budget
	require.Len(t, kept, 2)
	assert.Equal(t, first, kept[0].Content)
	assert.Equal(t, strings.Repeat("b", 10), kept[1].Content)
}

func TestBoundNotes_dropsExcerptsOnceTheBudgetIsSpent(t *testing.T) {
	// Given an excerpt that spends the whole budget and one after it
	found := []domainmindmap.NoteExcerpt{{Source: "1", Content: strings.Repeat("a", maxNoteChars)}, {Source: "2", Content: "late"}}

	// When they are bounded
	kept := boundNotes(found)

	// Then the late excerpt is dropped
	require.Len(t, kept, 1)
	assert.Equal(t, "1", kept[0].Source)
}

func TestBoundNotes_countsRunesNotBytes(t *testing.T) {
	// Given an excerpt of multi-byte runes longer than the budget
	found := []domainmindmap.NoteExcerpt{{Source: "1", Content: strings.Repeat("é", maxNoteChars+10)}}

	// When it is bounded
	kept := boundNotes(found)

	// Then it is cut at maxNoteChars runes without splitting one
	assert.Equal(t, strings.Repeat("é", maxNoteChars), kept[0].Content)
}
