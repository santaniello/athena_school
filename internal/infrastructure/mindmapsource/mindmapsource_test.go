package mindmapsource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationmindmap "github.com/santaniello/athena/internal/application/mindmap"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainmindmap "github.com/santaniello/athena/internal/domain/mindmap"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSessions_ReadSession_returnsTopicAndGoal(t *testing.T) {
	// Given a stored study session
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Caching", Goal: "Interview"}, nil)

	// When it is read
	got, err := NewSessions(sessions).ReadSession(ctx, "session-1")

	// Then topic and goal come back
	require.NoError(t, err)
	assert.Equal(t, domainmindmap.SourceSession{Topic: "Caching", Goal: "Interview"}, got)
}

func TestSessions_ReadSession_translatesAnUnknownSession(t *testing.T) {
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	sessions.EXPECT().GetByID(ctx, "nope").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound)

	_, err := NewSessions(sessions).ReadSession(ctx, "nope")

	require.ErrorIs(t, err, applicationmindmap.ErrSourceSessionNotFound)
}

func TestSessions_ReadSession_passesOtherFailuresThrough(t *testing.T) {
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	failure := errors.New("db down")
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{}, failure)

	_, err := NewSessions(sessions).ReadSession(ctx, "session-1")

	// Then it is returned as itself, not mistaken for an unknown session
	require.ErrorIs(t, err, failure)
	assert.NotErrorIs(t, err, applicationmindmap.ErrSourceSessionNotFound)
}

func TestSummaries_ReadSummary(t *testing.T) {
	failure := errors.New("db down")
	tests := []struct {
		name    string
		stored  domainstudy.ConversationSummary
		err     error
		want    string
		wantErr error
	}{
		{"a stored summary", domainstudy.ConversationSummary{Content: "We covered eviction."}, nil, "We covered eviction.", nil},
		{"no summary yet", domainstudy.ConversationSummary{}, domainstudy.ErrConversationSummaryNotFound, "", nil},
		{"a failing store", domainstudy.ConversationSummary{}, failure, "", failure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := studymocks.NewMockConversationSummaryRepository(t)
			ctx := context.Background()
			repo.EXPECT().Get(ctx, "session-1").Return(tt.stored, tt.err)

			got, err := NewSummaries(repo).ReadSummary(ctx, "session-1")

			assert.Equal(t, tt.want, got)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProfiles_ReadProfile_returnsAreaLevelAndLanguage(t *testing.T) {
	store := profilemocks.NewMockStore(t)
	store.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", Area: "backend", ExperienceLevel: "advanced", AssistantLanguage: "pt"}, nil)

	got, err := NewProfiles(store).ReadProfile(context.Background())

	// Then only what generation needs comes back, never the name
	require.NoError(t, err)
	assert.Equal(t, domainmindmap.GenerationProfile{Area: "backend", ExperienceLevel: "advanced", Language: "pt"}, got)
}

func TestProfiles_ReadProfile_returnsALoadFailure(t *testing.T) {
	store := profilemocks.NewMockStore(t)
	failure := errors.New("no profile")
	store.EXPECT().Load().Return(domainprofile.UserProfile{}, failure)

	_, err := NewProfiles(store).ReadProfile(context.Background())

	require.ErrorIs(t, err, failure)
}

func TestNotes_SearchNotes_searchesTheSessionWithTheQuery(t *testing.T) {
	retriever := knowledgemocks.NewMockRetriever(t)
	ctx := context.Background()
	retriever.EXPECT().Retrieve(ctx, "session-1", []string{"Caching"}).Return(domainknowledge.RetrievalResult{
		Chunks: []domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{FilePath: "cache.md", Content: "LRU evicts the oldest."}},
			{Chunk: domainknowledge.Chunk{FilePath: "ttl.md", Content: "TTL expires entries."}},
		},
	}, nil)

	got, err := NewNotes(retriever).SearchNotes(ctx, "session-1", "Caching")

	require.NoError(t, err)
	assert.Equal(t, []domainmindmap.NoteExcerpt{
		{Source: "cache.md", Content: "LRU evicts the oldest."},
		{Source: "ttl.md", Content: "TTL expires entries."},
	}, got)
}

func TestNotes_SearchNotes_returnsARetrievalFailure(t *testing.T) {
	retriever := knowledgemocks.NewMockRetriever(t)
	ctx := context.Background()
	retriever.EXPECT().Retrieve(ctx, "session-1", []string{"Caching"}).Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable)

	_, err := NewNotes(retriever).SearchNotes(ctx, "session-1", "Caching")

	require.ErrorIs(t, err, domainknowledge.ErrVectorStoreUnavailable)
}
