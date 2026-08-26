package knowledge

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	txmocks "github.com/santaniello/athena/internal/application/knowledge/mocks"
	configmocks "github.com/santaniello/athena/internal/domain/config/mocks"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
	knowledgemocks "github.com/santaniello/athena/internal/domain/knowledge/mocks"
	domainllm "github.com/santaniello/athena/internal/domain/llm"
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

// receiptFixture seeds a backend receipt for one candidate exactly as
// ExtractFromSession would have, without going through the LLM — SaveDrafts
// and SaveAndApprove tests only care about what save does with a receipt
// already on file.
func receiptFixture(service *Service, sessionID, sourceLabel, candidateID string, refs ...domainknowledge.EvidenceRef) string {
	return service.receipts.Create(sessionID, sourceLabel, []parsedCandidate{
		{Item: domainknowledge.Item{ID: candidateID}, EvidenceRefs: refs},
	})
}

func TestSaveDrafts_revalidatesAgainstTheReceiptAndPersistsItemWithEvidence(t *testing.T) {
	// Given a backend receipt tying candidate "candidate-1" to a session
	// Message and a hostile client input trying to smuggle its own ID/status
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "candidate-1" && item.ID != "" &&
			item.Topic == "Distributed systems" && item.Concept == "CAP theorem" && item.Definition == "A trade-off." &&
			item.Source == domainknowledge.SourceAthena && item.Status == domainknowledge.StatusDraft &&
			!item.CreatedAt.IsZero() && item.CreatedAt.Equal(item.UpdatedAt)
	})).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "CAP describes trade-offs."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID == "message-1" && e.SourceLabel == "Distributed systems" &&
			e.Excerpt == "CAP describes trade-offs." && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != "" && link.ItemID != "candidate-1"
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := receiptFixture(service, "session-1", "Distributed systems", "candidate-1",
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "CAP describes trade-offs."})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: " Distributed systems ", Concept: " CAP theorem ", Definition: " A trade-off. ", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusApproved},
	}

	// When saving the candidate
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then the item is persisted with regenerated server-owned fields, its
	// evidence snapshot is created and linked, and the receipt is consumed
	require.NoError(t, err)
	assert.Equal(t, []int{0}, savedIndices)
	_, stillFound := service.receipts.Get(batchID, "candidate-1")
	assert.False(t, stillFound)
}

func TestSaveDrafts_skipsCandidateWithNoMatchingReceipt(t *testing.T) {
	// Given no receipt at all for the batch/candidate pair the client sent —
	// a fabricated ID, a foreign batch, or simply an ID outside the extraction
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), studymocks.NewMockMessageRepository(t), llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, knowledgemocks.NewMockEvidenceRepository(t))
	input := []domainknowledge.Item{
		{ID: "unknown-candidate", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
	}

	// When saving with a batch ID that was never issued
	savedIndices, err := service.SaveDrafts(ctx, "unknown-batch", input)

	// Then nothing is saved and the repository is never touched
	require.NoError(t, err)
	assert.Empty(t, savedIndices)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestSaveDrafts_skipsCandidateWhenReceiptMessageWasDeleted(t *testing.T) {
	// Given a receipt whose cited Message no longer exists in the session
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{}, nil).Once()
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, knowledgemocks.NewMockEvidenceRepository(t))
	batchID := receiptFixture(service, "session-1", "Go", "candidate-1",
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Channels are typed conduits."})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
	}

	// When saving
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then the candidate is unsavable and the receipt stays for a later retry
	require.NoError(t, err)
	assert.Empty(t, savedIndices)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	_, stillFound := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, stillFound)
}

func TestSaveDrafts_skipsCandidateWhenTheMessageNoLongerContainsTheQuote(t *testing.T) {
	// Given the cited Message still exists but was edited to remove the quote
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Completely rewritten content."},
	}, nil).Once()
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), nil, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, knowledgemocks.NewMockEvidenceRepository(t))
	batchID := receiptFixture(service, "session-1", "Go", "candidate-1",
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Channels are typed conduits."})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
	}

	// When saving
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then the candidate is unsavable
	require.NoError(t, err)
	assert.Empty(t, savedIndices)
	repository.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}

func TestSaveDrafts_savesCandidateWhenTheMessageWasEditedAroundTheQuote(t *testing.T) {
	// Given the cited Message was edited but still contains the exact quote
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Edited intro. Channels are typed conduits. Edited outro."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := receiptFixture(service, "session-1", "Go", "candidate-1",
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "Channels are typed conduits."})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
	}

	// When saving
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then the candidate is still saved
	require.NoError(t, err)
	assert.Equal(t, []int{0}, savedIndices)
}

