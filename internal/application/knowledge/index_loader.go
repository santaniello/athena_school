package knowledge

import (
	"context"
	"log"
	"sync"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// IndexLoader is the background lifecycle coordinator for the knowledge
// vector index: it loads knowledge.Chunk rows safe to search
// (ChunkRepository.ListCurrent) into a VectorStore, tracks IndexStatus, and
// lets a failed retry keep the previous snapshot searchable rather than
// mislabeling working search as globally failed. See
// specs/phases/phase-02-knowledge-engine/04-vector-search.md.
//
// LoadInitial is launched fire-and-forget from Wails' OnDomReady, with no
// synchronous caller to hand a technical error back to — so, uniquely in
// this application layer, IndexLoader logs its own load failures directly
// instead of leaving that to a desktop binding.
type IndexLoader struct {
	chunks         domainknowledge.ChunkRepository
	store          domainknowledge.VectorStore
	embeddingModel string

	mu     sync.Mutex
	status domainknowledge.IndexStatus

	// opMu is held for the full duration of one reload (LoadInitial/Retry:
	// ListCurrent through ReplaceAll) or one mutation reservation
	// (BeginMutation/EndMutation: transaction commit through VectorStore
	// Add/Remove), never both at once. That closes the race where a reload
	// reads a pre-commit snapshot and publishes it after a concurrent
	// mutation's own Add/Remove already ran, resurrecting stale content —
	// and, since reload holds it too, the same lock also serializes two
	// reloads (LoadInitial racing a fast Retry, or two Retries) so their
	// ReplaceAll calls can never land out of order.
	opMu sync.Mutex
}

// NewIndexLoader creates an IndexLoader that has not started loading yet:
// Status already reports IndexStateLoading with no snapshot, so there is no
// window where an unstarted loader looks ready or failed.
func NewIndexLoader(
	chunks domainknowledge.ChunkRepository, store domainknowledge.VectorStore, embeddingModel string,
) *IndexLoader {
	return &IndexLoader{
		chunks:         chunks,
		store:          store,
		embeddingModel: embeddingModel,
		status:         domainknowledge.IndexStatus{State: domainknowledge.IndexStateLoading},
	}
}

// Status returns the coordinator's current lifecycle snapshot.
func (l *IndexLoader) Status() domainknowledge.IndexStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.status
}

// CheckMutationAllowed returns ErrIndexLoading while an initial load or a
// retry is in progress, and nil otherwise. ingest.Service and Service both
// call this (via their own locally defined guard interface) before any
// knowledge mutation, so a retry snapshot can never be overwritten by — or
// silently lose — a concurrent change.
func (l *IndexLoader) CheckMutationAllowed() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.status.State == domainknowledge.IndexStateLoading {
		return ErrIndexLoading
	}
	return nil
}

// BeginMutation reserves the index against a concurrent reload for the
// entire duration of one knowledge mutation — from its transaction commit
// through its post-commit VectorStore reconciliation — unlike
// CheckMutationAllowed's point-in-time read. It never blocks: if a reload
// currently holds the reservation, it returns ErrIndexLoading immediately,
// same contract as CheckMutationAllowed. Callers must call EndMutation
// exactly once after a nil return, even on the mutation's own later error,
// typically via `defer`.
func (l *IndexLoader) BeginMutation() error {
	if !l.opMu.TryLock() {
		return ErrIndexLoading
	}
	return nil
}

// EndMutation releases the reservation acquired by a successful
// BeginMutation.
func (l *IndexLoader) EndMutation() {
	l.opMu.Unlock()
}

// LoadInitial performs the first load. A failure marks the index failed —
// there is no prior snapshot to fall back to yet.
func (l *IndexLoader) LoadInitial(ctx context.Context) {
	l.setStatus(domainknowledge.IndexStatus{State: domainknowledge.IndexStateLoading})
	l.setStatus(l.reload(ctx))
}

// Retry rebuilds a separate snapshot away from the active one — Search
// keeps serving the previous snapshot throughout, since reload only
// publishes into the store at the very end. A failed retry restores the
// preceding ready state (keeping the old snapshot and its HasSnapshot=true)
// rather than reporting a global failure, while still recording the
// retry's own LastError.
//
// Reading the previous status, reloading, deciding whether to restore it,
// and publishing the final status are all done under opMu as one atomic
// operation — not just the reload — so a second concurrent Retry can never
// read a first Retry's transient Loading status as "previous" and later
// republish that Loading status as if it were a terminal one.
func (l *IndexLoader) Retry(ctx context.Context) domainknowledge.IndexStatus {
	l.opMu.Lock()
	defer l.opMu.Unlock()

	previous := l.Status()
	l.setStatus(domainknowledge.IndexStatus{State: domainknowledge.IndexStateLoading, HasSnapshot: previous.HasSnapshot})

	status := l.doReload(ctx)
	if status.State == domainknowledge.IndexStateFailed && previous.HasSnapshot {
		status = domainknowledge.IndexStatus{
			State:       previous.State,
			HasSnapshot: true,
			Issues:      previous.Issues,
			LastError:   status.LastError,
		}
	}
	l.setStatus(status)
	return status
}

func (l *IndexLoader) setStatus(status domainknowledge.IndexStatus) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.status = status
}

// reload acquires opMu for the duration of doReload — used directly by
// LoadInitial, which (unlike Retry) has no "previous" status to protect.
func (l *IndexLoader) reload(ctx context.Context) domainknowledge.IndexStatus {
	l.opMu.Lock()
	defer l.opMu.Unlock()
	return l.doReload(ctx)
}

// doReload lists current chunks, re-validates them as defense-in-depth
// against a repository-layer bug, publishes the valid subset atomically,
// and reports ready/ready_with_warnings/failed. It never touches l.status
// itself — callers decide how to fold the result in (LoadInitial publishes
// it directly, Retry may restore a prior snapshot on failure instead).
// Callers must hold opMu.
func (l *IndexLoader) doReload(ctx context.Context) domainknowledge.IndexStatus {
	result, err := l.chunks.ListCurrent(ctx, l.embeddingModel)
	if err != nil {
		log.Printf("knowledge index: loading current chunks: %v", err)
		return domainknowledge.IndexStatus{
			State:     domainknowledge.IndexStateFailed,
			LastError: "Could not load the knowledge index from the database.",
		}
	}

	valid := make([]domainknowledge.Chunk, 0, len(result.Chunks))
	issues := append([]domainknowledge.ChunkLoadIssue{}, result.Issues...)
	for _, chunk := range result.Chunks {
		if validateErr := domainknowledge.ValidateChunk(chunk); validateErr != nil {
			issues = append(issues, domainknowledge.ChunkLoadIssue{
				ChunkID: chunk.ID, ItemID: chunk.ItemID, Source: chunk.Source, FilePath: chunk.FilePath,
				Reason: domainknowledge.ReasonForValidationError(validateErr),
			})
			continue
		}
		valid = append(valid, chunk)
	}

	if err := l.store.ReplaceAll(ctx, valid); err != nil {
		log.Printf("knowledge index: publishing snapshot: %v", err)
		return domainknowledge.IndexStatus{
			State:     domainknowledge.IndexStateFailed,
			LastError: "Could not publish the loaded knowledge index.",
		}
	}

	state := domainknowledge.IndexStateReady
	if len(issues) > 0 {
		state = domainknowledge.IndexStateReadyWithWarnings
	}
	return domainknowledge.IndexStatus{State: state, HasSnapshot: true, Issues: issues}
}
