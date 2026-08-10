# Phase 2.1 — Knowledge Item Model

## Goal

Define the core domain entity that represents a unit of knowledge extracted from sessions or imported notes.

## Domain

```go
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

## Schema

```sql
CREATE TABLE knowledge_items (
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
```

## Tasks

- [ ] `internal/domain/knowledge/` — `KnowledgeItem` struct + lifecycle rules
- [ ] Repository interface: `KnowledgeRepository` (defined in domain)
- [ ] SQLite implementation: `internal/infrastructure/sqlite/knowledge_repository.go`
- [ ] Allowed status transitions: `draft → approved`, `approved → deprecated`

## Acceptance Criteria

- `KnowledgeRepository.Save` persists an item and returns its ID
- `KnowledgeRepository.FindByTopic` returns all items for a given topic
- Status transition `draft → approved` succeeds; `deprecated → approved` returns an error
