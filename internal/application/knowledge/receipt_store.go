package knowledge

import (
	"sync"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

type candidateReceipt struct {
	SessionID    string
	SourceLabel  string
	EvidenceRefs []domainknowledge.EvidenceRef
}

type receiptStore struct {
	mu      sync.RWMutex
	batches map[string]map[string]candidateReceipt
}

func newReceiptStore() *receiptStore {
	return &receiptStore{batches: make(map[string]map[string]candidateReceipt)}
}

func (s *receiptStore) Create(sessionID, sourceLabel string, candidates []parsedCandidate) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	batchID := uuid.NewString()
	receipts := make(map[string]candidateReceipt, len(candidates))
	for _, candidate := range candidates {
		receipts[candidate.ID] = candidateReceipt{
			SessionID:    sessionID,
			SourceLabel:  sourceLabel,
			EvidenceRefs: append([]domainknowledge.EvidenceRef(nil), candidate.EvidenceRefs...),
		}
	}
	s.batches[batchID] = receipts
	return batchID
}

func (s *receiptStore) Get(batchID, candidateID string) (candidateReceipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	receipt, found := s.batches[batchID][candidateID]
	receipt.EvidenceRefs = append([]domainknowledge.EvidenceRef(nil), receipt.EvidenceRefs...)
	return receipt, found
}

// Claim atomically removes and returns candidateID's receipt, so at most one
// concurrent SaveDrafts/SaveAndApprove call can save it — a plain Get
// followed by a later Consume would let two concurrent callers both read the
// same still-present receipt and persist the candidate twice. A save that
// does not end up persisting the candidate (an invalid quote, a failed
// transaction, ...) must call Restore to put the receipt back for retry.
func (s *receiptStore) Claim(batchID, candidateID string) (candidateReceipt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipts, found := s.batches[batchID]
	if !found {
		return candidateReceipt{}, false
	}
	receipt, found := receipts[candidateID]
	if !found {
		return candidateReceipt{}, false
	}
	delete(receipts, candidateID)
	if len(receipts) == 0 {
		delete(s.batches, batchID)
	}
	receipt.EvidenceRefs = append([]domainknowledge.EvidenceRef(nil), receipt.EvidenceRefs...)
	return receipt, true
}

// Restore puts a claimed receipt back — after a save that did not end up
// persisting the candidate — keeping it available for retry.
func (s *receiptStore) Restore(batchID, candidateID string, receipt candidateReceipt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipts, found := s.batches[batchID]
	if !found {
		receipts = make(map[string]candidateReceipt, 1)
		s.batches[batchID] = receipts
	}
	receipts[candidateID] = receipt
}

func (s *receiptStore) Discard(batchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.batches, batchID)
}
