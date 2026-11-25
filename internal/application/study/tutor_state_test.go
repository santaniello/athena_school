package study

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	applicationstudymocks "github.com/santaniello/athena/internal/application/study/mocks"
	foldermocks "github.com/santaniello/athena/internal/domain/folder/mocks"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainprofile "github.com/santaniello/athena/internal/domain/profile"
	profilemocks "github.com/santaniello/athena/internal/domain/profile/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// conceptLike matches a stored concept by every field except UpdatedAt, which
// must only be set.
func conceptLike(want domainstudy.ConceptState) any {
	return mock.MatchedBy(func(got domainstudy.ConceptState) bool {
		if got.UpdatedAt.IsZero() || got.UpdatedAt.Location() != time.UTC {
			return false
		}
		got.UpdatedAt = time.Time{}
		return got == want
	})
}

func stateReplyChunks(report string) []string {
	return []string{"Good point.\n\n", tutorStateOpenTag + "\n" + report + "\n" + tutorStateCloseTag}
}

func existingConceptStates(count int) []domainstudy.ConceptState {
	states := make([]domainstudy.ConceptState, 0, count)
	for i := 0; i < count; i++ {
		states = append(states, domainstudy.ConceptState{
			SessionID: "session-1", Name: fmt.Sprintf("concept %d", i), Status: domainstudy.StatusIntroduced,
		})
	}
	return states
}

// captureLog collects what the package logs until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logged bytes.Buffer
	original := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(original) })
	return &logged
}

func TestSendMessage_storesTheStateReport_andHidesTheBlock(t *testing.T) {
	logged := captureLog(t)
	// Given a reply whose block is split across chunks, for a new concept
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "race condition", DisplayName: "Race Condition", Status: domainstudy.StatusDemonstrated,
		LastStrategy: domainstudy.StrategyCorrectiveFeedback, AssistanceCount: 1, EvidenceCount: 1,
	})).Return(nil).Once()
	chunks := []string{
		"Good point.\n\n[[ATHENA_TUT", `OR_STATE]]` + "\n" + `{"concept":"Race Condition","status":"DEMONSTRATED",`,
		`"strategy":"CORRECTIVE_FEEDBACK","userEvidence":true,"assisted":true}` + "\n[[/ATHENA_TUTOR", "_STATE]]\n",
	}

	// When it streams
	outcome, err := streamReplyWith(t, chunks, WithConceptStates(concepts))

	// Then the user sees (the newline after the block is text) and the store keeps
	// the text without the block, and the concept was upserted without any log
	require.NoError(t, err)
	assert.NotContains(t, logged.String(), "storing tutor state")
	assert.Equal(t, "Good point.\n\n\n", joined(outcome.received))
	assert.Equal(t, "Good point.", outcome.stored.Content)
}

func TestSendMessage_appliesTheDomainRules_toTheStoredConcept(t *testing.T) {
	// Given a NEEDS_REVIEW concept and a DEMONSTRATED report with evidence
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return([]domainstudy.ConceptState{{
		SessionID: "session-1", Name: "race condition", Status: domainstudy.StatusNeedsReview,
		LastStrategy: domainstudy.StrategyConceptReview, AssistanceCount: 4, EvidenceCount: 1,
	}}, nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "race condition", DisplayName: "RACE condition", Status: domainstudy.StatusPartiallyUnderstood,
		LastStrategy: domainstudy.StrategyCorrectiveFeedback, AssistanceCount: 4, EvidenceCount: 2,
	})).Return(nil).Once()

	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(
		`{"concept":" RACE   condition ","status":"DEMONSTRATED","strategy":"CORRECTIVE_FEEDBACK","userEvidence":true}`,
	), WithConceptStates(concepts))

	// Then the existing row (found by its normalized name) steps to PARTIALLY_UNDERSTOOD
	require.NoError(t, err)
}

func TestSendMessage_capsAReportWithoutEvidence_atIntroduced(t *testing.T) {
	// Given a DEMONSTRATED report without the user's evidence, for a new concept
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "mutex", DisplayName: "Mutex", Status: domainstudy.StatusIntroduced,
		LastStrategy: domainstudy.StrategyDirectInstruction,
	})).Return(nil).Once()

	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(
		`{"concept":"Mutex","status":"DEMONSTRATED","strategy":"DIRECT_INSTRUCTION","userEvidence":false}`,
	), WithConceptStates(concepts))

	// Then it is stored as INTRODUCED
	require.NoError(t, err)
}

