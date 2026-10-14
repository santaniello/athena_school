# Phase 2.24 — Conversation Summary (persisted, not yet wired)

## Goal

Introduce the persisted, incremental **conversation summary** that compaction (2.25) will use,
without changing how chat requests are built yet. This increment delivers the table, the ports,
the model task and a `Summarizer` that can be tested in isolation.

## Product premise

A study session is the "notebook" of a topic: it is long-lived and the user returns to it whenever
they want to study the topic again; sessions are not discarded and recreated. Therefore the
summary is also Athena's **pedagogical memory for that session**: not only what was discussed, but
what the user understood, where they struggled and what is still pending. There is deliberately
**no** structured `learning_progress` table and **no** cross-session memory (spec 2.15 isolates
knowledge by session; spec 2.16 removed LLM extraction from conversations; structured progress is
phase 3).

## Decisions

1. **The full history is never deleted.** `messages` stays the source of truth. The summary is a
   derived artifact covering a prefix of the history.
2. **One row per session, replaced on compaction.** `conversation_summaries` keeps the current
   summary only (`session_id` primary key). Older versions are not kept in this increment.
3. **Incremental.** The summarizer input is `previous summary + messages after
   covers_until_message_id (up to a chosen cut)`; output is the new summary. The whole history is
   never re-summarized.
4. **Fixed sections to limit drift.** Because the summary is rewritten many times over months, the
   prompt demands the same markdown sections every time: `## Progress` (concepts understood),
   `## Open difficulties` (concept + short evidence of the mistake or confusion), `## Pending`
   (exercises/questions left open) and `## Discussion log` (what was covered). Markdown, not JSON.
5. **Evidence-preserving, honest.** The prompt instructs the model to record difficulties only when
   the conversation shows them, to keep a single success as "understood so far" rather than
   mastery, and to drop a difficulty only when later messages resolve it.
6. **The summary is untrusted LLM output, not a source.** It is never cited, never retrieved, never
   indexed. `message_sources` of summarized messages stay in the database untouched.
7. **Cheap tier.** New `TaskConversationSummary` in `domain/llm/router.go` mapped to `TierCheap`
   (precedent: `TaskQueryEnrichment`). The call goes through the normal `Chat` port and is
   recorded in `usage` with the session id, so it shows up in 2.22 totals.
8. **Cut selection is domain logic.** Which messages to fold in belongs to the next spec (the
   Context Builder). `Summarizer` accepts an explicit list of messages and the cut message id.
9. **Bounded input per call; folding is done in batches.** A single call must fit the cheap
   model's window with room for the previous summary, the prompt and the output. `Summarizer`
   splits the messages it is given into batches under an input budget (domain constant, estimated
   with `EstimateTokens`) and folds them sequentially: each call's output is the next call's
   "previous summary". Only the final result is persisted, in one `Save`, with
   `covers_until_message_id` set to the last folded message; if any batch fails, nothing is saved
   and the previous summary stays intact. This is what makes the first compaction of an already
   huge session possible. A single message larger than the batch budget is truncated for
   summarization only (the stored message is untouched) with a marker in the prompt.
10. **The summary has a size cap.** The summary is rewritten for months, so it must not grow
    without bound. The prompt receives a target size (domain constant, a fraction of the model
    window, initially ~10%, with an absolute floor/ceiling) and instructs the model to condense
    older material, mainly `## Discussion log`, instead of only appending. `## Open difficulties`
    and `## Pending` are condensed last. After each call the result is validated against a hard
    cap (cap + tolerance): if over, one extra condensing pass runs on the summary alone; if still
    over, the call returns `ErrSummaryTooLarge` and nothing is saved. `token_estimate` is stored
    so the Context Builder (2.25) can subtract it from the budget.

## Schema

```sql
CREATE TABLE IF NOT EXISTS conversation_summaries (
    session_id              TEXT PRIMARY KEY REFERENCES sessions(id) ON DELETE CASCADE,
    content                 TEXT NOT NULL,
    covers_until_message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    token_estimate          INTEGER NOT NULL,
    updated_at              TEXT NOT NULL
)
```

Appended to the `migrations` slice in `infrastructure/sqlite/migrations.go` (idempotent,
`CREATE TABLE IF NOT EXISTS`). `ON DELETE CASCADE` on `covers_until_message_id` means deleting that
message removes the summary; messages are only deleted with their session today, so this is
consistent. Foreign keys are enforced on every pooled connection (`foreign_keys(1)` is set in the
DSN in `sqlite.Open`), so the cascade is reliable. If a per-message delete is ever introduced, it
must be revisited, since it would silently drop the summary.

## New backend surface

- `domain/study`: `ConversationSummary{SessionID, Content, CoversUntilMessageID, TokenEstimate,
  UpdatedAt}`, `ConversationSummaryRepository` (`Get`, `Save`), constructor validation (non-blank
  content, non-empty ids, non-negative estimate).
- `domain/llm/router.go`: `TaskConversationSummary` → `TierCheap`.
- `application/study`: `Summarizer` (`Summarize(ctx, sessionID, previous *ConversationSummary,
  messages []Message) (ConversationSummary, error)`) with the prompt builder (`summary_prompt.go`,
  localized to the profile's assistant language like `prompt.go`).
- `infrastructure/sqlite`: repository implementation.
- Mocks via `make mock`.

## Tasks

1. Domain type + repository port + validation tests.
2. Migration + SQLite repository (real-DB tests, including legacy-DB migration test).
3. Router task + test.
4. `Summarizer` + prompt builder (unit tests with mocked `Chat`; assert the fixed sections are in
   the prompt, previous summary and messages are included, empty/invalid LLM output is an error,
   usage is attributed to the session).
5. Docs: `CHANGELOG.md`.

## Acceptance Criteria

- A summary can be saved, replaced and read back per session; deleting the session removes it.
- `Summarizer` returns a new summary built from the previous summary plus only the given messages;
  a blank LLM response returns an error and persists nothing.
- A message list larger than one batch is folded in several sequential calls; no call exceeds the
  input budget; a failure in any batch persists nothing and keeps the previous summary.
- A summary over the cap triggers one condensing pass; still over → `ErrSummaryTooLarge`, nothing
  saved. Repeated incremental runs on a long synthetic history keep `token_estimate` under the cap.
- The summarization call is recorded in `usage` under the session id with the cheap-tier model.
- No change to `SendMessage`, retrieval, prompts or the UI.
- Mutation testing on `domain` / `application` meets the 80% thresholds.

## Out of Scope

- Deciding when to compact or which messages to fold (2.25).
- Sending the summary to the model (2.25), UI of any kind (2.28).
- Summary version history, cross-session memory, structured learning progress.
