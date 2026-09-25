package knowledge

import "errors"

// ErrIndexLoading is returned by IndexLoader.CheckMutationAllowed while an
// initial load or a retry is in progress, so a concurrent knowledge
// mutation can never race a snapshot publish.
var ErrIndexLoading = errors.New("knowledge index is loading")

// errExactDuplicateAtSave signals createReconciledItem's transaction closure
// that its exact-match recheck found a duplicate. See
// specs/phases/phase-02-knowledge-engine/10-01-duplicate-detection-decisions.md
// Decision 3, and its addendum on running the recheck inside the same
// transaction as the write to close the check-then-act race a separate,
// pre-transaction lookup would leave open.
var errExactDuplicateAtSave = errors.New("knowledge: exact duplicate at save time")

// ErrReconciliationTargetStale is returned when a proposal's classified
// target was edited or removed since classification; the caller must run
// reconciliation again.
var ErrReconciliationTargetStale = errors.New("knowledge: reconciliation target changed since comparison; run reconciliation again")

// ErrReconciliationResolutionInvalid is returned when a conflict
// resolution is not one of ConflictKeepExisting, ConflictUpdateExisting,
// or ConflictCreateSeparately.
var ErrReconciliationResolutionInvalid = errors.New("knowledge: unknown conflict resolution")

// ErrReconciliationProposalNotPending is returned when an action from
// Knowledge Review targets a proposal that has already been applied,
// rejected, or marked stale.
var ErrReconciliationProposalNotPending = errors.New("knowledge: reconciliation proposal is not pending")

// ErrIndexingFailed is the sentinel every knowledge-indexing failure wraps
// — embedding, chunk persistence, or VectorStore reconciliation alike — so
// every caller can distinguish "item saved but not indexed" from a real
// write failure with a single errors.Is(err, ErrIndexingFailed) check. The
// item is always already persisted by the time this can be returned; see
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md's
// Failure policy.
var ErrIndexingFailed = errors.New("knowledge item saved but indexing failed")

// IndexingWarning wraps a post-commit VectorStore reconciliation failure
// (Approve/Deprecate/UpdateItem/DeleteItem/ingest all reconcile the store
// after their SQLite transaction has already committed). The durable
// mutation is never rolled back for this — callers that receive an
// IndexingWarning (via errors.As, or errors.Is against ErrIndexingFailed)
// must still treat the operation as a successful durable write, logging the
// technical failure and surfacing a retryable warning rather than a false
// write failure.
type IndexingWarning struct {
	// Err is the underlying VectorStore error (Add/Remove/ReplaceAll).
	Err error
}

func (w *IndexingWarning) Error() string {
	return "knowledge index reconciliation failed: " + w.Err.Error()
}

// Unwrap exposes both the underlying VectorStore error (so a caller can
// still errors.Is/As against it specifically) and ErrIndexingFailed (so
// every call site can standardize on one check regardless of which
// operation produced the warning).
func (w *IndexingWarning) Unwrap() []error {
	return []error{w.Err, ErrIndexingFailed}
}
