# Phase 2.26 — Paginated Message History (backend)

## Goal

Stop loading the whole conversation when a session is opened. Once compaction (2.25) lets sessions
grow without bound, `ResumeStudySession` returning every message and every `message_sources` row
becomes the next bottleneck. This spec adds cursor-based reads and makes resume return only the
most recent page.

## Decisions

1. **New read method; `ListBySession` is untouched.** `SendMessage` uses `ListBySession` to build
   the LLM request (and 2.25 uses `ListAfter`). The UI uses the new paged method. They must not
   share a code path.
2. **Cursor is a message id; order is `(created_at, rowid)`.** Message ids are `TEXT` (not
   sequential), so ordering uses `created_at` with SQLite's `rowid` as tiebreaker when timestamps
   collide. The cursor handed to the frontend is the id of the oldest message in the page (opaque
   to the frontend). The repository resolves `(created_at, rowid)` with a subselect scoped by
   `session_id`, the same pattern as `ListAfter`; an empty cursor id is invalid, and an id that is
   not a message of the session yields `study.ErrMessageNotInSession` (an error, never an empty
   page). `messages` has an implicit `rowid` (`id TEXT PRIMARY KEY`) that `VACUUM` may renumber,
   so the `rowid` is used only as an in-query tiebreaker and never leaves the database.
3. **Page shape.** `MessagePage{Messages (chronological), NextCursor (older), HasMore}`. Default
   page size 50, maximum 200, enforced in the application layer.
4. **Resume returns the latest page.** `ResumeStudySession` returns the session, the last 50
   messages and `hasMore`/`nextCursor`. The Wails DTO stays backward compatible for existing
   fields. Message ids already reach the frontend by then (added in 2.23).
5. **Sources per page, not per session.** `MessageSourceRepository` gains
   `ListByMessageIDs(ctx, ids)`; paged reads load sources only for the returned messages. The
   existing `ListBySession` stays for other callers.
6. **Index.** Add `CREATE INDEX IF NOT EXISTS idx_messages_session_created ON messages(session_id,
   created_at)` to the migrations slice (messages has none today). The paged query and `ListAfter`
   (2.25) both use it.
7. **Newest-first query, chronological result.** The SQL orders `DESC` with `LIMIT n+1` (to detect
   `hasMore`) and the application reverses the page.

## New/changed backend surface

- `domain/study`: `MessagePage`, `MessageCursor` (thin wrapper over a message id; non-empty),
  `MessageRepository.ListPage(ctx, sessionID, before *MessageCursor, limit int)`.
- `domain/knowledge`: `MessageSourceRepository.ListByMessageIDs`.
- `application/study`: `Service.ListMessagesBefore(ctx, sessionID, cursor, limit)`; `Resume` uses
  the latest page. **`Service.Resume` changes signature**: it returns the session plus a page
  (messages with sources, `HasMore`, `NextCursor`) instead of `[]MessageWithSources`; update its
  callers and mocks.
- `infrastructure/sqlite`: index migration, `ListPage`, `ListByMessageIDs`.
- Desktop: `ListStudyMessagesBefore(sessionID, cursor, limit)` binding; `ResumeStudySession`
  DTO gains `hasMore` and `nextCursor`; regenerate `wailsjs`.

## Tasks

1. Cursor type + validation tests (domain).
2. Migration (index) + `ListPage` + `ListByMessageIDs` (real-DB tests: ordering with equal
   timestamps, page boundaries, exact multiples of the limit, empty session, session isolation,
   a cursor id from another session or unknown → `ErrMessageNotInSession`, migration on a legacy
   DB creating the index without altering existing messages).
3. Application use case with limit clamping; `Resume` returns the latest page.
4. Binding + DTO + `wailsjs` regeneration.
5. Docs: `CHANGELOG.md`.

## Acceptance Criteria

- Opening a session with N > 50 messages returns exactly the 50 most recent, chronological, with
  their sources only, `hasMore=true` and a valid cursor.
- Following `nextCursor` repeatedly returns every older message exactly once, in order, with no
  duplicates or gaps, including when several messages share a `created_at`.
- A session with ≤ 50 messages returns everything with `hasMore=false`.
- A malformed cursor or a cursor from another session returns an error and no data.
- `SendMessage` behavior and its history reads are unchanged.
- The index exists after migrating both fresh and legacy databases.

## Out of Scope

- Any frontend change (2.27), message search (FTS), virtualization (2.29).
