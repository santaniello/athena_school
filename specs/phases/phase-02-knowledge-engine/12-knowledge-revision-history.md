# Phase 2.12 — Knowledge Revision History

## Goal

Preserve an immutable, evidence-backed history of every Knowledge Item mutation so the user can see what changed, why, and what supported the change.

History is read-only in Phase 2. Restoring an old revision is deliberately out of scope because it is a new mutation with conflict and re-indexing semantics of its own.

## Revision model

```go
const (
    RevisionBaseline   = "baseline"
    RevisionCreate     = "create"
    RevisionEdit       = "edit"
    RevisionApprove    = "approve"
    RevisionDeprecate  = "deprecate"
    RevisionReconcile  = "reconcile"
)

const (
    RevisionByUser   = "user"
    RevisionByAthena = "athena"
)

type Revision struct {
    ID             string
    ItemID         string
    Number         int
    Snapshot       Item
    ChangeType     string
    ChangedBy      string
    Reason         string
    CreatedAt      time.Time
    EvidenceIDs    []string
}
```

The JSON snapshot includes every Knowledge Item field at that revision. It is encoded and decoded by the infrastructure adapter; domain/application code works with the typed snapshot. Revision rows are append-only: repository ports expose `Append` and `ListByItem`, never `Update` or `Delete`.

## Atomic mutation rule

Creation, inline edit, approval, deprecation, and applied reconciliation append
exactly one revision in the same SQLite transaction as the item mutation and evidence
links. A failed revision write rolls the mutation back. Vector re-indexing happens
after that transaction and follows 2.8's recoverable failure policy.

Bulk approval invokes the same single-item application path, so every approved item receives its own revision. Rejecting/deleting an item remains permanent and cascades its revisions; the delete confirmation explicitly says its history will also be removed.

Items present when this feature is introduced receive one `baseline` revision with their current snapshot, `ChangedByAthena`, and reason `"revision history enabled"`. The backfill is idempotent: an item with any revision is skipped.

## Schema

```sql
CREATE TABLE IF NOT EXISTS knowledge_item_revisions (
    id          TEXT PRIMARY KEY,
    item_id     TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    number      INTEGER NOT NULL,
    snapshot    TEXT NOT NULL,
    change_type TEXT NOT NULL,
    changed_by  TEXT NOT NULL,
    reason      TEXT NOT NULL,
    created_at  DATETIME NOT NULL,
    UNIQUE (item_id, number)
);

CREATE TABLE IF NOT EXISTS knowledge_revision_evidence (
    revision_id TEXT NOT NULL REFERENCES knowledge_item_revisions(id) ON DELETE CASCADE,
    evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
    PRIMARY KEY (revision_id, evidence_id)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_item_revisions_item_number
    ON knowledge_item_revisions(item_id, number);
```

## History UI

The Knowledge Item detail gains a **History** tab ordered newest-first. Each row shows revision number, change type, author, reason, timestamp, and evidence. Expanding a row shows a field-level diff against the preceding revision. Lists are compared as ordered values; the baseline/create revision displays the complete snapshot.

Diff generation is a pure frontend helper over two typed snapshots. The stored snapshot remains authoritative; no generated prose is saved as history.

## Tasks

- [ ] `internal/domain/knowledge/revision.go` — constants, `Revision`, append/list repository
- [ ] `internal/application/knowledge/revisions.go` — transactional append orchestration and idempotent baseline backfill
- [ ] Route create/edit/approve/deprecate/reconciliation through the revision-writing transaction
- [ ] `internal/infrastructure/sqlite/migrations.go` — revision, evidence-link tables, and index
- [ ] SQLite revision repository with typed JSON snapshot codec
- [ ] `internal/interfaces/desktop/knowledge.go` — `ListKnowledgeItemRevisions(id)`
- [ ] Knowledge Item detail — History tab and evidence links
- [ ] `frontend/src/lib/knowledge.ts` — pure field-level revision diff helper
- [ ] Delete confirmation — state that item history is removed too

## Acceptance Criteria

- Creating, editing, approving, deprecating, and reconciling each append exactly one correctly typed revision
- A revision failure rolls back the corresponding Knowledge Item mutation
- A failed post-commit embedding/index update does not roll back the item or its revision
- Two mutations receive sequential unique revision numbers even when timestamps are equal
- Bulk approval creates one approval revision per item and no redundant aggregate revision
- Baseline backfill creates one revision only for items with no history and is idempotent
- Revision snapshots round-trip nil/empty slices as non-nil empty lists
- History is returned newest-first and its diff identifies scalar and list changes
- Historical evidence remains visible after the originating message or chunk changes
- Deleting an item removes its revisions and unreferenced evidence, after explicit confirmation
- No API for modifying or restoring a revision exists in Phase 2