func TestSendMessage_ignoresANotAssessedReport(t *testing.T) {
	// Given a NOT_ASSESSED report; Upsert has no expectation, so a write fails the test
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()

	// When the reply streams
	outcome, err := streamReplyWith(t, stateReplyChunks(
		`{"concept":"Mutex","status":"NOT_ASSESSED","strategy":"DIRECT_INSTRUCTION"}`,
	), WithConceptStates(concepts))

	// Then the reply is intact and nothing was written
	require.NoError(t, err)
	assert.Equal(t, "Good point.", outcome.stored.Content)
}

func TestSendMessage_keepsTheReplyAndWritesNothing_whenTheMarkerIsBad(t *testing.T) {
	// Given a block with an unknown status; the repository is only read for the prompt
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	logged := captureLog(t)

	// When the reply streams
	outcome, err := streamReplyWith(t, stateReplyChunks(
		`{"concept":"secret-topic","status":"MASTERED"}`,
	), WithConceptStates(concepts))

	// Then the reply is delivered and stored without the block, and the log has no message content
	require.NoError(t, err)
	assert.Equal(t, "Good point.\n\n", joined(outcome.received))
	assert.Equal(t, "Good point.", outcome.stored.Content)
	assert.Contains(t, logged.String(), "session-1")
	assert.NotContains(t, logged.String(), "secret-topic")
}

func TestSendMessage_cutsAnUnterminatedBlock_andWritesNothing(t *testing.T) {
	// Given a reply that opens the block and never closes it
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()

	// When the reply streams
	outcome, err := streamReplyWith(t, []string{"Good point.\n\n", tutorStateOpenTag + `{"concept":"A","sta`}, WithConceptStates(concepts))

	// Then neither the user nor the store sees the protocol text
	require.NoError(t, err)
	assert.Equal(t, "Good point.\n\n", joined(outcome.received))
	assert.Equal(t, "Good point.", outcome.stored.Content)
}

func TestSendMessage_evictsOnTheThirtyFirstConcept_neverTheOneJustWritten(t *testing.T) {
	// Given 30 stored concepts and a report for a new one
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(existingConceptStates(30), nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "new concept", DisplayName: "New Concept", Status: domainstudy.StatusDemonstrated,
		LastStrategy: domainstudy.StrategyGuidedPractice, EvidenceCount: 1,
	})).Return(nil).Once()
	concepts.EXPECT().DeleteOldest(context.Background(), "session-1", "new concept").Return(nil).Once()

	// When the reply streams
	_, err := streamReplyWith(t, stateReplyChunks(
		`{"concept":"New Concept","status":"DEMONSTRATED","strategy":"GUIDED_PRACTICE","userEvidence":true}`,
	), WithConceptStates(concepts))

	// Then one concept is evicted, keeping the new one
	require.NoError(t, err)
}

func TestSendMessage_doesNotEvict_whenTheSessionStaysWithinTheLimit(t *testing.T) {
	// Given 30 stored concepts and a report for one of them, and 29 stored and a new one
	for _, tt := range []struct {
		name   string
		stored int
		report string
	}{
		{"existing at the limit", 30, "concept 3"},
		{"new below the limit", 29, "brand new"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			concepts := studymocks.NewMockConceptStateRepository(t)
			concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(existingConceptStates(tt.stored), nil).Twice()
			concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
				SessionID: "session-1", Name: tt.report, DisplayName: tt.report, Status: domainstudy.StatusIntroduced,
				LastStrategy: domainstudy.StrategyGuidedPractice,
			})).Return(nil).Once()

			// When the reply streams (DeleteOldest has no expectation)
			_, err := streamReplyWith(t, stateReplyChunks(
				`{"concept":"`+tt.report+`","status":"INTRODUCED","strategy":"GUIDED_PRACTICE"}`,
			), WithConceptStates(concepts))

			// Then nothing is evicted
			require.NoError(t, err)
		})
	}
}

