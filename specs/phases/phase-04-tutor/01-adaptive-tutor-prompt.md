# Phase 4.1 — Adaptive Tutor Prompt

## Goal

Replace the rigid study prompt ("open with a question, wait, give feedback, ask a follow-up")
with an adaptive tutor. The tutor picks its approach on every reply: explain directly, ask a
question, give a hint, check understanding, correct a mistake or review a prerequisite. The user
chooses nothing and sees no mode; the tutor is simply better at deciding how to help.

This spec changes the prompt only. No schema, no new state, no UI. The tutor's memory of what the
user understood comes in [4.2](02-pedagogical-state.md) and [4.3](03-state-driven-adaptation.md).

Background: `tutor_spec.md` (the original proposal, written without the app's context). Its three
user-selectable modes are dropped; Direct Instruction and Socratic Questioning become internal
strategies of one adaptive tutor.

## Decisions

1. **One behavior, no modes.** There is no tutor mode, no intervention-level setting and no
   selector. Adaptive is the only behavior of every study session, old and new.
2. **Strategies are prompt instructions, not code.** The prompt names six strategies and the
   rules to choose between them. There is no strategy interface, registry or selector in Go; the
   choice happens inside the model. A new strategy is a new line in the prompt.
3. **The six strategies:** `DIRECT_INSTRUCTION` (explain), `SOCRATIC_QUESTIONING` (one
   investigative question), `GUIDED_PRACTICE` (a small task or example to apply), `COMPREHENSION_CHECK`
   (a short recall question after an explanation), `CORRECTIVE_FEEDBACK` (name and fix a
   misconception), `CONCEPT_REVIEW` (revisit a missing prerequisite). The names are for the
   prompt and, later, the state marker ([4.2](02-pedagogical-state.md)). The prompt forbids writing
   the raw identifiers (like `DIRECT_INSTRUCTION`) in a reply; the tutor may still describe its
   approach in plain words ("let's try a guiding question").
4. **Explicit requests always win.** If the user asks for an explanation, a hint, more depth or a
   simpler version, the tutor does that first, without answering a question with a question.
   This replaces the quick-action buttons of the original proposal: the user steers by text.
5. **Selection heuristics, not a state machine.** Unknown concept → explain. Correct answer with
   a justification → deepen at the same level by changing the angle (apply the idea to a new case,
   ask what would change if a condition changed). Recurring difficulty → simplify, give
   a hint, split into smaller steps. Concept just explained → a short comprehension check, only
   when it helps. Missing prerequisite → review it first.
6. **Anti-loop rules.** Do not turn every reply into a question; at most one main question per
   reply; never repeat a question already asked; after two replies without progress on the same
   point (the user's last two answers on it were wrong or empty), explain or give a hint instead
   of asking again; raise the difficulty only after consistent correct answers on the same point,
   never after a single one. Known limitation: until [4.2](02-pedagogical-state.md) these counts
   are read from the visible history, which a summary may shorten; 4.2 makes them reliable.
7. **Opening turn.** It keeps a short greeting and one focused question to gauge the starting
   point, as today, unless the session goal already shows the user wants to learn a new concept
   from scratch; then it opens with a short explanation. Still no lecture. The prompt describes
   the signals by intent, with examples, not by exact words (the goal text may be in any
   language; replies follow the profile language as today): learn, understand, "from scratch",
   introduction or "I don't know this" → short explanation first; review, practice, test me,
   go deeper, or a vague goal → probing question. When in doubt, ask the question.
8. **Length stays short by default** (a few sentences), deeper only when asked, as today.
9. **Existing prompt contracts do not change.** The offer markers (exercise, quiz, mind map), the
   language instruction, the profile, topic and goal lines, the knowledge and summary system
   messages and the citation rules are untouched. Only the "run this as a real back-and-forth
   session" block is replaced.
10. **No data compatibility to keep.** Existing sessions pick up the new behavior on their next
    message because the system prompt is rebuilt on every turn.
11. **Cost: zero.** No extra LLM call, no extra tokens beyond the longer instruction block.
12. **Prompt quality is not provable by unit tests.** Tests pin that each rule is present in the
    prompt (same approach as [3.10.1](../phase-03-architecture-lab/10-01-tutor-hints.md)
    decision 2). Behavior is checked by the manual scenarios below.

## Application

`internal/application/study/prompt.go`: `buildSystemPrompt` keeps its signature and its header
lines. The behavioral block becomes a package constant (`adaptiveTutorInstruction`, a pure
string) holding decisions 3 to 8, appended where the old block was, before the offer
instructions. The opening turn ([`RequestOpeningTurn`](../../../internal/application/study/opening_turn.go))
uses the same prompt, so decision 7 lives in the same constant.

No port, repository, binding or frontend change.

## Manual scenarios

Run against the real API before merging, with a topic the tester knows:

1. Ask "what is X?" on a new session → a direct explanation, not a question.
2. Answer a question wrongly twice → the third reply simplifies or gives a hint, not a third
   question.
3. Answer correctly with a justification → a deeper question or small task, not a recap.
4. Say "I don't know" → an explanation or hint, no new question first.
5. Say "just explain it" in the middle of a questioning streak → explains immediately.
6. Ask for "more depth" and then "simpler" → each is honored first, without a counter-question.
7. Open a session with a from-scratch goal, then another with a review goal → short explanation
   first, then greeting plus one question.
8. Across all scenarios, no raw strategy identifier (`DIRECT_INSTRUCTION`, ...) appears in a reply.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Prompt test: the six strategies are named and described
- [ ] Prompt test: explicit requests win (explain, hint, deeper, simpler)
- [ ] Prompt test: selection heuristics and the anti-loop rules are present (deepen at the same
      level after a correct answer; raise difficulty only after consistent correct answers)
- [ ] Prompt test: the opening-turn rule (greeting plus one question; short explanation for a
      from-scratch goal; ask when in doubt)
- [ ] Prompt test: the raw strategy identifiers must never be written in a reply
- [ ] Prompt test: the old rigid phrases ("wait for the answer before introducing new material",
      "after each answer, ask a follow-up question") are gone
- [ ] Each rule is pinned by its own distinctive phrase, so no mutant survives on a shared word
- [ ] Replace the old behavioral block; the existing prompt tests for the header, language and
      offer instructions keep passing unchanged
- [ ] Docs: CHANGELOG (`Changed`)

## Acceptance Criteria

- The system prompt no longer forces "wait for the answer before introducing new material" or
  "after each answer, ask a follow-up question".
- The prompt carries all six strategies, the heuristics of decision 5 and the anti-loop rules of
  decision 6.
- A user request for an explanation, a hint, more depth or a simpler version is covered by an
  explicit rule that takes priority over asking.
- The offer markers, language instruction, profile, topic and goal lines are byte-identical to
  before.
- The five manual scenarios pass in a real session.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed application
  code (assertions pin key phrases of each rule, not just that the prompt is non-empty), lint and
  vulnerability checks pass.

## Out of scope

- Any persisted state or marker ([4.2](02-pedagogical-state.md)).
- Tutor mode, intervention level, quick-action buttons, showing the current strategy in the UI.
- Any change to the Architecture Lab, quiz or mind map tutors.
