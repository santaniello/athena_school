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

type Item struct {
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

`Source` is a category, not provenance. Spec 2.9 deliberately keeps concrete
supporting messages/chunks in evidence tables instead of overloading this field with
polymorphic identifiers.

The type is `knowledge.Item`, not `knowledge.KnowledgeItem`. `revive`'s stutter
check flags any exported name that repeats its own package name as a prefix
(`knowledge.KnowledgeItem` says "knowledge" twice; `folder.Folder` doesn't
trigger it because the name is *identical* to the package, the same idiom as
`time.Time`). This name is referenced by every later spec in this phase —
02, 06, 07, 08, 09, 10, 11, 12 — so read `KnowledgeItem` there as `Item`,
qualified as `knowledge.Item` outside the package (or `domainknowledge.Item`
in `internal/application/knowledge`, which collides on package name with
`internal/domain/knowledge` the same way `internal/application/folder`
already does with `internal/domain/folder` — see 2.2), unless the context is a
method/DTO name (`ApproveKnowledgeItem`, `KnowledgeItemResult`, ...), which
lives in a different package and doesn't stutter.

## Lifecycle

Only two transitions are legal. `draft → deprecated` is **not** allowed: the explorer offers Deprecate on approved items only, and the review queue rejects drafts by deletion.

```go
// TransitionTo returns a copy with the new status, or an error if the
// transition is not allowed. now is injected so the function stays pure.
func (i Item) TransitionTo(next string, now time.Time) (Item, error)

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
    Save(ctx context.Context, item Item) error
    GetByID(ctx context.Context, id string) (Item, error)
    FindByTopic(ctx context.Context, topic string) ([]Item, error)
    List(ctx context.Context, filter Filter) ([]Item, error)
    ListTopics(ctx context.Context) ([]string, error)
    CountByStatus(ctx context.Context, status string) (int, error)
    Update(ctx context.Context, item Item) error
    Delete(ctx context.Context, id string) error
}
```

Every query returning `Item` orders by `created_at ASC, id ASC` — 2.7 requires oldest-first, and the `id` tiebreak keeps tests deterministic when timestamps collide. `ListTopics` returns `[]string`, not items, so that tiebreak doesn't apply to it — it orders `ORDER BY topic ASC` instead, so callers (2.6's topic tree) get a deterministic, alphabetical list without sorting client-side.

Deliberate design choices:

- **No `UpdateStatus`.** A status-only write path would invite callers to set a status directly. Approve/Deprecate are always load → transition → `Update`, so the lifecycle rule has exactly one entry point in the application layer.

  `Update` still takes a whole `Item`, so it *can* physically persist a hand-set `Status` — the repository is a dumb persistence port and validating transitions there would put a business rule in infrastructure, against ADR-001. The guarantee is therefore "one orchestration path", not "physically impossible": `UpdateItem` (2.6) never touches `Status`, and any future write path must go through `TransitionTo` too.
- **`Save` returns only `error`**, not the ID. This deviates from an earlier draft of this spec but matches every existing repository (`FolderRepository.Create`, `MessageRepository.Append`); returning an ID the caller just supplied is noise. The use case returns the populated item. Note the deviation in the commit body and `CHANGELOG.md`.
- **Malformed JSON in a list column is a read failure, not an empty list.** `unmarshalStringList` returns `([]string, error)`; `GetByID`, `FindByTopic`, and `List` wrap and propagate that error instead of silently degrading to `[]string{}`. Treating corrupt data as "just empty" would hide a real integrity problem behind a value that looks like a legitimate answer.
- **NULL tolerance lives in the SQL, not in the decoder.** Every `SELECT` reads the three list columns through `COALESCE(column, '')`, so the value handed to `unmarshalStringList` is always a plain `string`, never `sql.NullString`. This keeps `unmarshalStringList`'s signature `string -> ([]string, error)`, trivial to unit-test without a database.

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

- [ ] `internal/domain/knowledge/item.go` — status/source constants, `Item`, `TransitionTo`, `allowedTransition`
- [ ] `internal/domain/knowledge/repository.go` — `Filter`, `Repository`, sentinels `ErrItemNotFound` / `ErrInvalidStatusTransition` / `ErrUnknownStatus`
- [ ] `internal/infrastructure/sqlite/jsonlist.go` — `marshalStringList` / `unmarshalStringList`: the JSON-array-as-TEXT encoding (nil/empty → `"[]"`, never NULL; decode tolerates `""`, treats invalid JSON as an error, never returns a nil slice on success)
- [ ] `internal/infrastructure/sqlite/jsonlist_test.go` — direct unit tests for `marshalStringList`/`unmarshalStringList`: nil/empty/three-element round-trip, `""` decodes to `[]string{}`, invalid JSON returns an error
- [ ] `internal/infrastructure/sqlite/migrations.go` — append the table and both indexes
- [ ] `internal/infrastructure/sqlite/knowledge_repository.go` — reuse `requireRowAffected(result, knowledge.ErrItemNotFound)` from `folder_repository.go`; `SELECT`s read the three list columns via `COALESCE(column, '')`; `List` builds its WHERE from accumulated conditions and args, never string-concatenated values (gosec G201); `ListTopics` uses `ORDER BY topic ASC`
- [ ] `make mock` to regenerate `internal/domain/knowledge/mocks/`

## Acceptance Criteria

- `Save` persists an item and `GetByID` round-trips every field, including a three-element `Properties` slice
- Nil and empty slices round-trip to `[]string{}`, never `nil`
- `FindByTopic` returns only items of that topic
- `List` honours each `Filter` combination and returns items oldest-first
- `ListTopics` returns distinct topics ordered alphabetically; `CountByStatus` returns the right count per status
- A malformed JSON value in `properties`, `trade_offs`, or `related_concepts` makes `GetByID` return an error instead of an empty slice — asserted both directly on `unmarshalStringList` and end-to-end through a repository test that writes invalid JSON via raw SQL first
- `Update` and `Delete` return `ErrItemNotFound` for an unknown ID
- `TransitionTo(StatusApproved)` succeeds from `draft`; `TransitionTo(StatusDeprecated)` succeeds from `approved`
- `TransitionTo(StatusDeprecated)` from `draft` returns `ErrInvalidStatusTransition` — the transition the spec calls out as disallowed is asserted, not merely described
- `TransitionTo(StatusApproved)` from `deprecated` returns `ErrInvalidStatusTransition`; an unknown status returns `ErrUnknownStatus`
- `TransitionTo` stamps `UpdatedAt` from the injected `now` and leaves the receiver unmodified
- `Open` creates `knowledge_items` and is idempotent on a second call
