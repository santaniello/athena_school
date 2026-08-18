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

- [ ] `internal/domain/knowledge/reconciliation.go` — proposal, actions/statuses, changes, validation, repositories
- [ ] `internal/domain/knowledge/relation.go` — ID-based relation model and canonical symmetric relation rule
- [ ] `internal/infrastructure/sqlite/migrations.go` — proposal, evidence-link, and relation tables
- [ ] SQLite reconciliation/relation repositories
- [ ] `internal/application/knowledge/reconcile.go` — classifier prompt, parser, save/reject/apply and stale checks
- [ ] `internal/interfaces/desktop/knowledge.go` — reconcile, list pending, apply, reject bindings
- [ ] Extraction dialog — action/diff preview and immediate decisions
- [ ] Knowledge Review — pending reconciliation rows, conflict choices, stale-state recovery

## Acceptance Criteria

- A candidate without duplicate matches creates a `create` proposal without another LLM call
- The LLM cannot target an item outside the supplied shortlist or change server-owned fields
- Applying create ignores every candidate/client ID and generates a new item ID server-side
- Saving for review changes neither the candidate nor the target Knowledge Item
- Saving or applying a proposal regenerates evidence IDs from validated source-message references
- Applying update changes only the reviewed fields and retains ID, topic, source, status, and creation time
- Applying relate creates one canonical relation and repeated application is idempotent
- Conflict cannot resolve until the user chooses one of the three explicit outcomes
- No-change creates no Knowledge Item and preserves a resolved audit record
- A target edited or removed after proposal creation makes the proposal stale and prevents application
- Every created or updated item retains the proposal's evidence
- Bulk draft approval does not silently apply reconciliation proposals
