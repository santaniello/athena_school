# Phase 2.6 — Knowledge Explorer (UI)

## Goal

User can browse, filter, and manage all Knowledge Items from a dedicated screen.

## Layout

The `knowledge` nav section already exists in `frontend/src/lib/navigation.ts` as `{phase: 2, status: 'locked'}` — this spec unlocks it.

```text
sidebar rail                     main pane
─────────────────────            ────────────────────────────────────────
  Knowledge  ⟨3⟩                   [ Explorer | Review ⟨3⟩ ]  [Import notes]
    ├ All topics                   Status: ⟨All ▾⟩
    ├ Go                           ┌───────────────┬──────────────────────┐
    ├ Kubernetes                   │ item list     │ detail + actions     │
    └ System Design                └───────────────┴──────────────────────┘
```

- The topic tree renders inside the sidebar rail under the Knowledge item, exactly where `StudyFolderTree` renders under Study in `app-shell.tsx`.
- The main pane splits with plain flex (`w-80 shrink-0 border-r` list column + `flex-1` detail column). **Do not nest a second `ResizablePanelGroup`** inside the existing one.
- Explorer/Review is a local `'explorer' | 'review'` state machine, consistent with `App.tsx` and `app-shell.tsx` — the project deliberately has no router.

## Actions

Actions are gated by the domain lifecycle, so the UI never offers an illegal transition:

| Status | Available actions |
|---|---|
| `draft` | Approve · Edit · Delete |
| `approved` | Deprecate · Edit · Delete |
| `deprecated` | Edit · Delete |

Delete is irreversible and sits behind an `AlertDialog`, mirroring the folder-delete flow in `study-folder-tree.tsx`.

## Tasks

- [ ] `internal/application/knowledge/list.go` — `ListItems(ctx, topic, status)`, `ListTopics(ctx)`
- [ ] `internal/application/knowledge/approve.go` — `Approve(ctx, id)`: load → `TransitionTo` → `Update`. **This is the seam 2.8's indexing hook plugs into**
- [ ] `internal/application/knowledge/deprecate.go` — `Deprecate(ctx, id)`
- [ ] `internal/application/knowledge/update.go` — `UpdateItem(ctx, id, fields)`: validates, restamps `UpdatedAt`, never touches `Status` / `Source` / `CreatedAt`
- [ ] `internal/application/knowledge/delete.go` — `DeleteItem(ctx, id)`
- [ ] `internal/interfaces/desktop/knowledge.go` — `ListKnowledgeItems`, `ListKnowledgeTopics`, `ApproveKnowledgeItem`, `DeprecateKnowledgeItem`, `UpdateKnowledgeItem`, `DeleteKnowledgeItem`; the three mutating ones return the updated item so React can patch local state without a refetch (precedent: `UpdateProfile` in `settings.go`)
- [ ] `frontend/src/lib/navigation.ts` — flip `knowledge` to `status: 'unlocked'` (updates one assertion in `navigation.test.ts`)
- [ ] `frontend/src/lib/knowledge.ts` — add the new wrappers plus pure helpers `groupByTopic(items)` and `definitionPreview(text, max)`
- [ ] `frontend/src/components/knowledge-section.tsx` — Explorer/Review tab state
- [ ] `frontend/src/components/knowledge-topic-tree.tsx` — topics from `ListKnowledgeTopics` plus an "All topics" row
- [ ] `frontend/src/screens/KnowledgeExplorerScreen.tsx` — status filter (`Select`), item list (concept + definition preview + status `Badge`), detail view with all fields and the gated actions
- [ ] Inline editor reusing `frontend/src/components/tag-input.tsx` for `Properties` / `TradeOffs` / `RelatedConcepts`, `Input` for concept, `Textarea` for definition
- [ ] `frontend/src/components/knowledge-extraction-dialog.tsx` — add the third button **[Save & approve]**, completing the flow of `specs/Athena.md` §12

> Pushing logic (`groupByTopic`, `definitionPreview`, filter predicates) into `lib/knowledge.ts` as pure functions is what keeps the frontend 80% coverage and Stryker 80 thresholds reachable on two large screens — far better than letting Stryker chew on deep JSX branches.

## Acceptance Criteria

- All Knowledge Items are visible in the explorer after a study session with extraction
- Filtering by "draft" shows only unreviewed items; selecting a topic in the tree restricts the list to that topic
- Clicking "Approve" changes the badge from "draft" to "approved" without a page reload
- "Approve" is not offered on an approved or deprecated item; "Deprecate" is offered only on approved items
- Editing a field and saving persists the change to SQLite and leaves `Status`, `Source`, and `CreatedAt` untouched
- Delete asks for confirmation and removes the item permanently
- The Knowledge nav section is unlocked and no longer renders `ComingSoonPanel`
