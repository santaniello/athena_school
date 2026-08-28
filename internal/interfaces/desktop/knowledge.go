package desktop

import (
	"errors"
	"fmt"
	"log"
	"time"

	applicationknowledge "github.com/santaniello/athena/internal/application/knowledge"
	domainknowledge "github.com/santaniello/athena/internal/domain/knowledge"
)

// KnowledgeItemResult is an unpersisted extraction candidate returned to the UI.
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
	// Duplicates, SemanticCheckUnavailable, Reconciliation and
	// ReconciliationFailed are only ever populated by ExtractKnowledge —
	// every other KnowledgeItemResult producer (List, Approve, Deprecate,
	// Update, and the reconciliation apply/resolve bindings below) leaves
	// them at their zero value, since duplicate detection and
	// reconciliation classification both run at extraction time only. See
	// specs/phases/phase-02-knowledge-engine/10-duplicate-detection.md and
	// 11-knowledge-reconciliation.md.
	Duplicates               []DuplicateMatchResult          `json:"duplicates"`
	SemanticCheckUnavailable bool                            `json:"semanticCheckUnavailable"`
	Reconciliation           *ReconciliationSuggestionResult `json:"reconciliation"`
	ReconciliationFailed     bool                            `json:"reconciliationFailed"`
}

// DuplicateMatchResult is the desktop-facing DTO for a domainknowledge.DuplicateMatch.
type DuplicateMatchResult struct {
	ItemID    string  `json:"itemId"`
	Concept   string  `json:"concept"`
	Status    string  `json:"status"`
	MatchType string  `json:"matchType"`
	Score     float64 `json:"score"`
}

// ReconciliationSuggestionResult is the desktop-facing DTO for an
// applicationknowledge.ReconciliationSuggestion — the classifier's
// suggested action for one extraction candidate, before the user has
// decided anything.
type ReconciliationSuggestionResult struct {
	Action       string            `json:"action"`
	TargetItemID string            `json:"targetItemId"`
	Reason       string            `json:"reason"`
	Changes      ItemChangesResult `json:"changes"`
}

// ItemChangesResult is the desktop-facing DTO for a
// domainknowledge.ItemChanges — optional replacements for an existing
// Item's user-editable content fields. A nil Definition, or a nil list
// field, means "unchanged".
type ItemChangesResult struct {
	Definition      *string  `json:"definition"`
	Properties      []string `json:"properties"`
	TradeOffs       []string `json:"tradeOffs"`
	RelatedConcepts []string `json:"relatedConcepts"`
}

func toItemChangesResult(changes domainknowledge.ItemChanges) ItemChangesResult {
	return ItemChangesResult{
		Definition: changes.Definition, Properties: changes.Properties,
		TradeOffs: changes.TradeOffs, RelatedConcepts: changes.RelatedConcepts,
	}
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

// ExtractionResult carries candidates and the transcript truncation signal.
type ExtractionResult struct {
	BatchID   string                `json:"batchId"`
	Items     []KnowledgeItemResult `json:"items"`
	Truncated bool                  `json:"truncated"`
}

// KnowledgeSaveResult identifies persisted inputs even when a later save fails.
type KnowledgeSaveResult struct {
	SavedIndices []int  `json:"savedIndices"`
	Error        string `json:"error"`
}

// ExtractKnowledge extracts unpersisted knowledge candidates for review.
func (a *App) ExtractKnowledge(sessionID string, confirmedTruncation bool) (ExtractionResult, error) {
	batch, truncated, err := a.knowledge.ExtractFromSession(a.ctx, sessionID, confirmedTruncation)
	if errors.Is(err, applicationknowledge.ErrMalformedExtraction) {
		log.Printf("knowledge extraction returned malformed JSON: %v", err)
		return ExtractionResult{Items: []KnowledgeItemResult{}}, nil
	}
	if err != nil {
		return ExtractionResult{}, err
	}
	results := make([]KnowledgeItemResult, len(batch.Items))
	for index, candidate := range batch.Items {
		results[index] = toExtractionCandidateResult(candidate)
	}
	return ExtractionResult{BatchID: batch.ID, Items: results, Truncated: truncated}, nil
}

// toExtractionCandidateResult extends toKnowledgeItemResult with the
// candidate's duplicate-detection outcome.
func toExtractionCandidateResult(candidate applicationknowledge.ExtractionCandidate) KnowledgeItemResult {
	result := toKnowledgeItemResult(candidate.Item)
	result.Duplicates = make([]DuplicateMatchResult, len(candidate.Duplicates))
	for index, match := range candidate.Duplicates {
		result.Duplicates[index] = DuplicateMatchResult{
			ItemID: match.ItemID, Concept: match.Concept, Status: match.Status,
			MatchType: match.MatchType, Score: match.Score,
		}
	}
	result.SemanticCheckUnavailable = candidate.SemanticCheckUnavailable
	if candidate.Reconciliation != nil {
		result.Reconciliation = &ReconciliationSuggestionResult{
			Action: candidate.Reconciliation.Action, TargetItemID: candidate.Reconciliation.TargetItemID,
			Reason: candidate.Reconciliation.Reason, Changes: toItemChangesResult(candidate.Reconciliation.Changes),
		}
	}
	result.ReconciliationFailed = candidate.ReconciliationFailed
	return result
}

// toDomainItemFromInput builds the domainknowledge.Item content the
// reconciliation apply/resolve bindings pass to the application layer —
// the frontend is trusted for candidate content the same way
// SaveExtractedKnowledge already is, never for provenance (see
// batchID/candidateID, which are looked up against the backend's own
// receipt instead).
func toDomainItemFromInput(input KnowledgeItemInput) domainknowledge.Item {
	return domainknowledge.Item{
		ID: input.ID, Topic: input.Topic, Concept: input.Concept, Definition: input.Definition,
		Properties: input.Properties, TradeOffs: input.TradeOffs, RelatedConcepts: input.RelatedConcepts,
		Source: input.Source, Status: input.Status,
	}
}

// ApplyReconciliationCreate persists batchID/candidateID's classified
// candidate as a brand-new Knowledge Item at status (draft or approved).
func (a *App) ApplyReconciliationCreate(batchID, candidateID string, candidate KnowledgeItemInput, status string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.ApplyReconciliationCreate(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate), status)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying reconciliation create for candidate "+candidateID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ApplyReconciliationUpdate applies the classified changes to
// batchID/candidateID's target, preserving its identity and lifecycle.
func (a *App) ApplyReconciliationUpdate(batchID, candidateID string, candidate KnowledgeItemInput) (KnowledgeItemResult, error) {
	item, err := a.knowledge.ApplyReconciliationUpdate(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate))
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying reconciliation update for candidate "+candidateID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ApplyReconciliationRelate creates batchID/candidateID's candidate as a
// new draft Knowledge Item and links it to the classified target via a
// `related` relation.
func (a *App) ApplyReconciliationRelate(batchID, candidateID string, candidate KnowledgeItemInput) (KnowledgeItemResult, error) {
	item, err := a.knowledge.ApplyReconciliationRelate(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate))
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("applying reconciliation relate for candidate "+candidateID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// ResolveReconciliationConflict applies one of the three explicit conflict
// outcomes for batchID/candidateID: "keep_existing", "update_existing", or
// "create_separately".
func (a *App) ResolveReconciliationConflict(batchID, candidateID string, candidate KnowledgeItemInput, resolution string) (KnowledgeItemResult, error) {
	item, err := a.knowledge.ResolveReconciliationConflict(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate), resolution)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("resolving reconciliation conflict for candidate "+candidateID, err)
		return toKnowledgeItemResult(item), nil
	}
	if err != nil {
		return KnowledgeItemResult{}, err
	}
	return toKnowledgeItemResult(item), nil
}

