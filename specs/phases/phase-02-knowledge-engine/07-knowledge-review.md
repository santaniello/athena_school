# Phase 2.7 — Knowledge Review

## Goal

Drafts accumulated from extractions surface in a dedicated review queue so the user can approve or reject them efficiently.

## Tasks

- [ ] Review queue screen: lists all `draft` items sorted by creation date (oldest first)
- [ ] Per-item actions: Approve / Reject (reject = delete)
- [ ] Bulk actions: "Approve all" / "Reject all" with confirmation
- [ ] Navigation badge: pending draft count shown on the main nav icon
- [ ] Badge clears when draft count reaches zero

## Acceptance Criteria

- Badge shows the correct count of draft items
- Approving an item removes it from the queue and updates its status in SQLite
- Rejecting an item deletes it from the database
- Badge updates immediately after each action without navigating away
