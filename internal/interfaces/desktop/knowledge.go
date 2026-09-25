package desktop

import (
	"errors"
	"log"
	"strings"
	"time"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// Sentinel errors from the pending reconciliation bindings' own input
// validation, checked before ever calling the knowledge service — proposalID
// as an opaque non-empty token, and status/resolution against their known
// enums. Keeps these Wails bindings thin adapters: validate
// input, call the use case, return the result (see AGENTS.md).
var (
	ErrReconciliationProposalIDRequired = errors.New("desktop: reconciliation proposal id is required")
	ErrReconciliationStatusInvalid      = errors.New("desktop: status must be draft or approved")
	ErrReconciliationResolutionInvalid  = errors.New("desktop: resolution must be keep_existing, update_existing, or create_separately")
)

// validateReconciliationProposalID checks the single id every pending
// reconciliation binding takes.
func validateReconciliationProposalID(proposalID string) error {
	if strings.TrimSpace(proposalID) == "" {
		return ErrReconciliationProposalIDRequired
	}
	return nil
}

// validateKnowledgeStatus checks a target Knowledge Item status against
// the only two a reconciliation "create" action may persist at.
func validateKnowledgeStatus(status string) error {
	if status != domainknowledge.StatusDraft && status != domainknowledge.StatusApproved {
		return ErrReconciliationStatusInvalid
	}
	return nil
}

// validateConflictResolution checks a conflict resolution against the
// three explicit outcomes ResolveReconciliationConflict/
// ResolvePendingReconciliationConflict accept.
func validateConflictResolution(resolution string) error {
	switch resolution {
	case applicationknowledge.ConflictKeepExisting, applicationknowledge.ConflictUpdateExisting, applicationknowledge.ConflictCreateSeparately:
		return nil
	default:
		return ErrReconciliationResolutionInvalid
	}
}

// KnowledgeItemResult is a knowledge item returned to the UI.
type KnowledgeItemResult struct {
	ID              string   `json:"id"`
	Topic           string   `json:"topic"`
	Concept         string   `json:"concept"`
	Definition      string   `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"tradeOffs"`
	RelatedConcepts []string `json:"relatedConcepts"`
	Source          string   `json:"source"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// ItemChangesResult is the desktop-facing DTO for a
// domainknowledge.ItemChanges — optional replacements for an existing
// Item's user-editable content fields. Every field uses `omitempty` on a
// pointer (never on the slice itself, which would also drop an explicit
// empty list): a nil Definition/list means "unchanged" and is omitted
// from the wire format entirely, while a non-nil list — even an empty one
// — means "set to this" and is always serialized, distinguishing "leave
// this field alone" from "clear it".
type ItemChangesResult struct {
	Definition      *string   `json:"definition,omitempty"`
	Properties      *[]string `json:"properties,omitempty"`
	TradeOffs       *[]string `json:"tradeOffs,omitempty"`
	RelatedConcepts *[]string `json:"relatedConcepts,omitempty"`
}

func toItemChangesResult(changes domainknowledge.ItemChanges) ItemChangesResult {
	return ItemChangesResult{
		Definition: changes.Definition, Properties: listPointer(changes.Properties),
		TradeOffs: listPointer(changes.TradeOffs), RelatedConcepts: listPointer(changes.RelatedConcepts),
	}
}

// listPointer returns nil for a nil list — "unchanged" — and otherwise a
// pointer to list itself, even when it is empty, so `omitempty` never
// mistakes an explicit empty list for an absent one.
func listPointer(list []string) *[]string {
	if list == nil {
		return nil
	}
	return &list
}

// KnowledgeItemInput mirrors the full candidate returned by ExtractKnowledge.
type KnowledgeItemInput struct {
	ID              string   `json:"id"`
	Topic           string   `json:"topic"`
	Concept         string   `json:"concept"`
	Definition      string   `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"tradeOffs"`
	RelatedConcepts []string `json:"relatedConcepts"`
	Source          string   `json:"source"`
	Status          string   `json:"status"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// ListKnowledgeItems returns every Item matching topic/status. An empty
// topic or status means no constraint on that field.
func (a *App) ListKnowledgeItems(topic, status string) ([]KnowledgeItemResult, error) {
	items, err := a.knowledge.ListItems(a.ctx, topic, status)
	if err != nil {
		return nil, err
	}
	results := make([]KnowledgeItemResult, len(items))
	for index, item := range items {
		results[index] = toKnowledgeItemResult(item)
	}
	return results, nil
}

// KnowledgeEvidenceResult is an immutable Evidence snapshot shown in the
// Knowledge Explorer's Evidence section.
type KnowledgeEvidenceResult struct {
	OriginType  string `json:"originType"`
	SourceLabel string `json:"sourceLabel"`
	Excerpt     string `json:"excerpt"`
	CreatedAt   string `json:"createdAt"`
}

// ListKnowledgeItemEvidence returns id's persisted Evidence snapshots, in
// deterministic order — empty for a legacy or shadow Item.
func (a *App) ListKnowledgeItemEvidence(id string) ([]KnowledgeEvidenceResult, error) {
	evidence, err := a.knowledge.ListItemEvidence(a.ctx, id)
	if err != nil {
		return nil, err
	}
	results := make([]KnowledgeEvidenceResult, len(evidence))
	for index, e := range evidence {
		results[index] = KnowledgeEvidenceResult{
			OriginType: e.OriginType, SourceLabel: e.SourceLabel, Excerpt: e.Excerpt,
			CreatedAt: e.CreatedAt.Format(time.RFC3339Nano),
		}
	}
	return results, nil
}

// ListKnowledgeTopics returns every distinct topic, alphabetically.
func (a *App) ListKnowledgeTopics() ([]string, error) {
	return a.knowledge.ListTopics(a.ctx)
}

// CountDraftKnowledgeItems returns how many Items currently have draft
// status, for the sidebar review badge.
func (a *App) CountDraftKnowledgeItems() (int, error) {
	return a.knowledge.CountDrafts(a.ctx)
}

// ApproveKnowledgeItem transitions id from draft to approved and returns
// the updated item.
func (a *App) ApproveKnowledgeItem(id string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.Approve(a.ctx, id)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("approving item "+id, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// DeprecateKnowledgeItem transitions id from approved to deprecated and
// returns the updated item.
func (a *App) DeprecateKnowledgeItem(id string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.Deprecate(a.ctx, id)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("deprecating item "+id, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// UpdateKnowledgeItem overwrites id's editable fields and returns the
// updated item. Status, Source and CreatedAt are never touched.
func (a *App) UpdateKnowledgeItem(id string, input KnowledgeItemInput) (KnowledgeItemResult, error) {
	item, err := a.knowledge.UpdateItem(a.ctx, id, applicationknowledge.ItemFields{
		Topic:           input.Topic,
		Concept:         input.Concept,
		Definition:      input.Definition,
		Properties:      input.Properties,
		TradeOffs:       input.TradeOffs,
		RelatedConcepts: input.RelatedConcepts,
	})
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("updating item "+id, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// DeleteKnowledgeItem permanently removes id and every chunk it owns. This
// cannot be undone.
func (a *App) DeleteKnowledgeItem(id string) error {
	err := a.knowledge.DeleteItem(a.ctx, id)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("deleting item "+id, err)
		return nil
	}
	return err
}

// logIndexingFailure logs an indexing failure (errors.Is(err,
// applicationknowledge.ErrIndexingFailed)) — embedding, chunk persistence,
// or VectorStore reconciliation alike. The durable Knowledge Item mutation
// already succeeded, so every caller reports success to the frontend
// regardless; the backfill flow (CountUnindexedKnowledgeItems / "Index
// now") is the recovery path, and self-heals from SQLite on the next
// startup even without it.
func logIndexingFailure(op string, err error) {
	log.Printf("knowledge index: %s: %v", op, err)
}

func toKnowledgeItemResult(item domainknowledge.Item) KnowledgeItemResult {
	return KnowledgeItemResult{
		ID: item.ID, Topic: item.Topic, Concept: item.Concept, Definition: item.Definition,
		Properties: item.Properties, TradeOffs: item.TradeOffs, RelatedConcepts: item.RelatedConcepts,
		Source: item.Source, Status: item.Status,
		CreatedAt: item.CreatedAt.Format(time.RFC3339Nano), UpdatedAt: item.UpdatedAt.Format(time.RFC3339Nano),
	}
}

// ReindexProgressResult is the desktop-facing DTO for
// ReindexKnowledgeItems' progress callback.
type ReindexProgressResult struct {
	ItemsProcessed int    `json:"itemsProcessed"`
	ItemsTotal     int    `json:"itemsTotal"`
	CurrentTopic   string `json:"currentTopic"`
}

// ReindexFailureResult is the desktop-facing DTO for one item that failed
// to index during a reindex run.
type ReindexFailureResult struct {
	ItemID string `json:"itemId"`
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
}

// ReindexSummaryResult is the desktop-facing DTO for ReindexKnowledgeItems'
// final report.
type ReindexSummaryResult struct {
	ItemsProcessed int                    `json:"itemsProcessed"`
	ItemsIndexed   int                    `json:"itemsIndexed"`
	ItemsFailed    int                    `json:"itemsFailed"`
	Failures       []ReindexFailureResult `json:"failures"`
}

func toReindexProgressResult(p applicationknowledge.ReindexProgress) ReindexProgressResult {
	return ReindexProgressResult{
		ItemsProcessed: p.ItemsProcessed, ItemsTotal: p.ItemsTotal, CurrentTopic: p.CurrentTopic,
	}
}

func toReindexSummaryResult(s applicationknowledge.ReindexSummary) ReindexSummaryResult {
	failures := make([]ReindexFailureResult, len(s.Failures))
	for index, failure := range s.Failures {
		failures[index] = ReindexFailureResult{ItemID: failure.ItemID, Topic: failure.Topic, Reason: failure.Reason}
	}
	return ReindexSummaryResult{
		ItemsProcessed: s.ItemsProcessed, ItemsIndexed: s.ItemsIndexed, ItemsFailed: s.ItemsFailed, Failures: failures,
	}
}

// CountUnindexedKnowledgeItems returns how many Knowledge Items currently
// lack a current chunk — the count the Knowledge Explorer's backfill alert
// shows on mount.
func (a *App) CountUnindexedKnowledgeItems() (int, error) {
	return a.knowledge.CountUnindexedItems(a.ctx)
}

// ReindexKnowledgeItems processes every currently-unindexed Knowledge Item
// — the recovery path for every indexing failure documented in
// specs/phases/phase-02-knowledge-engine/08-knowledge-item-indexing.md, run
// only on explicit user consent ("Index now"). It streams progress via
// "ingest:progress", reusing 2.3's events (the UI only ever has one such
// operation active at a time), then emits "ingest:done" with the final
// summary, or "ingest:error" on failure.
func (a *App) ReindexKnowledgeItems() error {
	summary, err := a.knowledge.ReindexKnowledgeItems(a.ctx, func(p applicationknowledge.ReindexProgress) error {
		a.emit(a.ctx, eventIngestProgress, toReindexProgressResult(p))
		return nil
	})
	if err != nil {
		a.emit(a.ctx, eventIngestError, err.Error())
		return err
	}
	a.emit(a.ctx, eventIngestDone, toReindexSummaryResult(summary))
	return nil
}
