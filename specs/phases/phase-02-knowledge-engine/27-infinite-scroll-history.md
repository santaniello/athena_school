# Phase 2.27 — Infinite Scroll Upward (frontend)

## Goal

Consume the paginated history (2.26): the chat opens on the latest messages and loads older ones
automatically as the user scrolls to the top, without losing their place.

## Decisions

1. **Automatic loading near the top.** When the transcript scroll position is within a threshold
   of the top and `hasMore` is true, fetch the next older page via `ListStudyMessagesBefore`. A
   small "Loading earlier messages…" row shows while fetching; a failure shows an inline retry.
2. **Preserve scroll position on prepend.** Before inserting older messages, record
   `scrollHeight` and `scrollTop`; after render, set `scrollTop` so the viewport stays on the same
   message (layout effect). No visible jump.
3. **Coexists with sticky-bottom.** The 80px follow-the-stream behavior (2.23) is unchanged.
   Prepending older messages never re-enables following; streaming keeps working while older pages
   load.
4. **Single in-flight page request.** Repeated scroll events do not trigger duplicate fetches; a
   page is requested at most once per cursor. Switching sessions cancels/ignores a stale response
   (compare the session id on arrival, as chunk events already do).
5. **Dedup by message id.** If a message already exists locally (e.g. a just-sent message that
   also appears in a late page), it is not inserted twice.
6. **DOM still grows with scrolling.** This is accepted: it only grows as the user explicitly
   scrolls up in 50-message steps. Virtualization is 2.29, only if this proves insufficient.
7. **Citations and sources keep working** for older messages: each page carries its own sources
   (2.26), and `LocalSourcesStrip` / `CitationChip` render as for recent messages.

## Frontend design

- `lib/study.ts`: `listMessagesBefore(sessionId, cursor, limit)` wrapper; `resumeStudySession`
  result carries `hasMore` / `nextCursor`.
- `StudyChatScreen` (or a `useMessageHistory` hook extracted from it): owns messages, `hasMore`,
  `nextCursor`, loading/error state, the top-sentinel/scroll handler and the scroll-restoration
  logic. Keep the hook testable independent of layout.
- Tests need layout mocks: `setup.ts` already polyfills `ResizeObserver` and `getBoundingClientRect`;
  scroll metrics (`scrollHeight`, `scrollTop`) are stubbed per test.

## Tasks

1. `lib/study.ts` wrappers and types; wails binding usage.
2. History hook + top-trigger + loading/error row.
3. Scroll restoration on prepend; dedup; stale-response guard.
4. Tests: loads the next page at the top threshold, restores position, no duplicate requests,
   retry on failure, ignores a stale response after switching session, streaming + sticky-bottom
   regression.
5. Docs: `CHANGELOG.md`; run `npx prettier --check .` in `frontend/`.

## Acceptance Criteria

- Opening a long session renders only the latest page; scrolling to the top loads older pages
  until `hasMore` is false, after which no more requests are made.
- The message under the viewport does not move when an older page is inserted.
- New messages and streaming still follow the bottom as before; scrolling up while streaming is
  not hijacked by page loads.
- A failed page load shows retry and leaves existing messages intact.
- Coverage thresholds (80%) and the touched-files mutation run hold.

## Out of Scope

- Virtualization (2.29), in-conversation search, "jump to message", backend changes.
