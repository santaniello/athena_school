# Phase 2.1 — Knowledge Item Model

## Goal

Define the core domain entity that represents a unit of knowledge extracted from sessions or imported notes, plus its persistence.

## Domain

```go
const (
    SourceAthena      = "athena"
    SourceUserNote    = "user_note"
    SourceImportedDoc = "imported_doc"
)

const (
    StatusDraft      = "draft"
    StatusApproved   = "approved"
    StatusDeprecated = "deprecated"
)

type KnowledgeItem struct {
    ID              string
    Topic           string
    Concept         string
    Definition      string
    Properties      []string
    TradeOffs       []string
    RelatedConcepts []string
    Source          string    // athena | user_note | imported_doc
    Status          string    // draft | approved | deprecated
    CreatedAt       time.Time
    UpdatedAt       time.Time
}
```

## Lifecycle

Only two transitions are legal. `draft → deprecated` is **not** allowed: the explorer offers Deprecate on approved items only, and the review queue rejects drafts by deletion.

```go
// TransitionTo returns a copy with the new status, or an error if the
// transition is not allowed. now is injected so the function stays pure.
func (i KnowledgeItem) TransitionTo(next string, now time.Time) (KnowledgeItem, error)

func allowedTransition(from, to string) bool {
    return (from == StatusDraft && to == StatusApproved) ||
        (from == StatusApproved && to == StatusDeprecated)
}
```

Guard order inside `TransitionTo` (each guard is one test):

1. `next` is not one of the three status constants → `ErrUnknownStatus`
2. `!allowedTransition(i.Status, next)` → `ErrInvalidStatusTransition`
3. otherwise set `Status` and `UpdatedAt`, return the copy

> Written with `==` / `&&` / `||` rather than a `switch` on purpose: Gremlins mutates binary comparisons and logical operators, and `switch` case clauses give it nothing to bite.

`Validate()` is **not** part of this spec — its only consumer is 2.2. Adding it here would violate the "do not anticipate future specs" rule in `AGENTS.md`.

## Repository port

```go
type Filter struct{ Topic, Status string } // empty field = no constraint

type Repository interface {
    Save(ctx context.Context, item KnowledgeItem) error
    GetByID(ctx context.Context, id string) (KnowledgeItem, error)
    FindByTopic(ctx context.Context, topic string) ([]KnowledgeItem, error)
    List(ctx context.Context, filter Filter) ([]KnowledgeItem, error)
    ListTopics(ctx context.Context) ([]string, error)
    CountByStatus(ctx context.Context, status string) (int, error)
    Update(ctx context.Context, item KnowledgeItem) error
    Delete(ctx context.Context, id string) error
}
```

Every query orders by `created_at ASC, id ASC` — 2.7 requires oldest-first, and the `id` tiebreak keeps tests deterministic when timestamps collide.

Deliberate design choices:

- **No `UpdateStatus`.** A status-only write path would let callers bypass `TransitionTo`. Approve/Deprecate are always load → transition → `Update`, which makes the lifecycle rule structurally non-bypassable.
- **`Save` returns only `error`**, not the ID. This deviates from an earlier draft of this spec but matches every existing repository (`FolderRepository.Create`, `MessageRepository.Append`); returning an ID the caller just supplied is noise. The use case returns the populated item. Note the deviation in the commit body and `CHANGELOG.md`.

## Schema

```sql
CREATE TABLE IF NOT EXISTS knowledge_items (
    id               TEXT PRIMARY KEY,
    topic            TEXT,
    concept          TEXT,
    definition       TEXT,
    properties       TEXT, -- JSON array
    trade_offs       TEXT, -- JSON array
    related_concepts TEXT, -- JSON array
    source           TEXT,
    status           TEXT DEFAULT 'draft',
    created_at       DATETIME,
    updated_at       DATETIME
);

-- review queue + badge (status filter and ordering)
CREATE INDEX IF NOT EXISTS idx_knowledge_items_status_created_at
    ON knowledge_items(status, created_at);
-- explorer topic tree and topic filter
CREATE INDEX IF NOT EXISTS idx_knowledge_items_topic
    ON knowledge_items(topic);
```

All three statements are idempotent, so they are plain `execSQL(...)` entries appended to the `migrations` slice — no `PRAGMA`-guarded function is needed.

## Tasks

- [ ] `internal/domain/knowledge/item.go` — status/source constants, `KnowledgeItem`, `TransitionTo`, `allowedTransition`
- [ ] `internal/domain/knowledge/repository.go` — `Filter`, `Repository`, sentinels `ErrItemNotFound` / `ErrInvalidStatusTransition` / `ErrUnknownStatus`
- [ ] `internal/infrastructure/sqlite/jsonlist.go` — `marshalStringList` / `unmarshalStringList`: the JSON-array-as-TEXT encoding (nil/empty → `"[]"`, never NULL; decode tolerates NULL and `""`; never returns nil)
- [ ] `internal/infrastructure/sqlite/migrations.go` — append the table and both indexes
- [ ] `internal/infrastructure/sqlite/knowledge_repository.go` — reuse `requireRowAffected(result, knowledge.ErrItemNotFound)` from `folder_repository.go`; `List` builds its WHERE from accumulated conditions and args, never string-concatenated values (gosec G201)
- [ ] `make mock` to regenerate `internal/domain/knowledge/mocks/`

## Acceptance Criteria

- `Save` persists an item and `GetByID` round-trips every field, including a three-element `Properties` slice
- Nil and empty slices round-trip to `[]string{}`, never `nil`
- `FindByTopic` returns only items of that topic
- `List` honours each `Filter` combination and returns items oldest-first
- `ListTopics` returns distinct topics; `CountByStatus` returns the right count per status
- `Update` and `Delete` return `ErrItemNotFound` for an unknown ID
- `TransitionTo(StatusApproved)` succeeds from `draft`; from `deprecated` it returns `ErrInvalidStatusTransition`; an unknown status returns `ErrUnknownStatus`
- `TransitionTo` stamps `UpdatedAt` from the injected `now` and leaves the receiver unmodified
- `Open` creates `knowledge_items` and is idempotent on a second call