func TestSendMessage_doesNotFailTheReply_whenTheStateRepositoryFails(t *testing.T) {
	storeErr := errors.New("disk full")
	fresh := conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "fresh", DisplayName: "fresh", Status: domainstudy.StatusIntroduced,
		LastStrategy: domainstudy.StrategyGuidedPractice,
	})
	for name, setup := range map[string]func(*studymocks.MockConceptStateRepository){
		"list fails": func(c *studymocks.MockConceptStateRepository) {
			c.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, storeErr).Twice()
		},
		"upsert fails": func(c *studymocks.MockConceptStateRepository) {
			c.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
			c.EXPECT().Upsert(context.Background(), fresh).Return(storeErr).Once()
		},
		"evict fails": func(c *studymocks.MockConceptStateRepository) {
			c.EXPECT().ListBySession(context.Background(), "session-1").Return(existingConceptStates(30), nil).Twice()
			c.EXPECT().Upsert(context.Background(), fresh).Return(nil).Once()
			c.EXPECT().DeleteOldest(context.Background(), "session-1", "fresh").Return(storeErr).Once()
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Given a repository that fails
			concepts := studymocks.NewMockConceptStateRepository(t)
			setup(concepts)
			logged := captureLog(t)

			// When the reply streams
			outcome, err := streamReplyWith(t, stateReplyChunks(
				`{"concept":"fresh","status":"INTRODUCED","strategy":"GUIDED_PRACTICE"}`,
			), WithConceptStates(concepts))

			// Then the turn still succeeds with its stored reply, and the failure is logged
			require.NoError(t, err)
			assert.Equal(t, "Good point.", outcome.stored.Content)
			assert.Contains(t, logged.String(), "storing tutor state of session session-1: disk full")
		})
	}
}

func TestSendMessage_stripsTheBlockButWritesNothing_whenNoStateRepositoryIsWired(t *testing.T) {
	// Given a service without the concept repository
	// When a reply carries a block
	outcome, err := streamReplyWith(t, stateReplyChunks(`{"concept":"A","status":"INTRODUCED"}`))

	// Then the block is still hidden and stripped
	require.NoError(t, err)
	assert.Equal(t, "Good point.\n\n", joined(outcome.received))
	assert.Equal(t, "Good point.", outcome.stored.Content)
}

func TestSendMessage_neverLeaksAnOfferMarkerSplitNextToTheStateBlock(t *testing.T) {
	// Given an offer marker split where the state filter holds its first half,
	// followed by a state block
	gate := applicationstudymocks.NewMockOfferGate(t)
	gate.EXPECT().HasPending(context.Background(), "session-1").Return(false, nil).Once()
	gate.EXPECT().CreateOffer(context.Background(), "session-1", mock.MatchedBy(func(messageID string) bool { return messageID != "" }), domainstudy.OfferKindExercise).Return(nil).Once()
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Twice()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "a", DisplayName: "A", Status: domainstudy.StatusIntroduced,
	})).Return(nil).Once()
	chunks := []string{
		"Sure.\n[[ATHENA_", "EXERCISE_OFFER]]\n" + tutorStateOpenTag,
		`{"concept":"A","status":"INTRODUCED"}`, tutorStateCloseTag,
	}

	// When it streams
	outcome, err := streamReplyWith(t, chunks, WithOfferGate(gate), WithConceptStates(concepts))

	// Then no marker reaches the user or the store, and the offer was created
	require.NoError(t, err)
	assert.Equal(t, "Sure.\n\n", joined(outcome.received))
	assert.Equal(t, "Sure.", outcome.stored.Content)
}

func TestSendMessage_deliversBothFiltersHeldFragments_whenTheReplyEndsMidTag(t *testing.T) {
	// Given a reply that ends in what could be the start of either marker
	// When it streams
	outcome, err := streamReplyWith(t, []string{"See [[ATHENA_"})

	// Then the text reaches the user whole and is stored whole
	require.NoError(t, err)
	assert.Equal(t, "See [[ATHENA_", joined(outcome.received))
	assert.Equal(t, "See [[ATHENA_", outcome.stored.Content)
}

