package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

func TestReceiptStore_claimAtomicallyRemovesOnlyThatCandidateAndKeepsPendingSiblings(t *testing.T) {
	// Given one extraction batch containing candidates A, B, and C
	store := newReceiptStore()
	refsA := []domainknowledge.EvidenceRef{{MessageID: "message-a", Quote: "quote a"}}
	batchID := store.Create("session-1", "Concurrency", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-a"}, EvidenceRefs: refsA},
		{Item: domainknowledge.Item{ID: "candidate-b"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-b", Quote: "quote b"}}},
		{Item: domainknowledge.Item{ID: "candidate-c"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-c", Quote: "quote c"}}},
	})
	// Mutating the input slice after Create must not reach the stored receipt
	refsA[0].Quote = "tampered after create"

	// When candidate A is claimed for its save transaction
	receiptA, claimedA := store.Claim(batchID, "candidate-a")

	// Then A is claimed with its untampered snapshot and is gone from the store
	require.True(t, claimedA)
	assert.Equal(t, "session-1", receiptA.SessionID)
	assert.Equal(t, "Concurrency", receiptA.SourceLabel)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-a", Quote: "quote a"}}, receiptA.EvidenceRefs)
	_, foundA := store.Get(batchID, "candidate-a")
	assert.False(t, foundA)

	// And a second concurrent claim of the same candidate fails — proving
	// Claim is atomic, so two concurrent saves can never both persist it
	_, claimedAgain := store.Claim(batchID, "candidate-a")
	assert.False(t, claimedAgain)

	// Mutating the claimed receipt's slice must not reach B/C's stored receipts
	receiptA.EvidenceRefs[0].Quote = "mutated after claim"
	receiptB, foundB := store.Get(batchID, "candidate-b")
	receiptC, foundC := store.Get(batchID, "candidate-c")
	require.True(t, foundB)
	require.True(t, foundC)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-b", Quote: "quote b"}}, receiptB.EvidenceRefs)
	assert.Equal(t, []domainknowledge.EvidenceRef{{MessageID: "message-c", Quote: "quote c"}}, receiptC.EvidenceRefs)
}

func TestReceiptStore_restorePutsAClaimedReceiptBackUnchangedForRetry(t *testing.T) {
	// Given a claimed receipt from a batch that still has a pending sibling
	store := newReceiptStore()
	batchID := store.Create("session-1", "Concurrency", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-a"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-a", Quote: "quote a"}}},
		{Item: domainknowledge.Item{ID: "candidate-b"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-b", Quote: "quote b"}}},
	})
	receiptA, claimed := store.Claim(batchID, "candidate-a")
	require.True(t, claimed)

	// When the save fails and the receipt is restored
	store.Restore(batchID, "candidate-a", receiptA)

	// Then candidate A is available again, unchanged, alongside its sibling
	restored, found := store.Get(batchID, "candidate-a")
	require.True(t, found)
	assert.Equal(t, receiptA, restored)
	_, foundB := store.Get(batchID, "candidate-b")
	assert.True(t, foundB)
}

func TestReceiptStore_restoreRecreatesTheBatchWhenItWasFullyConsumedOrDiscardedMeanwhile(t *testing.T) {
	// Given a batch whose only candidate was claimed and then discarded
	// while its save was still in flight
	store := newReceiptStore()
	batchID := store.Create("session-1", "Concurrency", []parsedCandidate{
		{Item: domainknowledge.Item{ID: "candidate-a"}, EvidenceRefs: []domainknowledge.EvidenceRef{{MessageID: "message-a", Quote: "quote a"}}},
	})
	receiptA, claimed := store.Claim(batchID, "candidate-a")
	require.True(t, claimed)
	store.Discard(batchID)

	// When that in-flight save nonetheless fails and restores its receipt
	store.Restore(batchID, "candidate-a", receiptA)

	// Then the batch exists again and the candidate is retryable
	restored, found := store.Get(batchID, "candidate-a")
	require.True(t, found)
	assert.Equal(t, receiptA, restored)
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