func TestSaveDrafts_stopsWhenLinkingEvidenceToTheItemFails(t *testing.T) {
	// Given an Evidence snapshot that is created successfully but fails to link
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.Anything).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "shared evidence quote"},
	}, nil).Once()
	linkErr := errors.New("database locked")
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(linkErr).Once()
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llmmocks.NewMockProvider(t), configmocks.NewMockStore(t), knowledgemocks.NewMockChunkRepository(t), tx, nil, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := receiptFixture(service, "session-1", "Go", "candidate-1",
		domainknowledge.EvidenceRef{MessageID: "message-1", Quote: "shared evidence quote"})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "Channels", Definition: "Typed conduits."},
	}

	// When saving
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then the failure propagates, nothing is reported saved (the enclosing
	// transaction rolls back the Item write together with the failed link),
	// and the receipt is kept for retry
	assert.ErrorIs(t, err, linkErr)
	assert.Empty(t, savedIndices)
	_, stillFound := service.receipts.Get(batchID, "candidate-1")
	assert.True(t, stillFound)
}

func TestSaveDrafts_stopsAtTransactionFailureAndKeepsThatReceiptForRetry(t *testing.T) {
	// Given two candidates, the second of which fails to persist
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "first" })).Return(nil).Once()
	saveErr := errors.New("database locked")
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "second" })).Return(saveErr).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "shared evidence quote"},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := service.receipts.Create("session-1", "Go", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-1"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "shared evidence quote"}}},
		{Item: domainknowledge.Item{ID: "candidate-2"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "shared evidence quote"}}},
	})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "first", Definition: "one"},
		{ID: "candidate-2", Topic: "Go", Concept: "second", Definition: "two"},
	}

	// When saving both candidates
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then processing stops at the failed candidate, whose receipt is kept
	// for retry, while the successfully saved candidate's receipt is gone
	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, []int{0}, savedIndices)
	_, firstStillFound := service.receipts.Get(batchID, "candidate-1")
	assert.False(t, firstStillFound)
	_, secondStillFound := service.receipts.Get(batchID, "candidate-2")
	assert.True(t, secondStillFound)
}

func TestSaveDrafts_savesEveryItemAndConsumesEveryReceipt_butStopsAttemptingIndexingAfterTheFirstFailure(t *testing.T) {
	// Given three valid candidates, all of which will persist successfully,
	// but whose embedding call always fails (e.g. the OpenRouter key is
	// missing) — see Design decisions in
	// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md
	ctx := context.Background()
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "" && item.Topic == "Go" &&
			(item.Concept == "first" || item.Concept == "second" || item.Concept == "third")
	})).Return(nil).Times(3)
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "shared evidence quote"},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID != "" && e.SourceLabel != "" && e.Excerpt != "" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Times(3)
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID != ""
	})).Return(nil).Times(3)
	embedErr := errors.New("openrouter api key is missing")
	llm := llmmocks.NewMockProvider(t)
	llm.EXPECT().Embeddings(ctx, domainllm.EmbeddingRequest{Input: "first\n\none"}).Return(domainllm.EmbeddingResponse{}, embedErr).Once()
	chunks := knowledgemocks.NewMockChunkRepository(t)
	tx := txmocks.NewMockTransactor(t)
	runWithinTx(tx)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, knowledgemocks.NewMockVectorStore(t), passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := service.receipts.Create("session-1", "Go", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-1"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "shared evidence quote"}}},
		{Item: domainknowledge.Item{ID: "candidate-2"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "shared evidence quote"}}},
		{Item: domainknowledge.Item{ID: "candidate-3"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "shared evidence quote"}}},
	})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: "Go", Concept: "first", Definition: "one"},
		{ID: "candidate-2", Topic: "Go", Concept: "second", Definition: "two"},
		{ID: "candidate-3", Topic: "Go", Concept: "third", Definition: "three"},
	}

	// When saving the candidates
	savedIndices, err := service.SaveDrafts(ctx, batchID, input)

	// Then every item is still saved and its receipt consumed (indexing
	// failure never blocks a save or a retry), only one embedding call was
	// ever attempted, and the batch reports the indexing failure
	require.Equal(t, []int{0, 1, 2}, savedIndices)
	assert.ErrorIs(t, err, ErrIndexingFailed)
	chunks.AssertNotCalled(t, "DeleteByItemID", mock.Anything, mock.Anything)
	for _, candidateID := range []string{"candidate-1", "candidate-2", "candidate-3"} {
		_, found := service.receipts.Get(batchID, candidateID)
		assert.False(t, found, "receipt for %s should be consumed", candidateID)
	}
}
