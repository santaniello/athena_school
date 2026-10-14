# Phase 2.22 — Session Usage Visibility

## Goal

Make the two token metrics visible in the Study chat and keep them distinct:

- **Context of the next request** — how full the model's context window is. The backend already
  computes and streams it (`StudyContextUsage`, spec 2.6); the chat screen discards the numbers
  and only shows the warning/blocked banner.
- **Accumulated consumption** — total tokens and cost spent by the session across every LLM call.
  The data already exists in the `usage` table; nothing reads it per session.

This is the first increment of the long-conversation work (2.22–2.29). It is small, independent
and gives the visibility needed to validate compaction (2.25) later.

## Decisions

1. **No schema change.** Accumulated tokens and cost are `SUM(input_tokens)`,
   `SUM(output_tokens)`, `SUM(cost)` over `usage WHERE session_id = ?`. This includes HyDE and
   embedding calls, which is the real cost of the session.
2. **A read port, not a new write path.** `domain/llm.UsageReader` (`SessionTotals(ctx, sessionID)
   (UsageTotals, error)`) is implemented by the existing `sqlite/usage_repository.go`. The write
   side (`UsageRecorder`) is untouched.
3. **Never confuse the two numbers.** The UI labels them separately: "Context" (a meter,
   `usedTokens / contextLength`, with the `estimated` flag shown as "~") and "Session usage"
   (tokens and cost). Accumulated tokens are never used to decide anything about context.
4. **`usage.session_id` is `ON DELETE SET NULL`.** Deleting a session drops its totals from this
   view; that is acceptable and not changed here.
5. **Unknown cost is not zero cost.** `usage.cost` stores `0` both for free-model calls and for
   providers that omit `cost`, and this spec does not change the column (no migration). So the
   aggregate also reports `UnpricedCalls`: the number of rows with tokens > 0 and `cost = 0`
   (`UsageTotals{InputTokens, OutputTokens, Cost, UnpricedCalls}`). The UI shows: "—" when the
   session has no recorded calls; the exact total (including 0) when `UnpricedCalls == 0`; and
   "≥ US$ X" with a tooltip ("some calls have no reported cost") when `UnpricedCalls > 0`. This is
   deliberately conservative: a genuinely free call also marks the total as a lower bound. A
   nullable `cost` that separates the two cases is a possible follow-up if that proves noisy.
6. **No new Wails event.** Totals are fetched on resume and refreshed after `study:done` /
   `study:error`; the context meter is driven by the existing `study:context-*` events plus the
   `StudySession.context` returned on resume.

## New/changed backend surface

- `domain/llm`: `UsageTotals{InputTokens, OutputTokens, Cost}` and `UsageReader`.
- `infrastructure/sqlite/usage_repository.go`: `SessionTotals`.
- `application/study`: `Service.SessionUsage(ctx, sessionID)`.
- Desktop binding: `GetStudySessionUsage(sessionID)`.

## Frontend design

- `StudyChatScreen` keeps `usedTokens`, `contextLength` and `model` from `StudyContextUsage` and
  the context events instead of only `contextState`.
- A compact `SessionUsageBar` (new component) in the chat header area: a context meter (progress
  bar with the existing warning/blocked colors) and the session usage line. Typography follows the
  shared scale (2.x `adjust-font-size`).
- `lib/study.ts` gains `getSessionUsage(sessionId)`.

## Tasks

1. `UsageTotals` / `UsageReader` + SQLite implementation (real-DB test).
2. `Service.SessionUsage` + binding + regenerate `wailsjs`.
3. Frontend: keep numeric context in state; `SessionUsageBar`; refresh totals after a turn.
4. Docs: `CHANGELOG.md`.

## Acceptance Criteria

- Resuming a session shows the context meter with the persisted `usedTokens / contextLength`; an
  estimated measurement is marked as such; an unresolved limit shows no percentage.
- After each completed turn the meter and the session totals update without reopening the session.
- Totals include HyDE and embedding calls recorded for the session and exclude other sessions.
- A session with no recorded usage shows "—", not an error.
- A session containing a call with tokens but no reported cost shows the cost as a lower bound
  ("≥ US$ X"), never as an exact figure; a session whose calls all report a cost shows the exact
  total.
- Accumulated tokens never appear in the context meter and vice versa.

## Out of Scope

- Per-message token or cost columns.
- Cost budgets, alerts, or cross-session dashboards.
- Any change to context state computation (2.6) or compaction (2.25).