// AcknowledgeReconciliationNoChange marks batchID/candidateID's classified
// no_change proposal resolved without creating or changing any Item.
func (a *App) AcknowledgeReconciliationNoChange(batchID, candidateID string, candidate KnowledgeItemInput) error {
	return a.knowledge.AcknowledgeReconciliationNoChange(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate))
}

// SaveReconciliationForReview persists batchID/candidateID's classified
// proposal as pending, changing neither the candidate nor its target.
func (a *App) SaveReconciliationForReview(batchID, candidateID string, candidate KnowledgeItemInput) error {
	return a.knowledge.SaveReconciliationForReview(a.ctx, batchID, candidateID, toDomainItemFromInput(candidate))
}

// SaveExtractedKnowledge persists only the candidates confirmed by the user.
// batchID identifies the backend extraction receipt (see ExtractKnowledge);
// each input's ID is used only as an opaque lookup key into that receipt —
// the frontend is never trusted for provenance.
func (a *App) SaveExtractedKnowledge(batchID string, inputs []KnowledgeItemInput) KnowledgeSaveResult {
	items := make([]domainknowledge.Item, len(inputs))
	for index, input := range inputs {
		items[index] = domainknowledge.Item{
			ID: input.ID, Topic: input.Topic, Concept: input.Concept, Definition: input.Definition,
			Properties: input.Properties, TradeOffs: input.TradeOffs, RelatedConcepts: input.RelatedConcepts,
			Source: input.Source, Status: input.Status,
		}
	}
	savedIndices, err := a.knowledge.SaveDrafts(a.ctx, batchID, items)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("saving drafts", err)
		return KnowledgeSaveResult{SavedIndices: savedIndices}
	}
	result := KnowledgeSaveResult{SavedIndices: savedIndices}
	if err != nil {
		result.Error = fmt.Sprintf("knowledge save failed: %v", err)
	}
	return result
}

// DiscardExtraction drops every unsaved candidate in batchID — called when
// the user dismisses an extraction batch. It must never be called after a
// partial save error, so unsaved or failed candidates stay retryable.
func (a *App) DiscardExtraction(batchID string) {
	a.knowledge.DiscardExtraction(batchID)
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

// SaveAndApproveExtractedKnowledge persists only the confirmed candidates,
// directly as approved — the "Save as knowledge" option from
// specs/Athena.md §12, skipping the draft review stage SaveExtractedKnowledge
// (SaveDrafts) leaves candidates in. batchID identifies the backend
// extraction receipt (see ExtractKnowledge); each input's ID is used only
// as an opaque lookup key into that receipt.
func (a *App) SaveAndApproveExtractedKnowledge(batchID string, inputs []KnowledgeItemInput) KnowledgeSaveResult {
	items := make([]domainknowledge.Item, len(inputs))
	for index, input := range inputs {
		items[index] = domainknowledge.Item{
			ID: input.ID, Topic: input.Topic, Concept: input.Concept, Definition: input.Definition,
			Properties: input.Properties, TradeOffs: input.TradeOffs, RelatedConcepts: input.RelatedConcepts,
			Source: input.Source, Status: input.Status,
		}
	}
	savedIndices, err := a.knowledge.SaveAndApprove(a.ctx, batchID, items)
	if errors.Is(err, applicationknowledge.ErrIndexingFailed) {
		logIndexingFailure("saving and approving drafts", err)
		return KnowledgeSaveResult{SavedIndices: savedIndices}
	}
	result := KnowledgeSaveResult{SavedIndices: savedIndices}
	if err != nil {
		result.Error = fmt.Sprintf("knowledge save failed: %v", err)
	}
	return result
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
