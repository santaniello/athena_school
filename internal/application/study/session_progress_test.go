package study

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func newProgressService(t *testing.T, options ...Option) *Service {
	t.Helper()
	return NewService(studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), newFolderMock(t), knowledgemocks.NewMockRetriever(t),
		nil, nil, nil, nil, nil, options...)
}

func TestGetSessionProgress_groupsTheStoredConceptsWithTheirProofs(t *testing.T) {
	// Given a demonstrated concept with a proof
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.ConceptState{
		{SessionID: "session-1", Name: "mutex", Status: domainstudy.StatusDemonstrated, UpdatedAt: time.Now()},
	}, nil).Once()
	evidence := studymocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.Evidence{
		{SessionID: "session-1", Concept: "mutex", Assessment: domainstudy.AssessmentCorrect, Description: "explained the lock"},
	}, nil).Once()

	// When reading the progress
	progress, err := newProgressService(t, WithConceptStates(concepts), WithConceptEvidence(evidence)).
		GetSessionProgress(context.Background(), "session-1")

	// Then the concept is in the demonstrated group with its proof
	require.NoError(t, err)
	require.Len(t, progress.Demonstrated, 1)
	assert.Equal(t, "mutex", progress.Demonstrated[0].Name)
	require.Len(t, progress.Demonstrated[0].Evidence, 1)
	assert.Equal(t, "explained the lock", progress.Demonstrated[0].Evidence[0].Description)
}

func TestGetSessionProgress_withoutEvidenceRepository_stillListsTheConcepts(t *testing.T) {
	// Given only the concept repository
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.ConceptState{
		{SessionID: "session-1", Name: "mutex", Status: domainstudy.StatusIntroduced},
	}, nil).Once()

	// When reading the progress
	progress, err := newProgressService(t, WithConceptStates(concepts)).GetSessionProgress(context.Background(), "session-1")

	// Then the concept is listed, without proofs
	require.NoError(t, err)
	require.Len(t, progress.ExplainedOnly, 1)
	assert.Empty(t, progress.ExplainedOnly[0].Evidence)
}

func TestGetSessionProgress_withoutStateRepository_isEmpty(t *testing.T) {
	// Given a service that runs without the pedagogical state
	// When reading the progress
	progress, err := newProgressService(t).GetSessionProgress(context.Background(), "session-1")

	// Then it is empty, with no error
	require.NoError(t, err)
	assert.True(t, progress.IsEmpty())
	assert.NotNil(t, progress.Demonstrated)
}

func TestGetSessionProgress_reportsRepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	t.Run("concepts", func(t *testing.T) {
		// Given a concept repository that fails
		concepts := studymocks.NewMockConceptStateRepository(t)
		concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, boom).Once()

		// When reading the progress
		_, err := newProgressService(t, WithConceptStates(concepts)).GetSessionProgress(context.Background(), "session-1")

		// Then the error is wrapped and returned
		require.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "study: reading session progress")
	})
	t.Run("evidence", func(t *testing.T) {
		// Given an evidence repository that fails
		concepts := studymocks.NewMockConceptStateRepository(t)
		concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
		evidence := studymocks.NewMockEvidenceRepository(t)
		evidence.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, boom).Once()

		// When reading the progress
		_, err := newProgressService(t, WithConceptStates(concepts), WithConceptEvidence(evidence)).
			GetSessionProgress(context.Background(), "session-1")

		// Then the error is wrapped and returned
		require.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "study: reading session progress")
	})
}
