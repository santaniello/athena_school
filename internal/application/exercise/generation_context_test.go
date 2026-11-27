package exercise

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	exercisemocks "github.com/santaniello/athena/internal/application/exercise/mocks"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
)

type contextFixture struct {
	builder   generationContextBuilder
	summaries *exercisemocks.MockSummaryReader
	profiles  *exercisemocks.MockProfileReader
	notes     *exercisemocks.MockNotesSearcher
	titles    *exercisemocks.MockExistingTitleLister
}

func newContextFixture(t *testing.T) contextFixture {
	t.Helper()
	f := contextFixture{
		summaries: exercisemocks.NewMockSummaryReader(t),
		profiles:  exercisemocks.NewMockProfileReader(t),
		notes:     exercisemocks.NewMockNotesSearcher(t),
		titles:    exercisemocks.NewMockExistingTitleLister(t),
	}
	f.builder = generationContextBuilder{summaries: f.summaries, profiles: f.profiles, notes: f.notes, titles: f.titles}
	return f
}

var testSession = domainexercise.SourceSession{Topic: "Distributed Caching", Goal: "Ace the interview", ArchLabEnabled: true}

var testProfile = domainexercise.GenerationProfile{Area: "backend", ExperienceLevel: "advanced", Language: "pt"}

func TestGenerationContextBuilder_Build_collectsEveryPiece(t *testing.T) {
	// Given a session with a summary, a profile and one note excerpt
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("We covered eviction.", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).Return(nil, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").
		Return([]domainexercise.NoteExcerpt{{Source: "cache.md", Content: "LRU evicts the oldest."}}, nil)

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then every piece is present
	require.NoError(t, err)
	assert.Equal(t, "Distributed Caching", got.Topic)
	assert.Equal(t, "Ace the interview", got.Goal)
	assert.Equal(t, "We covered eviction.", got.Summary)
	assert.Equal(t, testProfile, got.Profile)
	assert.Equal(t, []domainexercise.NoteExcerpt{{Source: "cache.md", Content: "LRU evicts the oldest."}}, got.Notes)
}

func TestGenerationContextBuilder_Build_skipsMissingSummaryAndNotes(t *testing.T) {
	// Given a session with no summary and no notes
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).Return(nil, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, nil)

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then it still builds, with those pieces empty
	require.NoError(t, err)
	assert.Empty(t, got.Summary)
	assert.Empty(t, got.Notes)
}

func TestGenerationContextBuilder_Build_continuesWhenNotesSearchFails(t *testing.T) {
	// Given a notes index that is not ready
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).Return(nil, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, errors.New("index not ready"))

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then generation goes on without notes
	require.NoError(t, err)
	assert.Empty(t, got.Notes)
}

func TestGenerationContextBuilder_Build_failsWhenSummaryCannotBeRead(t *testing.T) {
	// Given a summary store that fails
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", errors.New("db down"))

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then the failure is returned
	require.Error(t, err)
}

func TestGenerationContextBuilder_Build_failsWhenProfileCannotBeRead(t *testing.T) {
	// Given a profile store that fails
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(domainexercise.GenerationProfile{}, errors.New("no profile"))

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then the failure is returned
	require.Error(t, err)
}

func TestGenerationContextBuilder_Build_keepsAtMostEightChunks(t *testing.T) {
	// Given ten short excerpts
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).Return(nil, nil)
	excerpts := make([]domainexercise.NoteExcerpt, 10)
	for i := range excerpts {
		excerpts[i] = domainexercise.NoteExcerpt{Source: "n.md", Content: string(rune('a' + i))}
	}
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(excerpts, nil)

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then only the first eight remain, in order
	require.NoError(t, err)
	assert.Equal(t, excerpts[:8], got.Notes)
}

func TestGenerationContextBuilder_Build_keepsAtMostSixThousandCharacters(t *testing.T) {
	// Given three excerpts of 4,000 characters each
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).Return(nil, nil)
	big := strings.Repeat("é", 4000)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return([]domainexercise.NoteExcerpt{
		{Source: "a.md", Content: big}, {Source: "b.md", Content: big}, {Source: "c.md", Content: big},
	}, nil)

	// When the context is built
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then the second is cut to fit exactly 6,000 characters and the third is dropped
	require.NoError(t, err)
	require.Len(t, got.Notes, 2)
	assert.Equal(t, big, got.Notes[0].Content)
	assert.Equal(t, strings.Repeat("é", 2000), got.Notes[1].Content)
	assert.Equal(t, "b.md", got.Notes[1].Source)
}

func TestGenerationContextBuilder_Build_collectsTheTitlesOfTheSameKindBoundedToTen(t *testing.T) {
	// Given a session whose earlier practice challenges have two titles
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindPractice, 10).
		Return([]string{"Cache-Aside", "Write-through"}, nil)

	// When the context is built for a practice
	got, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindPractice)

	// Then the titles are carried in the same order
	require.NoError(t, err)
	assert.Equal(t, []string{"Cache-Aside", "Write-through"}, got.ExistingTitles)
}

func TestGenerationContextBuilder_Build_failsWhenTheTitlesCannotBeListed(t *testing.T) {
	// Given a challenge store that fails
	f := newContextFixture(t)
	ctx := context.Background()
	f.summaries.EXPECT().ReadSummary(ctx, "session-1").Return("", nil)
	f.profiles.EXPECT().ReadProfile(ctx).Return(testProfile, nil)
	f.notes.EXPECT().SearchNotes(ctx, "session-1", "Distributed Caching").Return(nil, nil)
	f.titles.EXPECT().ListTitlesBySourceSession(ctx, "session-1", domainexercise.ChallengeKindInterview, 10).
		Return(nil, errors.New("db down"))

	// When the context is built
	_, err := f.builder.build(ctx, "session-1", testSession, domainexercise.ChallengeKindInterview)

	// Then the failure is returned
	require.Error(t, err)
}
