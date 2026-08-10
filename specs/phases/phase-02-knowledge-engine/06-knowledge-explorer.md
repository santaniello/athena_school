# Phase 2.6 — Knowledge Explorer (UI)

## Goal

User can browse, filter, and manage all Knowledge Items from a dedicated screen.

## Tasks

- [ ] Left sidebar: topic tree organized by `topic` field
- [ ] Filter bar: status filter (draft / approved / deprecated)
- [ ] Item list: shows concept name + definition preview per item
- [ ] Detail view: full `KnowledgeItem` fields (properties, trade-offs, related concepts)
- [ ] Actions on detail view:
  - Approve (draft → approved)
  - Edit (opens inline editor)
  - Deprecate (approved → deprecated)
  - Delete (irreversible; confirmation required)

## Acceptance Criteria

- All Knowledge Items are visible in the explorer after a study session with extraction
- Filtering by "draft" shows only unreviewed items
- Clicking "Approve" changes the badge from "draft" to "approved" without page reload
- Editing a field and saving persists the change to SQLite
