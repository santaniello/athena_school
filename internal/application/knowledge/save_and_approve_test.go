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
	llmmocks "github.com/santaniello/athena/internal/domain/llm/mocks"
	domainstudy "github.com/santaniello/athena/internal/domain/study"
	studymocks "github.com/santaniello/athena/internal/domain/study/mocks"
)

func TestSaveAndApprove_revalidatesAgainstTheReceiptAndPersistsDirectlyAsApproved(t *testing.T) {
	// Given a backend receipt tying candidate "candidate-1" to a session
	// Message — see specs/Athena.md §12 ("Salvar como conhecimento" skips
	// the draft stage)
	ctx := context.Background()
	var savedItemID string
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool {
		return item.ID != "candidate-1" && item.ID != "" &&
			item.Topic == "Go" && item.Concept == "Channels" && item.Definition == "Typed conduits." &&
			item.Source == domainknowledge.SourceAthena && item.Status == domainknowledge.StatusApproved &&
			!item.CreatedAt.IsZero() && item.CreatedAt.Equal(item.UpdatedAt)
	})).Run(func(_ context.Context, item domainknowledge.Item) { savedItemID = item.ID }).Return(nil).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "Channels are typed conduits."},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID == "message-1" && e.SourceLabel == "Go" &&
			e.Excerpt == "Channels are typed conduits." && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID == savedItemID
	})).Return(nil).Once()
	llm := llmmocks.NewMockProvider(t)
	chunks := knowledgemocks.NewMockChunkRepository(t)
	store := knowledgemocks.NewMockVectorStore(t)
	tx := txmocks.NewMockTransactor(t)
	expectSuccessfulIndexing(ctx, llm, chunks, store, tx, 1)
	service := NewService(repository, studymocks.NewMockSessionRepository(t), messages, llm, configmocks.NewMockStore(t), chunks, tx, store, passingIndexGuard(t), domainknowledge.RetrievalThresholds{}, evidenceRepo)
	batchID := service.receipts.Create("session-1", "Go", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-1"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-1", Quote: "Channels are typed conduits."}}},
	})
	input := []domainknowledge.Item{
		{ID: "candidate-1", Topic: " Go ", Concept: " Channels ", Definition: " Typed conduits. ", Source: domainknowledge.SourceImportedDoc, Status: domainknowledge.StatusDraft},
	}

	// When saving and approving the candidate
	savedIndices, err := service.SaveAndApprove(ctx, batchID, input)

	// Then it is persisted and indexed directly as approved
	require.NoError(t, err)
	assert.Equal(t, []int{0}, savedIndices)
}

func TestSaveAndApprove_stopsAtTransactionFailureAndKeepsThatReceiptForRetry(t *testing.T) {
	// Given two candidates, the second of which fails to persist
	ctx := context.Background()
	var savedItemID string
	repository := knowledgemocks.NewMockRepository(t)
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "first" })).
		Run(func(_ context.Context, item domainknowledge.Item) { savedItemID = item.ID }).Return(nil).Once()
	saveErr := errors.New("database locked")
	repository.EXPECT().Save(ctx, mock.MatchedBy(func(item domainknowledge.Item) bool { return item.Concept == "second" })).Return(saveErr).Once()
	messages := studymocks.NewMockMessageRepository(t)
	messages.EXPECT().ListBySession(ctx, "session-1").Return([]domainstudy.Message{
		{ID: "message-1", Content: "shared evidence quote"},
	}, nil).Once()
	evidenceRepo := knowledgemocks.NewMockEvidenceRepository(t)
	evidenceRepo.EXPECT().GetOrCreate(ctx, mock.MatchedBy(func(e domainknowledge.Evidence) bool {
		return e.ID != "" && e.OriginType == domainknowledge.OriginSessionMessage &&
			e.OriginID == "message-1" && e.SourceLabel == "Go" &&
			e.Excerpt == "shared evidence quote" && !e.CreatedAt.IsZero()
	})).Return(domainknowledge.Evidence{ID: "evidence-1"}, nil).Once()
	evidenceRepo.EXPECT().LinkToItem(ctx, mock.MatchedBy(func(link domainknowledge.ItemEvidence) bool {
		return link.EvidenceID == "evidence-1" && link.ItemID == savedItemID
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

	// When saving and approving both candidates
	savedIndices, err := service.SaveAndApprove(ctx, batchID, input)

	// Then processing stops at the failed candidate, whose receipt is kept for retry
	assert.ErrorIs(t, err, saveErr)
	assert.Equal(t, []int{0}, savedIndices)
	_, secondStillFound := service.receipts.Get(batchID, "candidate-2")
	assert.True(t, secondStillFound)
}
