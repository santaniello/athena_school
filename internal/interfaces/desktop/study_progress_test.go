package desktop

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/santaniello/athena/internal/application/study"
	domainstudy "github.com/santaniello/athena/internal/domain/study"

	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func newProgressApp(t *testing.T, options ...study.Option) *App {
	t.Helper()
	app, _ := newTestStudyApp(t, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t),
		profilemocks.NewMockStore(t), foldermocks.NewMockRepository(t), options...)
	return app
}

func TestApp_GetStudySessionProgress_exposesTheGroupsAndTheirProofs(t *testing.T) {
	// Given a demonstrated concept with one proof, and a concept to review
	updatedAt := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	proofAt := time.Date(2026, 11, 21, 11, 0, 0, 0, time.UTC)
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.ConceptState{
		{SessionID: "session-1", Name: "mutex", Status: domainstudy.StatusDemonstrated, AssistanceCount: 1, EvidenceCount: 2, UpdatedAt: updatedAt},
		{SessionID: "session-1", Name: "deadlock", Status: domainstudy.StatusNeedsReview, UpdatedAt: updatedAt},
	}, nil).Once()
	evidence := studymocks.NewMockEvidenceRepository(t)
	evidence.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.Evidence{
		{SessionID: "session-1", Concept: "mutex", Assessment: domainstudy.AssessmentCorrect, Description: "explained the lock", Assisted: true, CreatedAt: proofAt},
	}, nil).Once()
	app := newProgressApp(t, study.WithConceptStates(concepts), study.WithConceptEvidence(evidence))

	// When reading the progress
	result, err := app.GetStudySessionProgress("session-1")

	// Then every group is exposed with RFC 3339 times and each proof's fields
	require.NoError(t, err)
	assert.Equal(t, StudyProgressResult{
		Demonstrated: []StudyProgressConceptResult{{
			Name: "mutex", Status: "DEMONSTRATED", AssistanceCount: 1, EvidenceCount: 2, UpdatedAt: "2026-11-21T12:00:00Z",
			Evidence: []StudyProgressEvidenceResult{{
				Assessment: "CORRECT", Description: "explained the lock", Assisted: true, CreatedAt: "2026-11-21T11:00:00Z",
			}},
		}},
		ExplainedOnly: []StudyProgressConceptResult{},
		Difficulties:  []StudyProgressConceptResult{},
		NeedsReview: []StudyProgressConceptResult{{
			Name: "deadlock", Status: "NEEDS_REVIEW", UpdatedAt: "2026-11-21T12:00:00Z", Evidence: []StudyProgressEvidenceResult{},
		}},
	}, result)
}

func TestApp_GetStudySessionProgress_withoutConcepts_returnsEmptyGroups(t *testing.T) {
	// Given a session without observed concepts
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	app := newProgressApp(t, study.WithConceptStates(concepts))

	// When reading the progress
	result, err := app.GetStudySessionProgress("session-1")

	// Then every group is an empty list, never null
	require.NoError(t, err)
	assert.Equal(t, StudyProgressResult{
		Demonstrated: []StudyProgressConceptResult{}, ExplainedOnly: []StudyProgressConceptResult{},
		Difficulties: []StudyProgressConceptResult{}, NeedsReview: []StudyProgressConceptResult{},
	}, result)
}

func TestApp_GetStudySessionProgress_propagatesErrors(t *testing.T) {
	// Given a failing concept repository
	boom := errors.New("db down")
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, boom).Once()
	app := newProgressApp(t, study.WithConceptStates(concepts))

	// When reading the progress
	_, err := app.GetStudySessionProgress("session-1")

	// Then the error comes back
	require.ErrorIs(t, err, boom)
}
