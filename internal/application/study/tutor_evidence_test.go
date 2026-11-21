package study

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

const evidenceReportJSON = `{"concept":"Race Condition","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true,"assisted":true,"assessment":"CORRECT","description":"Explained why a lock is needed"}`

// conceptsAccepting lets a new concept be stored, so the evidence step is the
// only thing under test.
func conceptsAccepting(t *testing.T) *studymocks.MockConceptStateRepository {
	t.Helper()
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Maybe()
	concepts.EXPECT().Upsert(context.Background(), mock.AnythingOfType("study.ConceptState")).Return(nil).Maybe()
	return concepts
}

func TestSendMessage_storesEvidenceLinkedToBothMessages(t *testing.T) {
	// Given a report with the user's evidence and a valid assessment
	evidenceRepo := studymocks.NewMockEvidenceRepository(t)
	var stored domainstudy.Evidence
	evidenceRepo.EXPECT().Add(context.Background(), mock.AnythingOfType("study.Evidence")).
		Run(func(_ context.Context, e domainstudy.Evidence) { stored = e }).Return(nil).Once()
	evidenceRepo.EXPECT().TrimOldest(context.Background(), "session-1", "race condition", domainstudy.MaxEvidencePerConcept).Return(nil).Once()

	// When the reply streams
	outcome, err := streamReplyWith(t, stateReplyChunks(evidenceReportJSON),
		WithConceptStates(conceptsAccepting(t)), WithConceptEvidence(evidenceRepo))

	// Then the proof names the user's message and the stored reply, and carries the report's fields
	require.NoError(t, err)
	assert.NotEmpty(t, outcome.user.ID)
	assert.Equal(t, outcome.user.ID, stored.UserMessageID)
	assert.Equal(t, outcome.stored.ID, stored.ReplyMessageID)
	assert.Equal(t, "session-1", stored.SessionID)
	assert.Equal(t, "race condition", stored.Concept)
	assert.Equal(t, domainstudy.AssessmentCorrect, stored.Assessment)
	assert.Equal(t, "Explained why a lock is needed", stored.Description)
	assert.True(t, stored.Assisted)
	assert.WithinDuration(t, time.Now(), stored.CreatedAt, time.Minute)
	assert.Equal(t, time.UTC, stored.CreatedAt.Location())
}

func TestSendMessage_writesNoEvidence_whenTheReportSupportsNone(t *testing.T) {
	cases := map[string]string{
		"no user evidence":   `{"concept":"Mutex","status":"INTRODUCED","strategy":"DIRECT_INSTRUCTION","assessment":"CORRECT"}`,
		"no assessment":      `{"concept":"Mutex","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true}`,
		"unknown assessment": `{"concept":"Mutex","status":"DEMONSTRATED","strategy":"COMPREHENSION_CHECK","userEvidence":true,"assessment":"GREAT"}`,
		"not assessed":       `{"concept":"Mutex","status":"NOT_ASSESSED","strategy":"COMPREHENSION_CHECK","userEvidence":true,"assessment":"CORRECT"}`,
	}
	for name, report := range cases {
		t.Run(name, func(t *testing.T) {
			// Given a report that supports no evidence row; the evidence repository has no expectations
			evidenceRepo := studymocks.NewMockEvidenceRepository(t)

			// When the reply streams
			_, err := streamReplyWith(t, stateReplyChunks(report),
				WithConceptStates(conceptsAccepting(t)), WithConceptEvidence(evidenceRepo))

			// Then the turn succeeds and nothing is stored as evidence
			require.NoError(t, err)
		})
	}
}

func TestSendMessage_writesNoEvidence_whenTheStateReportIsBad(t *testing.T) {
	// Given a malformed block; the evidence repository has no expectations
	evidenceRepo := studymocks.NewMockEvidenceRepository(t)

	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(`{"concept":`),
		WithConceptStates(conceptsAccepting(t)), WithConceptEvidence(evidenceRepo))

	// Then the turn succeeds and no evidence is written
	require.NoError(t, err)
}

func TestSendMessage_keepsTheConceptAndTheReply_whenTheEvidenceWriteFails(t *testing.T) {
	storeErr := errors.New("disk full")
	for name, setup := range map[string]func(*studymocks.MockEvidenceRepository){
		"add fails": func(e *studymocks.MockEvidenceRepository) {
			e.EXPECT().Add(context.Background(), mock.AnythingOfType("study.Evidence")).Return(storeErr).Once()
		},
		"trim fails": func(e *studymocks.MockEvidenceRepository) {
			e.EXPECT().Add(context.Background(), mock.AnythingOfType("study.Evidence")).Return(nil).Once()
			e.EXPECT().TrimOldest(context.Background(), "session-1", "race condition", domainstudy.MaxEvidencePerConcept).Return(storeErr).Once()
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Given an evidence repository that fails, and a concept repository that must still be written
			evidenceRepo := studymocks.NewMockEvidenceRepository(t)
			setup(evidenceRepo)
			concepts := studymocks.NewMockConceptStateRepository(t)
			concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
			concepts.EXPECT().Upsert(context.Background(), mock.AnythingOfType("study.ConceptState")).Return(nil).Once()
			logged := captureLog(t)

			// When the reply streams
			outcome, err := streamReplyWith(t, stateReplyChunks(evidenceReportJSON),
				WithConceptStates(concepts), WithConceptEvidence(evidenceRepo))

			// Then the turn succeeds, the concept was still upserted and the failure is logged without message content
			require.NoError(t, err)
			assert.Equal(t, "Good point.", outcome.stored.Content)
			assert.Contains(t, logged.String(), "storing tutor evidence of session session-1: disk full")
		})
	}
}

func TestSendMessage_writesNoEvidence_whenTheConceptCouldNotBeStored(t *testing.T) {
	// Given a concept write that fails; the evidence repository has no expectations
	evidenceRepo := studymocks.NewMockEvidenceRepository(t)
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), mock.AnythingOfType("study.ConceptState")).Return(errors.New("boom")).Once()
	captureLog(t)

	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(evidenceReportJSON),
		WithConceptStates(concepts), WithConceptEvidence(evidenceRepo))

	// Then the turn succeeds and no proof is written for a concept that has no row
	require.NoError(t, err)
}

func TestSendMessage_writesNoEvidence_whenNoEvidenceRepositoryIsWired(t *testing.T) {
	// Given only the concept repository
	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(evidenceReportJSON), WithConceptStates(conceptsAccepting(t)))

	// Then the turn succeeds
	require.NoError(t, err)
}

func TestRequestOpeningTurn_writesNoEvidence(t *testing.T) {
	// Given an opening reply whose report claims evidence and an assessment; the evidence repository has no expectations
	evidenceRepo := studymocks.NewMockEvidenceRepository(t)
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	concepts.EXPECT().Upsert(context.Background(), mock.AnythingOfType("study.ConceptState")).Return(nil).Once()

	// When requesting the opening turn
	_, _, err := openingTurn(t, stateReplyChunks(evidenceReportJSON), nil, nil,
		WithConceptStates(concepts), WithConceptEvidence(evidenceRepo))

	// Then there is no user message to point to, so no proof is written
	require.NoError(t, err)
}
