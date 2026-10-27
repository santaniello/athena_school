# Phase 3.16 — Exam Feedback in the Study Session

## Goal

Let the tutor know how the user did. When an exam started from a study session finishes, a short
summary of the result joins the context of the session's next messages, so the user can ask "how
did I do?" or "what should I review?" and get an answer grounded in the real evaluation.

It is a bounded summary, not the diagram or the transcript, in the same spirit as the
conversation summary ([2.24](../phase-02-knowledge-engine/24-conversation-summary.md)).

## Decisions

1. **A short result summary, nothing else.** For each finished attempt of the session the context
   carries: the challenge title, topic and difficulty, the outcome (Passed or Failed), the two
   scores and the minimum passing score, the mandatory criteria that were not met, up to three
   gaps and up to three recommendations. It never carries the diagram XML, the tutor's questions
   and answers, the per-criterion justifications or the hints.
2. **Derived, not stored.** The summary is built when a message is sent, from the finished attempts
   that belong to the session through its `SessionExercise`
   ([3.3](03-attempts-and-session-link.md)). There is no new table and no copy that can drift from
   the real evaluation. An attempt started from the catalog, with no session, contributes nothing.
3. **Built by code, not by the model.** The text is composed deterministically from stored values:
   no LLM call, no cost. Gaps and recommendations are taken from the evaluations in order; the
   model's own wording is quoted, never rewritten.
4. **Bounded.** The most recent three finished attempts of the session, newest first, and at most
   2,000 characters in total. The oldest attempt is dropped first, and a gap or recommendation is
   cut at 200 characters.
5. **It is untrusted data.** The evaluations contain model output derived from the user's diagram
   and answers, so the block is framed as the conversation summary is: reference data to inform
   the answer, never instructions, never cited as a source, escaped so it cannot close its own
   delimiter ([2.24](../phase-02-knowledge-engine/24-conversation-summary.md)). In strict-notes mode
   it does not authorize answers outside the user's notes.
6. **Where it sits.** A system message after the knowledge context and the conversation summary
   and before the history, built next to `buildSummaryContext`. It counts against the prompt
   budget like the other blocks ([2.6](../phase-02-knowledge-engine/06-study-context-limits.md)),
   inside its own 2,000-character cap.
7. **The tutor does not speak first.** Nothing is posted into the chat when an exam finishes. The
   result is available to the next reply the user asks for. The persistent card of
   [3.17](17-session-exercises-ui.md) is what the user sees in the chat.
8. **Only finished attempts.** There are no unfinished attempts to report
   ([3.12](12-architecture-defense.md)); a discarded exam leaves no trace in the conversation.
9. **English text.** The block is written in English; the reply language still follows the
   assistant language in the profile.
10. **No data compatibility to keep.**

## Application

In `internal/application/study`, which keeps no lab code:

- a narrow port `ExerciseFeedbackReader` with
  `SessionExerciseFeedback(ctx, sessionID string) ([]ExerciseFeedback, error)`, implemented by the
  lab application layer, where `ExerciseFeedback` is a plain struct holding the values of
  decision 1;
- `buildExerciseFeedbackContext(feedback []ExerciseFeedback, sourceMode string) (domainllm.Message,
  bool)`: pure, applies decisions 4 to 6 and returns no message when there is nothing to say;
- `SendMessage` asks the reader on every send and adds the message to the context; an error from
  the reader is logged and the message is sent without the block, so feedback never blocks the
  chat.

In `internal/application/exercise`:

- `FeedbackService.SessionExerciseFeedback(ctx, sessionID)` lists the session's finished attempts
  through the repositories of [3.3](03-attempts-and-session-link.md) to
  [3.5](05-evaluation-engine.md) and the outcome of [3.6](06-completion-rules.md), takes the three
  most recent and fills the struct. It adds no rule.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks), keeps the build green and is
committed on its own.

- [ ] `FeedbackService`: finished attempts of a session only, three most recent, fields of
      decision 1, nothing for a standalone attempt or a session with none
- [ ] `buildExerciseFeedbackContext`: framing, escaping, strict-notes clause, caps (200 per item,
      2,000 total, oldest dropped first), empty input gives no message
- [ ] `SendMessage`: reader called on each send, block placed after the summary, reader error
      does not fail the send
- [ ] Context budget: the block counts toward the prompt budget (test with a full context)
- [ ] Mocks regenerated; docs: CHANGELOG

## Acceptance Criteria

- After an exam started from a session finishes, the next reply is built with a block that has the
  title, outcome, both scores, the minimum score, unmet mandatory criteria, at most three gaps and
  three recommendations.
- The block never contains the XML, the tutor's questions and answers, justifications or hint
  text.
- An attempt from the catalog, a discarded exam and a session with no finished attempt add no
  block.
- With more than three finished attempts only the three most recent are included; the total never
  passes 2,000 characters and each gap or recommendation is cut at 200.
- The block is framed as untrusted data, is never citable, cannot close its own delimiter, and in
  strict-notes mode does not widen the allowed answers.
- A failure of the feedback reader does not stop the message from being sent.
- No LLM call is made to build the block.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks pass.

## Out of scope

- The tutor commenting on its own when an exam ends, and any message written into the chat.
- Including the diagram, the transcript, the justifications or hints, and summaries made by the
  model.
- The persistent exercise card and the Exercises tab ([3.17](17-session-exercises-ui.md)).
- Feedback for attempts that did not start from a session.