// openingTurn runs RequestOpeningTurn against ports that stream chunks, fail
// the stream with llmErr, or fail the assistant message's persistence with
// appendErr.
func openingTurn(t *testing.T, chunks []string, llmErr, appendErr error, options ...Option) (stored *domainstudy.Message, received []string, err error) {
	t.Helper()
	sessions := studymocks.NewMockSessionRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	llm := llmmocks.NewMockProvider(t)
	profiles := profilemocks.NewMockStore(t)
	tx := applicationstudymocks.NewMockTransactor(t)
	if llmErr == nil {
		runWithinTx(tx)
	}
	messages.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	sessions.EXPECT().GetByID(context.Background(), "session-1").Return(normalSession("session-1"), nil).Once()
	profiles.EXPECT().Load().Return(domainprofile.UserProfile{Name: "Ana", AssistantName: "Atena"}, nil)
	llm.EXPECT().ChatStream(context.Background(), mock.AnythingOfType("llm.ChatRequest"), mock.AnythingOfType("func(string) error")).
		Run(func(_ context.Context, _ domainllm.ChatRequest, handler func(string) error) {
			for _, chunk := range chunks {
				require.NoError(t, handler(chunk))
			}
		}).Return(domainllm.StreamResponse{}, llmErr).Once()
	if llmErr == nil {
		messages.EXPECT().Append(context.Background(), mock.MatchedBy(func(m domainstudy.Message) bool { return m.Role == domainstudy.RoleAssistant })).
			Run(func(_ context.Context, m domainstudy.Message) { stored = &m }).Return(appendErr).Once()
		if appendErr == nil {
			sessions.EXPECT().UpdateContext(context.Background(), "session-1", mock.AnythingOfType("study.ContextUsage")).Return(nil).Once()
		}
	}
	messageSources := knowledgemocks.NewMockMessageSourceRepository(t)
	if llmErr == nil && appendErr == nil {
		messageSources = mockMessageSourceSave(t, nil)
	}
	service := NewService(sessions, messages, llm, profiles, foldermocks.NewMockRepository(t), knowledgemocks.NewMockRetriever(t), tx, nil, messageSources, nil, nil, options...)

	err = service.RequestOpeningTurn(context.Background(), "session-1", "Caching",
		func(chunk string) error { received = append(received, chunk); return nil }, nil, nil)
	return stored, received, err
}

func TestRequestOpeningTurn_forcesNoUserEvidence_onItsReport(t *testing.T) {
	// Given an opening reply whose report claims evidence although the user said nothing
	concepts := studymocks.NewMockConceptStateRepository(t)
	concepts.EXPECT().ListBySession(context.Background(), "session-1").Return(nil, nil).Once()
	concepts.EXPECT().Upsert(context.Background(), conceptLike(domainstudy.ConceptState{
		SessionID: "session-1", Name: "caching", DisplayName: "Caching", Status: domainstudy.StatusIntroduced,
		LastStrategy: domainstudy.StrategyDirectInstruction, AssistanceCount: 1,
	})).Return(nil).Once()

	// When requesting the opening turn
	stored, received, err := openingTurn(t, stateReplyChunks(
		`{"concept":"Caching","status":"DEMONSTRATED","strategy":"DIRECT_INSTRUCTION","userEvidence":true,"assisted":true}`,
	), nil, nil, WithConceptStates(concepts))

	// Then the report is stored as if it had no evidence, and the block is hidden
	require.NoError(t, err)
	assert.Equal(t, "Good point.\n\n", joined(received))
	assert.Equal(t, "Good point.", stored.Content)
}

func TestRequestOpeningTurn_writesNoState_whenTheStreamFails(t *testing.T) {
	// Given a stream that delivers a block and then fails; the repository has no expectations
	concepts := studymocks.NewMockConceptStateRepository(t)

	// When requesting the opening turn
	stored, _, err := openingTurn(t, stateReplyChunks(`{"concept":"A","status":"INTRODUCED"}`), context.Canceled, nil, WithConceptStates(concepts))

	// Then the turn fails and nothing was stored
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, stored)
}

func TestRequestOpeningTurn_writesNoState_whenTheReplyCannotBePersisted(t *testing.T) {
	// Given a stream that completes but whose assistant message cannot be stored
	concepts := studymocks.NewMockConceptStateRepository(t)

	// When requesting the opening turn
	_, _, err := openingTurn(t, stateReplyChunks(`{"concept":"A","status":"INTRODUCED"}`), nil, errors.New("disk full"), WithConceptStates(concepts))

	// Then the turn fails and the state was not written
	require.Error(t, err)
}
