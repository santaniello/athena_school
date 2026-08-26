package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

func TestReceiptStore_consumeRemovesOnlySavedCandidateAndKeepsPendingSiblings(t *testing.T) {
	// Given one extraction batch containing candidates A, B, and C
	store := newReceiptStore()
	batchID := store.Create("session-1", "Concurrency", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-a"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-a", Quote: "quote a"}}},
		{Item: domainknowledge.Item{ID: "candidate-b"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-b", Quote: "quote b"}}},
		{Item: domainknowledge.Item{ID: "candidate-c"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-c", Quote: "quote c"}}},
	})

	// When candidate A is consumed after its successful transaction
	store.Consume(batchID, "candidate-a")

	// Then A is gone while B and C retain their exact extraction receipts
	_, foundA := store.Get(batchID, "candidate-a")
	receiptB, foundB := store.Get(batchID, "candidate-b")
	receiptC, foundC := store.Get(batchID, "candidate-c")
	assert.False(t, foundA)
	require.True(t, foundB)
	require.True(t, foundC)
	assert.Equal(t, "session-1", receiptB.SessionID)
	assert.Equal(t, "Concurrency", receiptB.SourceLabel)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-b", Quote: "quote b"}}, receiptB.EvidenceRefs)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-c", Quote: "quote c"}}, receiptC.EvidenceRefs)
}

func TestReceiptStore_discardRemovesEveryPendingCandidateInBatch(t *testing.T) {
	// Given an extraction batch containing pending candidates
	store := newReceiptStore()
	batchID := store.Create("session-1", "Concurrency", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-a"}},
		{Item: domainknowledge.Item{ID: "candidate-b"}},
	})

	// When the user dismisses the batch
	store.Discard(batchID)

	// Then no pending candidate receipt remains
	_, foundA := store.Get(batchID, "candidate-a")
	_, foundB := store.Get(batchID, "candidate-b")
	assert.False(t, foundA)
	assert.False(t, foundB)
}
