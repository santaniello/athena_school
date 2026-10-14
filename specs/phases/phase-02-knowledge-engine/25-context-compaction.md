# Phase 2.25 — Context Compaction

## Goal

Let a study session grow indefinitely without hitting the context wall. When the next request
would exceed the warning threshold, Athena folds the oldest part of the conversation into the
persisted summary (2.24) and sends `summary + recent messages` instead of the full history. The
complete history stays in SQLite and in the UI.

This **amends spec 2.6**, which states "no history is truncated, deleted, or summarized
automatically" and that `blocked` is a permanent, monotonic state. Truncation and deletion remain
forbidden. Automatic summarization of the *prompt* (not of the stored history) becomes the normal
path, and the 95% block changes meaning: it is no longer a product limit but a **recoverable
safety net** that exists only when compaction failed or is impossible.

## Decisions

1. **Compaction affects the prompt, never storage.** Messages are not deleted or edited.
2. **Automatic, with notice.** Compaction runs on its own before the next send and the UI is told
   (event `study:context-compacted`). A manual "compact now" action beyond the retry button is
   optional (2.28).
3. **Trigger = the existing 80% warning threshold**, using the resolved model's real context
   length (`ModelContextResolver`), not new 75%/87.5% thresholds.
4. **Hysteresis: trigger at 80%, compact down to a 50% target.** Compacting only to "just below
   80%" would recompact every few turns, paying an LLM call and losing detail each time. The target
   is a domain constant (`CompactionTargetRatio = 0.5`); the Context Builder folds the oldest
   messages until the estimated new prompt is at or below the target.
5. **Order inside `SendMessage`: compaction runs before the blocked check.** Today the blocked
   check is the first thing in the send transaction and rejects before any other work. Under this
   spec the flow is: validate source mode and content → take the in-flight reservation → load the
   session → **if the state is `warning` or `blocked`, or the estimated next prompt crosses 80%,
   try to compact** → only if compaction failed or is impossible *and* the session is `blocked`
   does the send fail with `ErrSessionContextLimitReached` (nothing persisted, no retrieval, no
   provider call, as in 2.6). A successful compaction on a blocked session lowers it to `normal`
   and the send proceeds in the same turn, transparently.
6. **`blocked` is recoverable, not terminal.** The persisted `blocked` state is still restored on
   resume so the alert shows immediately, but every send attempt and the explicit retry action
   tries to compact first. The domain gains an explicit `blocked → normal` (and `warning →
   normal`) transition on successful compaction; the 2.6 monotonicity rule still holds for
   everything else (same model and limit, no compaction).
   - Transient causes (provider down, no credits, timeout): the retry works once the service
     returns.
   - Model with a smaller window: the catalog change already recomputes the state (2.6);
     compaction then applies if it still does not fit.
   - A single message that cannot fit: it is rejected before being persisted; the session itself
     is not blocked by it and the user shortens the text.
   - Only when even a compacted prompt cannot fit (see 8) does the session stay blocked, and
     `Start new session` remains the last-resort alternative.
7. **Output reserve.** The budget is `contextLength − outputReserve − (system + knowledge
   messages) − summary`. `outputReserve` is a domain constant (initial value fixed in the plan
   step, e.g. 10% of the window, bounded) because today no reserve exists and no `max_tokens` is
   set.
8. **Context Builder is pure domain logic.** `domain/study.BuildContext(...)` takes the summary
   (optional), the messages after `covers_until_message_id`, the fixed messages' estimated size and
   the window, and returns which messages to keep verbatim and which to fold, aiming at the
   hysteresis target. It always keeps a minimum number of most recent messages (including the
   current user message) verbatim. If even `summary + minimum recent + fixed messages` exceeds the
   budget, it returns `ErrBudgetImpossible`; the caller does not loop and the blocked fallback
   applies. Token counts use `EstimateTokens`.
9. **Compaction is synchronous, bounded and batched.** The fold goes through `Summarizer` (2.24),
   which batches large inputs and caps the summary size. The first compaction of an already huge
   session therefore runs as several sequential cheap-tier calls inside one turn; the UI shows the
   "compacting…" state. If any call fails or the result is invalid, nothing is persisted and the
   previous summary stays intact. A compaction failure never fails the user's turn by itself
   unless the session is already `blocked` (decision 5); below 95% the turn proceeds with the full
   history as before.
10. **Summary message framing.** The summary is sent as an additional `system` message after the
    knowledge message and before the history, wrapped like `prompt_context.go` wraps retrieved
    data: explicitly marked as untrusted conversation memory, to be used as context about what the
    user studied and struggled with, never as a citable source. In `strict-notes` the prompt
    states the summary does not authorize answers outside the notes. `stripCitationMarkers` still
    applies to verbatim assistant messages.
11. **Context usage is recomputed after compaction.** The persisted measurement is reset to the
    estimate of the *new* prompt (see 6). `measureUsage` already sums the messages actually sent,
    so the next native measurement is consistent.
