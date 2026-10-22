# Phase 2.28 — Compaction UX

> **Status: scoped.** Scope re-grilled and reduced after 2.25 shipped (PR #114). Product reference:
> ChatGPT / Claude, where compaction is automatic and has no manual controls.

## Goal

Make compaction legible. After 2.25 the model works from a summary instead of the older turns
verbatim; the user should be able to see where that happened and read what Athena remembers.

## Scope

1. **Transcript marker.** An orange divider ("Earlier messages were summarized") placed right
   after the message identified by `covers_until_message_id`. It has no action of its own.
   Requires returning the summary cursor with the resume DTO and updating it live when a
   compaction happens.
   - Cut message loaded: the marker goes right after it.
   - Whole loaded window is after the cut: the marker goes above the first loaded message (after
     the history sentinel once `hasMore` is false).
   - No summary: no marker.
   - Must not break the `scrollTop` restore or the `IntersectionObserver` of the paginated
     history (2.26/2.27).
2. **View summary.** A "View summary" button in the session header, to the left of the source-mode
   selector, always visible but disabled until the session has a summary (unlocked by the first
   compaction). The summary belongs to the whole
   session (one row, replaced and cumulative on each compaction, see 2.24), not to the marker.
   It opens a read-only dialog with the current summary and its fixed sections from 2.24
   (`Progress`, `Open difficulties`, `Pending`, `Discussion log`). The content is untrusted LLM
   output: rendered without raw HTML or live links. No editing.

## Not planned

- **Compact now button.** Automatic compaction (2.25) plus the existing retry action is enough;
  ChatGPT/Claude expose no manual control. Revisit only if automatic compaction proves unreliable.
- **Session recap on reopen.** A new kind of tutor turn (own prompt, idle threshold, opt-out, rule
  against fabricating progress). Not wanted for the product; the marker and the summary viewer re-orient
  the user at no LLM cost.

## Constraints carried from earlier specs

- The summary is untrusted LLM output and never a citable source (2.24, 2.25).
- No structured learning progress table and no cross-session memory (2.24 product premise).
- Business logic stays in `domain`/`application`; bindings stay thin (ADR-001).

## Design

### Backend
- `application/study` `Resume` also receives the `ConversationSummaryRepository` and returns the
  summary's `CoversUntilMessageID` next to the history page; a missing summary
  (`ErrConversationSummaryNotFound`) means "no marker", not an error.
- New read use case returning `Content`, `CoversUntilMessageID` and `UpdatedAt`; not found yields
  an empty result.
- `StudySessionHistoryResult` gains an optional `summaryCoversUntilMessageId`; new thin binding
  `GetStudySessionSummary`.

### Frontend
- Marker row in `StudyChatScreen`, refreshed from the `study:context-compacted` handler.
- Header button and read-only summary dialog; wrappers in `lib/study.ts`.

## Open questions

None.
