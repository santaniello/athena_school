# Phase 2.11 — Knowledge Reconciliation

## Goal

Compare newly extracted knowledge with existing matches and let the user decide whether it creates, updates, relates, conflicts with, or adds nothing to the Knowledge Base.

The LLM proposes. The application validates. The user decides. No proposal may mutate an existing Knowledge Item automatically.

## Proposal model

```go
const (
    ReconcileCreate   = "create"
    ReconcileUpdate   = "update"
    ReconcileRelate   = "relate"
    ReconcileConflict = "conflict"
    ReconcileNoChange = "no_change"
)

const (
    ProposalPending  = "pending"
    ProposalApplied  = "applied"
    ProposalRejected = "rejected"
    ProposalStale    = "stale"
)

type ReconciliationProposal struct {
    ID              string
    Action          string
    Status          string
    Candidate       Item
    TargetItemID    string
    TargetUpdatedAt time.Time
    Reason          string
    Changes         ItemChanges
    EvidenceIDs     []string
    CreatedAt       time.Time
}
```

`ItemChanges` contains optional replacements for `Definition`, `Properties`, `TradeOffs`, and `RelatedConcepts`. It never contains server-owned ID, topic, source, status, or timestamps.

## Classification flow

```text
Validated extraction candidate
          ↓
2.10 duplicate shortlist empty? ── yes → deterministic create proposal
          ↓ no                              (no second LLM call)
LLM compares candidate + evidence with shortlist only
          ↓
Go validates action, target, bounded diff, and evidence ownership
          ↓
User applies, saves for review, or rejects
```

The comparison prompt includes only the candidate, its bounded evidence, and at most `DefaultDuplicateTopK` matches. It cannot nominate an item outside that shortlist. Malformed output leaves the original extraction candidate available as a manual create decision and reports the reconciliation failure; it never applies a guessed fallback to an existing item.

## Persistence and concurrency

Choosing **Save for review** stores the proposal and its evidence links. Pending reconciliation proposals appear alongside drafts in Knowledge Review, but have their own action badge and do not count as draft Knowledge Items.

At extraction time the candidate still carries validated source-message quotes, not
trusted evidence-row IDs. Saving or immediately applying a proposal reloads those
messages, repeats the verbatim checks, and atomically materializes
`knowledge_evidence` snapshots plus the proposal or item links. Client-supplied
evidence IDs are never accepted.

When a proposal targets an existing item, `TargetUpdatedAt` is an optimistic concurrency token. `ApplyProposal` reloads the target and compares timestamps. A missing or changed target marks the proposal `stale`; the user must run reconciliation again against current knowledge.

```sql
CREATE TABLE IF NOT EXISTS knowledge_reconciliation_proposals (
    id                TEXT PRIMARY KEY,
    action            TEXT NOT NULL,
    status            TEXT NOT NULL,
    candidate_snapshot TEXT NOT NULL, -- validated JSON Item snapshot
    target_item_id    TEXT,
    target_updated_at DATETIME,
    reason            TEXT NOT NULL,
    changes           TEXT NOT NULL, -- validated JSON ItemChanges
    created_at        DATETIME NOT NULL,
    resolved_at       DATETIME
);

CREATE TABLE IF NOT EXISTS knowledge_reconciliation_evidence (
    proposal_id TEXT NOT NULL REFERENCES knowledge_reconciliation_proposals(id) ON DELETE CASCADE,
    evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
    PRIMARY KEY (proposal_id, evidence_id)
);

CREATE TABLE IF NOT EXISTS knowledge_item_relations (
    from_item_id TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    to_item_id   TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    relation_type TEXT NOT NULL,
    created_at    DATETIME NOT NULL,
    PRIMARY KEY (from_item_id, to_item_id, relation_type),
    CHECK (from_item_id <> to_item_id)
);
```

`related` is symmetric: persist one canonical row with the lexicographically smaller ID in `from_item_id`. Phase 7 may add directional `prerequisite` and `extends` relations without replacing this table.

## Applying each action

- **create** — regenerate the item ID server-side, create a new draft or approved item according to the user's explicit button, and attach the proposal evidence
- **update** — apply only the reviewed `ItemChanges` to the target, retain its identity/lifecycle, and attach the new evidence
- **relate** — create the candidate separately and add a `related` relation to the target; never merge definitions implicitly
- **conflict** — remain pending until the user chooses **Keep existing**, **Update existing**, or **Create separately**; the choice is stored as the resolution reason
- **no_change** — mark applied without changing or creating an item; preserve the proposal as the audit record

