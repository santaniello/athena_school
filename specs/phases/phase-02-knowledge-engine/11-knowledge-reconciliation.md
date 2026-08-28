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

Increment 2 — Knowledge Review (not started):

- [ ] Knowledge Review — pending reconciliation rows, conflict choices, stale-state recovery
- [ ] `reviewCount = draftCount + pendingProposalCount` combined badge (see [07-knowledge-review.md](07-knowledge-review.md))
- [ ] `SaveReconciliationForReview`'s "Save for review" affordance in the extraction dialog — the backend call exists (Increment 1) but stays unexposed in the UI until there is somewhere to act on what it saves

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

Increment 2 (not yet applicable): pending-proposal listing, conflict choices resumed from Knowledge Review, and stale-state recovery outside the extraction dialog's own session.

## Implementation handoff — 2026-08-28

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
