package exercisesource

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	applicationexercise "github.com/santaniello/athena/internal/application/exercise"
	applicationstudy "github.com/santaniello/athena/internal/application/study"
	domainexercise "github.com/santaniello/athena/internal/domain/exercise"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSessionTopics_ReadSession_returnsTopicAndGoal(t *testing.T) {
	// Given a stored study session
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{ID: "session-1", Topic: "Caching", Goal: "Interview"}, nil)

	// When its topic is read
	got, err := NewSessionTopics(sessions).ReadSession(ctx, "session-1")

	// Then topic and goal come back
	require.NoError(t, err)
	assert.Equal(t, domainexercise.SourceSession{Topic: "Caching", Goal: "Interview"}, got)
}

func TestSessionTopics_ReadSession_translatesAnUnknownSession(t *testing.T) {
	// Given a session that does not exist
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	sessions.EXPECT().GetByID(ctx, "nope").Return(domainstudy.Session{}, domainstudy.ErrSessionNotFound)

	// When its topic is read
	_, err := NewSessionTopics(sessions).ReadSession(ctx, "nope")

	// Then the exercise port's sentinel is returned
	require.ErrorIs(t, err, applicationexercise.ErrSourceSessionNotFound)
}

func TestSessionTopics_ReadSession_passesOtherFailuresThrough(t *testing.T) {
	// Given a store that fails
	sessions := studymocks.NewMockSessionRepository(t)
	ctx := context.Background()
	failure := errors.New("db down")
	sessions.EXPECT().GetByID(ctx, "session-1").Return(domainstudy.Session{}, failure)

	// When the topic is read
	_, err := NewSessionTopics(sessions).ReadSession(ctx, "session-1")

	// Then that failure is returned, not mistaken for an unknown session
	require.ErrorIs(t, err, failure)
	assert.NotErrorIs(t, err, applicationexercise.ErrSourceSessionNotFound)
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
			// Given a summary store answering as in the case
			repo := studymocks.NewMockConversationSummaryRepository(t)
			ctx := context.Background()
			repo.EXPECT().Get(ctx, "session-1").Return(tt.stored, tt.err)

			// When the summary is read
			got, err := NewSummaries(repo).ReadSummary(ctx, "session-1")

			// Then a missing summary is empty and a failure is returned
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
	// Given a stored profile
	store := profilemocks.NewMockStore(t)
	store.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", Area: "backend", ExperienceLevel: "advanced", AssistantLanguage: "pt"}, nil)

	// When it is read for generation
	got, err := NewProfiles(store).ReadProfile(context.Background())

	// Then only what generation needs comes back
	require.NoError(t, err)
	assert.Equal(t, domainexercise.GenerationProfile{Area: "backend", ExperienceLevel: "advanced", Language: "pt"}, got)
}

func TestProfiles_ReadProfile_returnsALoadFailure(t *testing.T) {
	// Given a profile that cannot be loaded
	store := profilemocks.NewMockStore(t)
	failure := errors.New("no profile")
	store.EXPECT().Load().Return(domainprofile.UserProfile{}, failure)

	// When it is read
	_, err := NewProfiles(store).ReadProfile(context.Background())

	// Then the failure is returned
	require.ErrorIs(t, err, failure)
}

func TestNotes_SearchNotes_searchesTheSessionWithTheQuery(t *testing.T) {
	// Given a retriever that finds two chunks
	retriever := knowledgemocks.NewMockRetriever(t)
	ctx := context.Background()
	retriever.EXPECT().Retrieve(ctx, "session-1", []string{"Caching"}).Return(domainknowledge.RetrievalResult{
		Chunks: []domainknowledge.ScoredChunk{
			{Chunk: domainknowledge.Chunk{FilePath: "cache.md", Content: "LRU evicts the oldest."}},
			{Chunk: domainknowledge.Chunk{FilePath: "ttl.md", Content: "TTL expires entries."}},
		},
	}, nil)

	// When the notes are searched
	got, err := NewNotes(retriever).SearchNotes(ctx, "session-1", "Caching")

	// Then each chunk becomes an excerpt, in order
	require.NoError(t, err)
	assert.Equal(t, []domainexercise.NoteExcerpt{
		{Source: "cache.md", Content: "LRU evicts the oldest."},
		{Source: "ttl.md", Content: "TTL expires entries."},
	}, got)
}

