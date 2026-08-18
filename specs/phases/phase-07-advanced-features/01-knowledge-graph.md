# Phase 7.1 — Knowledge Graph

## Goal

Visual graph showing relationships between concepts in the Knowledge Base.

## Persistence

Phase 2.11 already introduces ID-backed relations for reconciliation. Reuse and
extend `knowledge_item_relations`; no new relation table or schema migration is
needed. `related` remains a canonical symmetric edge. `prerequisite` and `extends`
use `from_item_id → to_item_id` direction. Concept names are display data, never
relationship keys.

## Tasks

- [ ] `internal/domain/knowledge/graph.go` — relation model and graph traversal
- [ ] Directional relations proposed by the LLM from approved Knowledge Items and persisted only after user confirmation
- [ ] UI: interactive graph visualization (React Flow or D3)
- [ ] Clicking a node opens the Knowledge Item detail view
- [ ] Relation types shown as edge labels with different colors
- [ ] Graph filtered by topic

## Acceptance Criteria

- Approved Knowledge Items with ID-backed relations appear as connected nodes
- Clicking a node navigates to its Knowledge Item detail
- Graph renders without crashing for up to 200 nodes
- Filtering by topic shows only nodes in that topic and their inter-connections