An exact match against a deprecated target may create a new successor linked by `related`; the deprecated item never transitions back to approved.

## Tasks

Increment 1 — reconciliation engine + extraction-dialog decisions (complete):

- [x] `internal/domain/knowledge/reconciliation.go` — proposal, actions/statuses, changes, validation, repositories
- [x] `internal/domain/knowledge/relation.go` — ID-based relation model and canonical symmetric relation rule
- [x] `internal/infrastructure/sqlite/migrations.go` — proposal, evidence-link, and relation tables
- [x] SQLite reconciliation/relation repositories
- [x] `internal/application/knowledge/reconcile.go` — classifier prompt, parser, apply/resolve/acknowledge/save-for-review and stale checks
- [x] `internal/interfaces/desktop/knowledge.go` — apply/resolve/acknowledge/save-for-review bindings
- [x] Extraction dialog — action preview and immediate decisions

Increment 2a — Knowledge Review, pending proposals (complete):

- [x] `ReconciliationRepository` gains `GetByID`, `ListByStatus`, `UpdateStatus`, `CountByStatus`
- [x] `internal/application/knowledge/reconcile_pending.go` — list (with eager per-proposal staleness check), apply/resolve/acknowledge/reject against a persisted proposal, never a transient receipt
- [x] `internal/interfaces/desktop/reconcile_pending.go` — list/count/apply/resolve/acknowledge/reject bindings
- [x] Knowledge Review — pending reconciliation rows (own section, own badge, never counted as a draft), conflict choices, reject
- [x] `reviewCount = draftCount + pendingProposalCount` combined sidebar badge (see [07-knowledge-review.md](07-knowledge-review.md)) — the Review tab's own badge stays draft-only
- [x] `SaveReconciliationForReview`'s "Save for review" affordance in the extraction dialog — unhidden now that Knowledge Review can show what it saves

Increment 2b — stale-state recovery (not started):

- [ ] Reclassifying a stale pending proposal against its target's current content (a new LLM call, not a data read) — 2a can only reject a stale proposal, never repair it

## Acceptance Criteria

Increment 1:

- [x] A candidate without duplicate matches creates a `create` proposal without another LLM call
- [x] The LLM cannot target an item outside the supplied shortlist or change server-owned fields
- [x] Applying create ignores every candidate/client ID and generates a new item ID server-side
- [x] Saving for review changes neither the candidate nor the target Knowledge Item
- [x] Saving or applying a proposal regenerates evidence IDs from validated source-message references
- [x] Applying update changes only the reviewed fields and retains ID, topic, source, status, and creation time
- [x] Applying relate creates one canonical relation and repeated application is idempotent
- [x] Conflict cannot resolve until the user chooses one of the three explicit outcomes
- [x] No-change creates no Knowledge Item and preserves a resolved audit record
- [x] A target edited or removed after proposal creation makes the proposal stale and prevents application
- [x] Every created or updated item retains the proposal's evidence
- [x] Bulk draft approval does not silently apply reconciliation proposals — the extraction dialog's batch buttons only ever reach `create`/`no_change` candidates; `update`/`relate`/`conflict` candidates are never selectable there

Increment 2a:

- [x] Pending reconciliation proposals appear in Knowledge Review, separate from drafts, with their own action badge, and never count as a draft Knowledge Item
- [x] Opening Knowledge Review checks every pending proposal's target eagerly; a stale one shows as stale without blocking the rest of the list from loading
- [x] Applying a pending proposal never re-verifies its evidence against the original study session — it uses exactly the candidate snapshot and evidence already persisted at "Save for review" time
- [x] Applying a stale proposal is refused
- [x] Rejecting a pending proposal transitions it to `rejected` without creating or changing any item, and never checks target freshness first
- [x] The sidebar badge sums `draftCount + pendingProposalCount`; the Review tab's own badge stays draft-only
- [x] The extraction dialog's decision-zone rows offer "Save for review" alongside their immediate action

Increment 2b (not yet applicable): reclassifying/repairing a stale pending proposal.

## Implementation handoff — 2026-08-28 (Increment 1)

Designed via `/grilling-design`: the spec was decomposed into two increments before implementation began, mirroring how [09-persistent-provenance.md](09-persistent-provenance.md) split its own scope. Increment 1 (this handoff) is complete; Increment 2 (Knowledge Review) has not been designed or authorized yet.

### Approved design decisions