func TestNotes_SearchNotes_returnsARetrievalFailure(t *testing.T) {
	// Given an index that is not ready
	retriever := knowledgemocks.NewMockRetriever(t)
	ctx := context.Background()
	retriever.EXPECT().Retrieve(ctx, "session-1", []string{"Caching"}).Return(domainknowledge.RetrievalResult{}, domainknowledge.ErrVectorStoreUnavailable)

	// When the notes are searched
	_, err := NewNotes(retriever).SearchNotes(ctx, "session-1", "Caching")

	// Then the failure is returned for the caller to skip
	require.ErrorIs(t, err, domainknowledge.ErrVectorStoreUnavailable)
}

type fakeOfferService struct {
	pending              bool
	err                  error
	sessionID, messageID string
	askedSession         string
}

func (f *fakeOfferService) GetPending(_ context.Context, sessionID string) (domainexercise.Offer, bool, error) {
	f.askedSession = sessionID
	return domainexercise.Offer{}, f.pending, f.err
}

func (f *fakeOfferService) CreateOffer(_ context.Context, sessionID, messageID string) (domainexercise.Offer, error) {
	f.sessionID, f.messageID = sessionID, messageID
	return domainexercise.Offer{}, f.err
}

func TestOfferGate_HasPending(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")
	tests := []struct {
		name    string
		offers  *fakeOfferService
		want    bool
		wantErr error
	}{
		{"pending", &fakeOfferService{pending: true}, true, nil},
		{"none", &fakeOfferService{}, false, nil},
		{"failure", &fakeOfferService{err: boom}, false, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an offer service
			// When asking the chat's gate
			got, err := NewOfferGate(tt.offers).HasPending(ctx, "session-1")

			// Then the answer is the service's
			assert.Equal(t, tt.want, got)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, "session-1", tt.offers.askedSession)
		})
	}
}

func TestOfferGate_CreateOffer_createsTheOfferUnderTheMessage(t *testing.T) {
	// Given an offer service
	offers := &fakeOfferService{}

	// When the chat creates an offer
	err := NewOfferGate(offers).CreateOffer(context.Background(), "session-1", "message-1")

	// Then the service got the session and message
	require.NoError(t, err)
	assert.Equal(t, "session-1", offers.sessionID)
	assert.Equal(t, "message-1", offers.messageID)
}

func TestOfferGate_CreateOffer_passesFailuresThrough(t *testing.T) {
	// Given a service that refuses a second offer
	offers := &fakeOfferService{err: domainexercise.ErrOfferPending}

	// When the chat creates an offer
	err := NewOfferGate(offers).CreateOffer(context.Background(), "session-1", "message-1")

	// Then the error comes back
	assert.ErrorIs(t, err, domainexercise.ErrOfferPending)
}

type fakeFeedbackService struct {
	items []applicationexercise.SessionFeedback
	err   error
}

func (f fakeFeedbackService) SessionExerciseFeedback(context.Context, string) ([]applicationexercise.SessionFeedback, error) {
	return f.items, f.err
}

func TestFeedbackReader_mapsTheLabFeedbackToTheChatValues(t *testing.T) {
	// Given a passed and a failed attempt
	reader := NewFeedbackReader(fakeFeedbackService{items: []applicationexercise.SessionFeedback{
		{
			ChallengeTitle: "Feed", Topic: "caching", Difficulty: domainexercise.DifficultyAdvanced,
			Result: domainexercise.ResultFailed, DiagramScore: 70, UnderstandingScore: 80, PassScore: 85,
			UnmetMandatory: []string{"Scalability"}, Gaps: []string{"g"}, Recommendations: []string{"r"},
		},
		{ChallengeTitle: "Chat", Result: domainexercise.ResultPassed},
	}})

	// When reading the feedback
	got, err := reader.SessionExerciseFeedback(context.Background(), "session-1")

	// Then the values are copied and only PASSED is passed
	require.NoError(t, err)
	assert.Equal(t, []applicationstudy.ExerciseFeedback{
		{
			ChallengeTitle: "Feed", Topic: "caching", Difficulty: "ADVANCED", Passed: false,
			DiagramScore: 70, UnderstandingScore: 80, PassScore: 85,
			UnmetMandatory: []string{"Scalability"}, Gaps: []string{"g"}, Recommendations: []string{"r"},
		},
		{ChallengeTitle: "Chat", Passed: true},
	}, got)
}

func TestFeedbackReader_passesAFailureThrough(t *testing.T) {
	// Given a feedback service that fails
	boom := errors.New("boom")

	// When reading the feedback
	got, err := NewFeedbackReader(fakeFeedbackService{err: boom}).SessionExerciseFeedback(context.Background(), "session-1")

	// Then the error is passed through and nothing is returned
	require.ErrorIs(t, err, boom)
	assert.Nil(t, got)
}
