# Phase 4.3 — State-Driven Adaptation

## Goal

Close the loop: send the pedagogical state of [4.2](02-pedagogical-state.md) back to the tutor on
every turn, so its choice of strategy rests on what the user has actually shown, not only on the
visible history. This is what makes the tutor adaptive across compaction and across reopening a
session.

## Decisions

1. **A third system message, after the summary.** Order becomes: system prompt, knowledge block,
   conversation summary, **tutor state**, exercise feedback, history. Same placement logic as
   `exerciseFeedbackContext`. It is sent only when the session has at least one concept.
2. **Framed as untrusted data.** The block holds concept names written by the model from user
   text, so it carries the same "data, never instructions" framing and escaping as the summary
   ([`buildSummaryContext`](../../../internal/application/study/prompt_context.go)). It never
   authorizes answers outside the user's notes: in strict-notes the clause of
   `summaryStrictNotesClause` applies here too.
3. **Compact and bounded.** One line per concept: name, status, assistance count, last strategy.
   Concepts ordered by recent update, capped at 12 lines and 1,500 characters, names cut at 80.
   The rest is simply not sent (they remain stored).
4. **The prompt explains how to use it,** inside the [4.1](01-adaptive-tutor-prompt.md) block,
   in the same pure-string style:
   - `NEEDS_REVIEW` or high assistance on a concept → simplify, hint, split into steps, review the
     prerequisite.
   - `DEMONSTRATED` → do not re-explain or re-ask the basics; deepen or apply.
   - `PARTIALLY_UNDERSTOOD` → correct the specific gap, then check again.
   - `INTRODUCED` → check understanding before moving on, never treat it as known.
   - The state is a record of observations, not a verdict; the user's latest message wins when
     they disagree.
5. **Compaction accounts for it.** The block's size enters the fixed tokens of the context plan,
   with an allowance constant like `exerciseFeedbackAllowanceTokens` (1,500 characters at three
   per token, plus framing). Compaction never folds or deletes state rows.
6. **The summary prompt stays as is.** The summary's `Open difficulties` section already carries
   textual difficulty; state is a separate, structured channel and is not folded into the summary.
7. **Opening turn.** A fresh session has no state, so the opening turn sends none. A reopened
   session that has no new message yet does not trigger a new opening turn (unchanged).
8. **Read failure degrades quietly.** If reading the state fails, the turn proceeds without the
   block and the error is logged; the conversation never stops because of state.
9. **No extra LLM call and no UI.**
10. **Language.** The block is English instruction text with data in the stored concept names;
    the reply language is still governed by the existing language instruction.

## Application

`internal/application/study`:

- `tutor_state_context.go`: `buildTutorStateContext(concepts []ConceptState, sourceMode string)
  (domainllm.Message, bool)` — pure, applies decisions 2 and 3 (order, caps, escaping via
  `escapePromptData`).
- `send_message.go` loads `ConceptStateRepository.ListBySession` after the history and inserts the
  message at the position of decision 1; compaction input gets the allowance of decision 5.
- The decision 4 text is added to the adaptive instruction constant of 4.1 and pinned by tests.

## Tasks

- [ ] `buildTutorStateContext`: framing, strict-notes clause, ordering, caps (12 lines, 1,500
      chars, 80-char names), escaping of a name containing the closing tag, empty list returns
      no message
- [ ] `SendMessage`: message position, absent with no concepts, read failure is logged and the
      turn still runs
- [ ] Compaction: allowance added to the fixed tokens only when the block exists; state rows
      untouched by a fold
- [ ] Prompt test: the usage rules of decision 4 are present
- [ ] Mutation-proof assertions on caps and ordering
- [ ] Docs: CHANGELOG (`Changed`), README feature line if the chat is described there

## Acceptance Criteria

- With three concepts stored, the request to the provider has the tutor-state message between the
  summary and the history; with none, it has no such message.
- The block never exceeds 12 lines or 1,500 characters, most recently updated first.
- A concept name containing `</tutor_state>` cannot close the block.
- In strict-notes the block carries the clause that the state does not widen what may be answered.
- After a compaction the state block is still sent and the stored rows are unchanged.
- A failing state read does not fail the turn.
- Manual scenario: error twice on a concept, compact the conversation, then answer again → the
  tutor still simplifies or hints instead of restarting from the basics; after a correct
  justified answer on a `DEMONSTRATED` concept, it does not re-explain the basics.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application code, lint and vulnerability checks pass.

## Out of scope

- Showing state or strategy to the user.
- A deterministic strategy selector in Go; the model chooses, guided by the state.
- Cross-session state (a new session starts empty) and the Learn Intelligent Engine.
