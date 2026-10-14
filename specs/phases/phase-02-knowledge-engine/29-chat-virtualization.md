# Phase 2.29 — Chat Virtualization (conditional)

> **Status: conditional.** Implement only if, after 2.23 (cheap rendering) and 2.27 (paginated
> loading), real sessions still make scrolling or streaming sluggish because too many message
> components are mounted. Gather evidence first (React Profiler on a long session); do not build
> this speculatively.

## Goal

Render only the messages near the viewport, unmounting the rest without discarding their data, so
DOM size stays bounded no matter how far up the user has scrolled.

## Why it is deferred

- Message heights are dynamic: markdown, syntax-highlighted code blocks, citation chips, source
  strips and a streaming bubble that grows chunk by chunk. A virtualizer must measure and
  re-measure rows.
- It interacts with the three scroll behaviors already in place: sticky-bottom while streaming
  (2.23), scroll restoration when older pages are prepended (2.27) and jumping to a citation.
- jsdom has no layout: every test needs mocked measurements, making the suite more brittle.
- 2.23 + 2.27 already bound the work per interaction: finished bubbles do not re-render, and the
  DOM only grows in 50-message steps when the user chooses to scroll up.

## Candidate approach

- `@tanstack/react-virtual` (`useVirtualizer` with `measureElement` for dynamic heights and a
  small overscan), keyed by message id.
- The streaming bubble is rendered outside the virtualized list (pinned at the bottom) so its
  growth does not invalidate row measurements.
- Keep the 2.27 page loader: the virtualizer's item count grows as pages are prepended, with scroll
  anchoring on the first visible item.
- Accessibility: keep `role="log"` semantics and `aria-label="Conversation"`; verify screen-reader
  behavior with unmounted rows.

## Acceptance Criteria (when activated)

- Mounted `MessageBubble` count stays bounded (viewport + overscan) on a 1,000-message session.
- Sticky-bottom, streaming, prepend-without-jump and citation navigation behave as in 2.23/2.27.
- Coverage thresholds hold with documented layout mocks; the mutation run on touched files does
  not regress.

## Out of Scope

- In-conversation search and "jump to message" (separate decision, would use SQLite FTS5).
