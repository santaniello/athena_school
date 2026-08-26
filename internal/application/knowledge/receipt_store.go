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

func (s *receiptStore) Consume(batchID, candidateID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	receipts, found := s.batches[batchID]
	if !found {
		return
	}
	delete(receipts, candidateID)
	if len(receipts) == 0 {
		delete(s.batches, batchID)
	}
}

func (s *receiptStore) Discard(batchID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.batches, batchID)
}