- The extraction dialog splits into two zones: candidates classified `update`/`relate`/`conflict` render in a "Needs your decision" list, each with its own buttons that apply immediately on click; candidates classified `create`/`no_change` stay in the original checkbox + batch-button flow. The split exists because `update`/`relate`/`conflict` all touch or link an existing Item, and a user must not be able to approve that in a five-candidate batch click without ever looking at what changes.
- A decision is applied immediately, per row — never staged for a later confirmation. Dismissing the dialog only discards whatever was never decided; anything already applied stays applied, mirroring the batch zone's existing `DiscardExtraction` semantics.
- `SaveReconciliationForReview` is implemented and tested on the backend (status `pending`, no item mutation, no staleness check — a pending proposal is allowed to go stale while it waits) but its frontend affordance is deliberately withheld from this increment: exposing a "Save for review" button with nowhere to review it would be worse than not offering it.
- A conflict's three resolutions never change the persisted proposal `Action` away from `conflict` — even "Keep existing" and "Create separately" keep the audit trail showing the LLM detected a conflict; the chosen resolution is appended to `Reason` instead of encoded as a different `Status`.
- Every classified action carries its own `ReconciliationProposal` audit row once decided, whether applied immediately or (in Increment 2) saved for later — there is no path that mutates a Knowledge Item without a row explaining why.
- `relate` and conflict's `create_separately` always create the new Item as a `draft`, never letting an uncertain match auto-approve.
- Update's diff preview in the dialog shows only the proposed new field values, not a true old-vs-new diff: the backend's `ReconciliationSuggestion` deliberately does not duplicate the target's full prior content (only `TargetItemID`, cross-referenced against the candidate's own `duplicates` for concept/status) — showing an old→new diff would need a new backend surface not covered by this increment's grilling.

### Implemented (complete)

