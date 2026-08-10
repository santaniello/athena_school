# Phase 7.1 — Knowledge Graph

## Goal

Visual graph showing relationships between concepts in the Knowledge Base.

## Schema

```sql
CREATE TABLE concept_relations (
    from_concept TEXT,
    to_concept   TEXT,
    relation_type TEXT, -- related | prerequisite | extends
    weight       REAL
);
```

## Tasks

- [ ] `internal/domain/knowledge/graph.go` — relation model and graph traversal
- [ ] Relations extracted by LLM from approved Knowledge Items
- [ ] UI: interactive graph visualization (React Flow or D3)
- [ ] Clicking a node opens the Knowledge Item detail view
- [ ] Relation types shown as edge labels with different colors
- [ ] Graph filtered by topic

## Acceptance Criteria

- Approved Knowledge Items with `related_concepts` appear as connected nodes
- Clicking a node navigates to its Knowledge Item detail
- Graph renders without crashing for up to 200 nodes
- Filtering by topic shows only nodes in that topic and their inter-connections
