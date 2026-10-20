# Phase 2.27 — Infinite Scroll Upward (frontend)

## Goal

Consume the paginated history (2.26): the chat opens on the latest messages and loads older ones
automatically as the user scrolls to the top, without losing their place.

## Decisions

1. **Automatic loading via a top sentinel.** An empty element at the top of the transcript is
   observed with an `IntersectionObserver` (`rootMargin` = a named constant, 200px, next to
   `STICKY_BOTTOM_THRESHOLD_PX`). When it is visible, `hasMore` is true and no request is in
   flight, fetch the next older page via `ListStudyMessagesBefore`. The observer re-evaluates after
   every prepend, so a short first page (no scrollbar, hence no scroll events) keeps loading until
   the viewport is filled or `hasMore` is false. A small "Loading earlier messages…" row shows
   while fetching; a failure shows an inline retry, tracked in the hook's own error state (not the
   screen's global `error` alert). While in the error state the sentinel does **not** re-trigger;
   only the retry action does.
2. **Preserve scroll position on prepend.** Before inserting older messages, record
   `scrollHeight` and `scrollTop`; after render, set `scrollTop` so the viewport stays on the same
   message (layout effect). No visible jump. The transcript sets `overflow-anchor: none` so the
   manual restoration is deterministic across WebView2 (native scroll anchoring) and WebKitGTK
   (none).
3. **Coexists with sticky-bottom.** The 80px follow-the-stream behavior (2.23) is unchanged.
   Prepending older messages never re-enables following and never scrolls to the bottom: the
   `messages` effect that calls `scrollToBottomIfFollowing` must skip prepends (only appends and
   the initial hydration follow). Streaming keeps working while older pages load.
4. **Single in-flight page request.** Repeated scroll events do not trigger duplicate fetches:
   at most one request per cursor is in flight at a time. The dedup applies only to concurrent
   calls; a failed request leaves the cursor available so retry (decision 1) can request it again,
   and a successful one advances to `nextCursor`. Switching sessions cancels/ignores a stale
   response (compare the session id on arrival, as chunk events already do). The same guard is
   added to the initial resume, and hasMore/cursor/in-flight state is reset on session change.
5. **Dedup by message id (defense in depth).** The backend never returns overlapping pages (every
   page is strictly older than its cursor), but if a message id already exists locally it is not
   inserted twice.
6. **DOM still grows with scrolling.** This is accepted: it only grows as the user explicitly
   scrolls up in 50-message steps. Virtualization is 2.29, only if this proves insufficient.
7. **Citations and sources keep working** for older messages: each page carries its own sources
   (2.26), and `LocalSourcesStrip` / `CitationChip` render as for recent messages.

## Frontend design

- `lib/study.ts`: `listMessagesBefore(sessionId, cursor, limit)` wrapper; `resumeStudySession`
  result carries `hasMore` / `nextCursor` (and its stale "full message history" comment is fixed).
- `hooks/use-message-history.ts` (`useMessageHistory`): a pagination-only hook. It owns `hasMore`,
  `nextCursor`, the loading/error status, and exposes `loadOlder` / `retry`; it hands each fetched
  page to the screen through a callback. `messages` stays in `StudyChatScreen`, which already
  appends on send and streaming and performs the prepend plus scroll restoration. Keep the hook
  testable independent of layout. Test: a failed page load followed by retry issues a second
  request for the same cursor.
- Tests need layout mocks: `setup.ts` already polyfills `ResizeObserver` and `getBoundingClientRect`;
  it gains an `IntersectionObserver` mock whose callback tests can fire; scroll metrics
  (`scrollHeight`, `scrollTop`) are stubbed per test.

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
- A first page too short to scroll keeps loading older pages until the viewport is filled or
  `hasMore` is false.
- After a failed page load the sentinel does not re-trigger by itself; only retry requests again.
- The message under the viewport does not move when an older page is inserted.
- New messages and streaming still follow the bottom as before; scrolling up while streaming is
  not hijacked by page loads.
- A failed page load shows retry and leaves existing messages intact.
- Coverage thresholds (80%) and the touched-files mutation run hold.

## Out of Scope

- Virtualization (2.29), in-conversation search, "jump to message", backend changes.