- `internal/domain/knowledge/reconciliation.go` — `ReconciliationProposal`, `ItemChanges`, action/status constants, `Validate`, `ReconciliationRepository`
- `internal/domain/knowledge/relation.go` — `Relation`, `CanonicalRelation` (lexicographically-smaller-ID ordering), `RelationRepository`
- `internal/infrastructure/sqlite/migrations.go` — `knowledge_reconciliation_proposals`, `knowledge_reconciliation_evidence`, `knowledge_item_relations`
- `internal/infrastructure/sqlite/reconciliation_repository.go`, `relation_repository.go`
- `internal/application/knowledge/reconcile.go` — `classifyCandidates`/`classifyCandidate` (wired into `ExtractFromSession` right after 2.10's duplicate shortlist), `buildReconciliationPrompt`/`parseReconciliation`, `ApplyReconciliationCreate`/`ApplyReconciliationUpdate`/`ApplyReconciliationRelate`, `ResolveReconciliationConflict`, `AcknowledgeReconciliationNoChange`, `SaveReconciliationForReview`
- `internal/domain/llm/router.go` — `TaskKnowledgeReconciliation` (routed at `TierCheap`, alongside `TaskKnowledgeExtraction`)
- `internal/interfaces/desktop/knowledge.go` — bindings for every application-layer entry point above, plus `ReconciliationSuggestionResult`/`ItemChangesResult` DTOs on `KnowledgeItemResult`
- `frontend/src/lib/knowledge.ts`, `frontend/src/components/knowledge-extraction-dialog.tsx` — the two-zone dialog described above
- Quality gates: `go test ./...`, `golangci-lint run` (0 issues), `govulncheck ./...` (0 vulnerabilities in this code), combined Go coverage 88.7%, `gremlins unleash` against `internal/domain` and `internal/application` with every mutant in changed code killed except one accepted equivalent mutant (`CanonicalRelation`'s swap-on-equal boundary — behaviorally identical to the original for every input, since swapping two equal values is a no-op either way); `npx tsc --noEmit`, `npx eslint .`, `npx prettier --check`, and `npx vitest run` (552 passed) on the frontend

### Still out of scope for this increment

- Knowledge Review's pending-proposal rows, conflict-choice resumption, and stale-state recovery (Increment 2)
- The combined `reviewCount` badge
- Exposing `SaveReconciliationForReview` in the extraction dialog
- A true old-vs-new diff for `update`/`conflict` previews (would need the target's full content on the DTO, not just its id)
- Directional `prerequisite`/`extends` relations (Phase 7)

## Implementation handoff — 2026-08-30 (Increment 2a)

Designed via `/grilling-design`, continuing the same session: Increment 2 was further split into 2a (pending-proposal listing and resolution — this handoff) and 2b (stale-state recovery, a reclassification flow, not started).

### Approved design decisions

- Knowledge Review checks every pending proposal's target **eagerly**, when the list loads — not lazily, on click. Reconsulting a target is a cheap local SQLite read here (no network), and showing a proposal as actionable when it is not would let the user click into a surprise failure; a per-proposal check that fails (target deleted) marks only that row stale without failing the whole list, mirroring how `retrieval.go` already drops an orphaned chunk rather than failing a whole search.
- Applying a pending proposal reads exclusively from what was already persisted at "Save for review" time — the `candidate_snapshot` and the evidence already linked via `knowledge_reconciliation_evidence` — and never reloads or re-verifies against the original study session. The session may no longer exist days later, and the evidence was already verified once; re-checking it again would only add a new, unrelated way to fail, contradicting the immutable-evidence-snapshot principle 2.9 already established.
- Rejecting a pending proposal never checks target freshness — rejecting applies nothing, so a target that changed since classification has no bearing on whether the reject itself is safe.
- A conflict's resolution note is appended to `Reason` the same way Increment 1 does it — `ReconciliationRepository.UpdateStatus` was extended to take and overwrite `Reason` alongside `Status`/`resolved_at` so this stays true for a proposal resolved from Knowledge Review, not just an immediate decision.
- The Review screen's pending rows reuse the exact same `ReconciliationDecisionRow` component the extraction dialog's decision zone already uses (extracted into `frontend/src/components/reconciliation-decision-row.tsx`), so both surfaces present the identical action vocabulary and visual treatment; only Knowledge Review passes `onReject`, and only the extraction dialog passes `onSaveForReview`.
- The combined badge lives only on the sidebar nav item; the Review tab's own badge (inside `KnowledgeSection`) stays `draftCount`-only, matching [07-knowledge-review.md](07-knowledge-review.md)'s original design.

### Implemented (complete)

- `internal/domain/knowledge/reconciliation.go` — `ReconciliationRepository` gains `GetByID`, `ListByStatus`, `UpdateStatus` (now reason-aware), `CountByStatus`; `ErrProposalNotFound`
- `internal/infrastructure/sqlite/reconciliation_repository.go` — the four methods above, decoding `candidate_snapshot`/`changes` and joining `knowledge_reconciliation_evidence` for `GetByID`
- `internal/application/knowledge/reconcile.go` — `checkReconciliationTargetFresh` generalized to take `(targetItemID, targetUpdatedAt)` directly so both the receipt-based (Increment 1) and persisted-proposal (2a) paths share one staleness check; `reasonWithResolution` extracted and reused by both
- `internal/application/knowledge/reconcile_pending.go` — `PendingReconciliation`, `ListPendingReconciliations` (eager per-row staleness), `CountPendingReconciliations`, `ApplyPendingReconciliationCreate/Update/Relate`, `ResolvePendingReconciliationConflict`, `AcknowledgePendingReconciliationNoChange`, `RejectPendingReconciliationProposal`
- `internal/interfaces/desktop/reconcile_pending.go` — bindings for every entry point above, plus `PendingReconciliationResult`
- `frontend/src/components/reconciliation-decision-row.tsx` — the shared row component (extracted from the extraction dialog), now with `stale`/`onReject`/`onSaveForReview`
- `frontend/src/components/pending-reconciliation-section.tsx` — the Review tab's pending-proposal list, mounted inside `KnowledgeSection` above the existing draft list
- `frontend/src/components/app-shell.tsx` — `pendingProposalCount` alongside `draftCount`; `refreshReviewCounts` refetches both together; the sidebar badge sums them
- `frontend/src/components/knowledge-extraction-dialog.tsx` — the "Save for review" button un-hidden on decision-zone rows
- Quality gates: `go test ./...`, `golangci-lint run` (0 issues), `govulncheck ./...` (0 vulnerabilities in this code), `gremlins unleash` against `internal/domain` and `internal/application` with every mutant in changed code killed (one boolean-switch coverage-granularity false negative in `reconcile_pending.go` resolved by restructuring to nested `if`, not by weakening a test); `npx tsc --noEmit`, `npx eslint .`, `npx prettier --check`, and `npx vitest run` (561 passed) on the frontend

### Still out of scope for this increment

- Reclassifying/repairing a stale pending proposal (Increment 2b)
- A true old-vs-new diff for `update`/`conflict` previews
- Directional `prerequisite`/`extends` relations (Phase 7)
