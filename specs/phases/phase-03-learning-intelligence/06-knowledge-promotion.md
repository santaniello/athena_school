# Phase 3.6 — Knowledge Promotion

## Goal

After sessions, the user is prompted to promote draft Knowledge Items to approved, which triggers automatic flashcard generation.

## Flow

```text
Session ends
    ↓
Modal: "Promote these draft items?" (list)
    ↓
User approves items → status = "approved"
    ↓
"Generate flashcards for approved items?" (confirmation)
    ↓
Flashcards created as "draft" (see 3.5)
```

## Tasks

- [ ] Post-session modal with draft items pending promotion
- [ ] Batch approve: user can check/uncheck individual items
- [ ] After approval: optional flashcard generation prompt
- [ ] If user declines flashcard generation, approved item remains without cards

## Acceptance Criteria

- A session that extracted knowledge shows the promotion modal on close
- Approving one item and declining another results in the correct status for each
- Accepting flashcard generation creates draft cards for every approved item
- Declining flashcard generation creates no cards and saves no changes to flashcards table