12. **Concurrency.** Compaction runs under the same in-flight guard as the turn; there is at most
    one compaction per session at a time. The summary write and the context-usage reset share one
    transaction.
13. **Full history is loaded in memory to decide the cut.** Deciding what to fold requires the
    messages after `covers_until_message_id`; the first compaction of a huge session therefore
    reads the whole history (`ListAfter` with an empty cursor). For a local SQLite database even
    thousands of messages are a few megabytes, so this is accepted deliberately; optimize only if
    measured as a problem.
14. **Opening turn unchanged.** `RequestOpeningTurn` builds a one-message request and does not
    compact.
15. **Cost visibility.** Summarization calls are recorded in `usage` (2.24) and appear in the 2.22
    session totals.

## New/changed backend surface

- `domain/study`: `BuildContext` + budget types, `CompactionTargetRatio`, `ErrBudgetImpossible`;
  `ContextUsage` reset-after-compaction operation (lowers `warning`/`blocked` to `normal`);
  `MessageRepository.ListAfter(ctx, sessionID, afterMessageID)`.
- `application/study`: `SendMessage` reorders to try compaction before the blocked rejection;
  `Summarizer` and `ConversationSummaryRepository` become dependencies; `Service.CompactSession(ctx,
  sessionID)` (the explicit retry use case, same path as the send-time compaction, under the
  in-flight guard); `study:context-compacted` callback.
- `infrastructure/sqlite/message_repository.go`: `ListAfter` (order by `created_at, rowid`).
- Desktop: binding `CompactStudySession(sessionID)`; new events `study:context-compacting`
  (started) and `study:context-compacted` with `{sessionId, usedTokens, contextLength}`.
- Spec 2.6 gets an "Amended by 2.25" note on the no-summarization and permanent-`blocked` lines.

## Frontend design

- The blocked alert no longer makes the textarea read-only or disables Send/Enter/source-mode
  (2.6 did): sending is the trigger for recovery. The alert text changes from "limit reached, start
  a new session" to "Could not compact the conversation (reason). Try again."
- Primary alert action: **Try to compact again** (`CompactStudySession`); secondary: **Start new
  session** (unchanged 2.6 behavior).
- The warning alert is cleared when `study:context-compacted` arrives; a transient notice shows
  "Older messages were summarized to keep the conversation going". During a compaction a
  "Summarizing earlier messages…" state replaces the plain thinking indicator.
- `lib/study.ts`: event wrappers and `compactStudySession`. Visual transcript marker is 2.28.

## Tasks

1. `ListAfter` (+ query correct without the 2.26 index).
2. Domain: `BuildContext`, hysteresis target, budget math, minimum-recent rule, `ErrBudgetImpossible`,
   `blocked → normal` transition (table-driven tests; boundaries at exactly 80%, at the 50% target
   and at the reserve).
3. `SendMessage` integration: compaction before the blocked check, success lowers the state and
   the send continues, failure keeps 2.6 behavior (blocked → `ErrSessionContextLimitReached`
   without persisting), summary system message, single transaction.
4. `Service.CompactSession` + binding + events.
5. Frontend: non-blocking composer, new alert copy and actions, notice, compacting state.
6. Amend spec 2.6; `CHANGELOG.md`; README if it describes context limits.

## Acceptance Criteria

- A session whose next prompt would cross 80% compacts once, sends `[system, knowledge, summary,
  recent messages]`, and ends at or below the 50% target; the next several turns do not compact
  again.
- A session already persisted as `blocked` that sends a message compacts first; on success it
  becomes `normal`, the message is sent in the same turn and the user sees only the compaction
  notice.
- A `blocked` session whose compaction fails keeps the 2.6 guarantees (`ErrSessionContextLimitReached`,
  nothing persisted, no retrieval or provider call), shows the retry alert, and a later retry
  succeeds once the provider recovers.
- The first compaction of a very large session (history larger than one summarizer batch) succeeds
  through batched calls and persists one summary; a failure mid-way persists nothing.
- The full message history is unchanged in SQLite and still shown after reopening the session.
- Below 95% a summarizer failure does not fail the turn; behavior falls back to the full history.
- The summary is framed as untrusted memory and is never cited; `strict-notes` turns still return
  the fixed no-match message when retrieval is insufficient.
- Two concurrent turns or compactions on one session never run two compactions; different sessions
  are independent.
- Boundary tests: exactly at threshold; `ErrBudgetImpossible` (no loop, session stays blocked,
  `Start new session` available); model/limit unresolved (no compaction, 2.6 behavior); a single
  oversized message is rejected without blocking the session.
- Mutation testing on `domain` / `application` meets 80%.

## Out of Scope

- Pagination of the UI history (2.26–2.27).
- Transcript marker, summary viewer, session recap and a dedicated "compact now" button beyond the
  retry action (2.28).
- Re-summarizing from scratch, summary version history, per-model tokenizers, streaming the history
  from SQLite in chunks.
