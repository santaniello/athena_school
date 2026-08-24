# Phase 2.7 — Knowledge Review

## Goal

Drafts accumulated from extractions surface in the existing Explorer/Review tab so the
user can approve or reject them one at a time, deliberately, with a badge that keeps
the sidebar honest about how many are waiting.

## Scope decision: item-by-item only, bulk actions deferred

Per-item Approve/Reject already exists today in `KnowledgeExplorerScreen` (mode
`review`), including a full, untruncated detail panel (concept, definition,
properties, trade-offs, related concepts) shown before the user acts — this already
satisfies "read the content consciously before approving." No new preview needs to be
built.

Given that, this increment does **not** implement `ApproveAllDrafts` / `RejectAllDrafts`
or any bulk-action UI. The user wants approval and rejection to stay item-by-item for
now, so each decision is deliberate. Bulk actions (with their own partial-failure
semantics — stop-on-error vs. skip-and-report, still undecided) are a **separate future
increment**, to be spec'd on its own. The `ApproveAllDrafts`/`RejectAllDrafts` design
notes below are kept for that future spec, not for this one:

- `ApproveAllDrafts` would iterate per item **through `TransitionTo`**, not a single
  bulk `UPDATE` — approval must take the same path as the single-item action, and N is
  small on a local database with one connection
- `RejectAllDrafts` would iterate `Delete` per item. There is no `draft → deleted`
  transition, so `TransitionTo` has nothing to say here
- Both would skip anything that is not a draft

**Because bulk actions are out of scope, `KnowledgeReviewScreen.tsx` is not created in
this increment.** The existing Review tab inside `KnowledgeSection` (rendering
`KnowledgeExplorerScreen` with `mode="review"`) stays as the UI; it only gains the
`onKnowledgeChanged` wiring described below so the badge stays live.

## Badge freshness without a global store

The project has no Context, no store, and no router — state is lifted to the nearest
owner and synced through callback props. The badge follows that pattern exactly:

- `draftCount` lives in `AppShell`, which already owns `profile` and `activeSession`
- an `onKnowledgeChanged: () => void` callback is passed down to `KnowledgeSection`
  (threaded into `KnowledgeExplorerScreen`'s approve/delete handlers so it fires after
  any action that can change the draft count) and to `StudyChatScreen` (for the
  extraction modal, `KnowledgeExtractionDialog`)
- `AppShell` fetches on mount and re-fetches whenever the callback fires

**Do not introduce Context or a store for one integer.** If prop-drilling ever exceeds
three levels, the escape hatch consistent with this codebase is a Wails
`knowledge:changed` event subscribed in `AppShell` — the same mechanism as
`study:chunk` — not a client-side store.

**Where the badge renders**: the count shows in **two places**, both reading the same
`AppShell`-owned `draftCount` — no separate local fetch anywhere else:

- the existing `knowledge` entry in the sidebar `NAVIGATION`, via a new optional
  `badge?: number` prop on `nav-item.tsx` (a prop, **not** a field on `NAVIGATION`,
  which is static configuration data). `NAVIGATION` is a closed, one-entry-per-phase
  list — Review does **not** get its own nav entry
- the "Review" tab inside `KnowledgeSection`, which already renders a `Badge` today
  from a locally-fetched count; that local fetch is removed and replaced by the
  `draftCount` prop coming down from `AppShell`

Spec 2.11 extends Review with pending reconciliation proposals. They render in a
separate group and never count as draft Knowledge Items. At that point the navigation
badge becomes `reviewCount = draftCount + pendingProposalCount`, while the two counts
remain separately labelled in the screen. This still applies once bulk actions and
reconciliation proposals are built in later increments.

## Tasks

- [ ] `internal/application/knowledge/review.go` — `CountDrafts(ctx)` returning the
      count (`s.items.CountByStatus(ctx, StatusDraft)` already exists on the
      repository)
- [ ] `internal/interfaces/desktop/knowledge.go` — `CountDraftKnowledgeItems`
- [ ] `frontend/src/components/nav-item.tsx` — optional `badge?: number` prop
      rendering the vendored `Badge` when `> 0`
- [ ] `frontend/src/components/app-shell.tsx` — own `draftCount`, fetch on mount via
      `CountDraftKnowledgeItems`, pass `onKnowledgeChanged` down to `KnowledgeSection`
      and `StudyChatScreen`, pass `draftCount` down to render the nav badge
- [ ] `frontend/src/components/knowledge-section.tsx` — drop the local `draftCount`
      fetch; take `draftCount` as a prop for the Review tab's `Badge`; thread
      `onKnowledgeChanged` into `KnowledgeExplorerScreen`'s approve/delete handlers
- [ ] `frontend/src/components/knowledge-extraction-dialog.tsx` — call
      `onKnowledgeChanged` after a successful drafts save

## Acceptance Criteria

- The badge shows the correct count of draft items and is absent at zero, in both the
  sidebar `Knowledge` nav item and the Review tab
- The queue lists all drafts sorted oldest-first (already the case —
  `List(Filter{Status: draft})` orders `created_at ASC, id ASC`)
- Approving an item removes it from the queue and updates its status in SQLite
- Rejecting an item deletes it from the database
- The badge updates immediately after each action without navigating away
- Saving drafts from the extraction modal in the study screen updates the badge
  without a reload

## Deferred to a future increment

- `ApproveAllDrafts` / `RejectAllDrafts` (application layer) and
  `ApproveAllDraftKnowledgeItems` / `RejectAllDraftKnowledgeItems` (desktop interface)
- Bulk-action UI ("Approve all" / "Reject all" behind an `AlertDialog`) in a
  `KnowledgeReviewScreen.tsx`
- Partial-failure semantics for bulk actions (stop-on-error vs. skip-and-report) —
  raised during design of this phase, not yet decided
- The corresponding acceptance criteria: bulk actions ask for confirmation, report the
  affected count, leave non-draft items untouched, and leave reconciliation proposals
  (2.11) untouched
