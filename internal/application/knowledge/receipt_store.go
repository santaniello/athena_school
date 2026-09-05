package knowledge

import (
	"sync"
	"time"

	"github.com/google/uuid"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// reconciliationClassification is the transient result of comparing one
// extraction candidate against its duplicate shortlist (see
// Service.classifyCandidates) — everything Apply/Resolve/Acknowledge/
// SaveForReview need to act on the user's eventual decision. It lives only
// in the candidate's receipt until then, mirroring EvidenceRefs.
type reconciliationClassification struct {
	Action          string
	TargetItemID    string
	TargetUpdatedAt time.Time
	Reason          string
	Changes         domainknowledge.ItemChanges
}

type candidateReceipt struct {
	SessionID      string
	SourceLabel    string
	EvidenceRefs   []domainknowledge.EvidenceRef
	Reconciliation reconciliationClassification
}

type receiptStore struct {
	mu      sync.RWMutex
	batches map[string]map[string]candidateReceipt
	// discarded remembers every batch ID Discard has ever removed, so a
	// Restore racing an in-flight save against a concurrent Discard cannot
	// resurrect a batch the user explicitly dismissed. Batch IDs are
	// per-extraction UUIDs, never reused, so this only ever grows — bounded
	// in practice by how many extractions one long-running session performs.
	discarded map[string]struct{}
}

func newReceiptStore() *receiptStore {
	return &receiptStore{
		batches:   make(map[string]map[string]candidateReceipt),
		discarded: make(map[string]struct{}),
	}
}

// Create stores one receipt per candidate, pairing each with its
// classification at the same index (see Service.classifyCandidates) —
// classifications must be the same length as candidates, in the same
// order.
func (s *receiptStore) Create(sessionID, sourceLabel string, candidates []parsedCandidate, classifications []reconciliationClassification) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	batchID := uuid.NewString()
	receipts := make(map[string]candidateReceipt, len(candidates))
	for index, candidate := range candidates {
		receipts[candidate.ID] = candidateReceipt{
			SessionID:      sessionID,
			SourceLabel:    sourceLabel,
			EvidenceRefs:   append([]domainknowledge.EvidenceRef(nil), candidate.EvidenceRefs...),
			Reconciliation: classifications[index],
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
// persisting the candidate — keeping it available for retry. It is a no-op
// for a batch the user has since discarded: a save that was still in flight
// when Discard ran must not resurrect a receipt the user explicitly
// dismissed. A batch merely drained by every candidate being claimed (not
// discarded) is recreated normally.
func (s *receiptStore) Restore(batchID, candidateID string, receipt candidateReceipt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, discarded := s.discarded[batchID]; discarded {
		return
	}
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
	s.discarded[batchID] = struct{}{}
}
