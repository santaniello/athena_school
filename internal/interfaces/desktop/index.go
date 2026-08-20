package desktop

import (
	"context"

	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// eventKnowledgeIndexStatus is emitted whenever the vector index
// coordinator's lifecycle status changes: after the initial background
// load (StartKnowledgeIndex) and after every retry (RetryKnowledgeIndex).
const eventKnowledgeIndexStatus = "knowledge-index:status"

// ChunkLoadIssueResult is the desktop-facing DTO for one isolated,
// excluded chunk. Only safe fields are carried — Reason is a stable code
// the frontend maps to English copy, never a raw technical error.
type ChunkLoadIssueResult struct {
	ChunkID  string `json:"chunkId"`
	ItemID   string `json:"itemId"`
	Source   string `json:"source"`
	FilePath string `json:"filePath"`
	Reason   string `json:"reason"`
}

// IndexStatusResult is the desktop-facing DTO for the vector index
// coordinator's lifecycle snapshot.
type IndexStatusResult struct {
	State       string                 `json:"state"`
	HasSnapshot bool                   `json:"hasSnapshot"`
	Issues      []ChunkLoadIssueResult `json:"issues"`
	LastError   string                 `json:"lastError"`
}

func toIndexStatusResult(status domainknowledge.IndexStatus) IndexStatusResult {
	issues := make([]ChunkLoadIssueResult, len(status.Issues))
	for i, issue := range status.Issues {
		issues[i] = ChunkLoadIssueResult{
			ChunkID: issue.ChunkID, ItemID: issue.ItemID, Source: issue.Source,
			FilePath: issue.FilePath, Reason: issue.Reason,
		}
	}
	return IndexStatusResult{
		State:       string(status.State),
		HasSnapshot: status.HasSnapshot,
		Issues:      issues,
		LastError:   status.LastError,
	}
}

// GetKnowledgeIndexStatus returns the vector index coordinator's current
// lifecycle snapshot. The frontend registers the "knowledge-index:status"
// event listener before calling this, closing the race where a fast
// background load finishes before the listener subscribes.
func (a *App) GetKnowledgeIndexStatus() IndexStatusResult {
	return toIndexStatusResult(a.index.Status())
}

// RetryKnowledgeIndex rebuilds a separate snapshot from SQLite. The
// previous snapshot stays searchable throughout — including on failure,
// where the preceding ready state is restored — since the coordinator only
// publishes atomically at the very end. Emits "knowledge-index:status"
// with the outcome.
func (a *App) RetryKnowledgeIndex() IndexStatusResult {
	result := toIndexStatusResult(a.index.Retry(a.ctx))
	a.emit(a.ctx, eventKnowledgeIndexStatus, result)
	return result
}

// StartKnowledgeIndex performs the initial background load and emits
// "knowledge-index:status" with the outcome. Called from Wails' OnDomReady
// lifecycle hook (see main.go) in its own goroutine so the window can
// render before chunks are decoded — not meant to be called by the
// frontend directly; it is exported only because Wails binds every
// exported App method, the same reason Startup is exported.
func (a *App) StartKnowledgeIndex(ctx context.Context) {
	a.index.LoadInitial(ctx)
	a.emit(ctx, eventKnowledgeIndexStatus, toIndexStatusResult(a.index.Status()))
}
