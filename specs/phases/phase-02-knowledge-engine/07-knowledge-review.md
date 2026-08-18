# Phase 2.7 — Knowledge Review

## Goal

Drafts accumulated from extractions surface in a dedicated review queue so the user can approve or reject them efficiently.

## Bulk actions

The two bulk actions are **not** symmetric, because rejection is not a status change:

- `ApproveAllDrafts` iterates per item **through `TransitionTo`**, not a single bulk `UPDATE` — approval must take the same path as the single-item action, and N is small on a local database with one connection
- `RejectAllDrafts` iterates `Delete` per item. There is no `draft → deleted` transition, so `TransitionTo` has nothing to say here

Both skip anything that is not a draft.

## Badge freshness without a global store

The project has no Context, no store, and no router — state is lifted to the nearest owner and synced through callback props. The badge follows that pattern exactly:

- `draftCount` lives in `AppShell`, which already owns `profile` and `activeSession`
- an `onKnowledgeChanged: () => void` callback is passed down to `KnowledgeSection` and to `StudyChatScreen` (for the extraction modal)
- `AppShell` fetches on mount and re-fetches whenever the callback fires

**Do not introduce Context or a store for one integer.** If prop-drilling ever exceeds three levels, the escape hatch consistent with this codebase is a Wails `knowledge:changed` event subscribed in `AppShell` — the same mechanism as `study:chunk` — not a client-side store.

## Tasks

- [ ] `internal/application/knowledge/review.go` — `CountDrafts(ctx)`, `ApproveAllDrafts(ctx)`, `RejectAllDrafts(ctx)`, each returning the affected count
- [ ] `internal/interfaces/desktop/knowledge.go` — `CountDraftKnowledgeItems`, `ApproveAllDraftKnowledgeItems`, `RejectAllDraftKnowledgeItems`
- [ ] `frontend/src/components/nav-item.tsx` — optional `badge?: number` prop rendering the vendored `Badge` when `> 0`. A prop, **not** a field on `NAVIGATION`, which is static configuration data
- [ ] `frontend/src/components/app-shell.tsx` — own `draftCount`, fetch on mount, pass `onKnowledgeChanged` down
- [ ] `frontend/src/screens/KnowledgeReviewScreen.tsx` — drafts oldest-first (`List(Filter{Status: draft})` already orders `created_at ASC, id ASC`), per-row Approve / Reject, and "Approve all" / "Reject all" behind an `AlertDialog`

## Acceptance Criteria

- The badge shows the correct count of draft items and is absent at zero
- The queue lists all drafts sorted oldest-first
- Approving an item removes it from the queue and updates its status in SQLite
- Rejecting an item deletes it from the database
- The badge updates immediately after each action without navigating away
- Bulk actions ask for confirmation, report the affected count, and leave non-draft items untouched
- Saving drafts from the extraction modal in the study screen updates the badge without a reload
