package knowledge

import "errors"

// ErrMalformedExtraction is returned when an LLM response has no valid JSON envelope.
var ErrMalformedExtraction = errors.New("malformed knowledge extraction response")

// ErrTranscriptTooLarge is returned when no complete transcript message fits
// within the extraction budget.
var ErrTranscriptTooLarge = errors.New("no complete transcript message fits within the extraction limit")

// ErrIndexLoading is returned by IndexLoader.CheckMutationAllowed while an
// initial load or a retry is in progress, so a concurrent knowledge
// mutation can never race a snapshot publish.
var ErrIndexLoading = errors.New("knowledge index is loading")

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
