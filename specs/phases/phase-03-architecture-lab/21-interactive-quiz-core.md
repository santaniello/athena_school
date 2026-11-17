# Phase 3.21 — Interactive Quiz: Core

## Goal

Add the Interactive Quiz: an exam the user asks for from a study session, with N questions about
what was studied there, graded from 0 to 10 and kept in a history that can be opened later. This
spec is the backend core. The menu section is 3.22 and the offer inside the study chat is 3.23.

The quiz is a new bounded context ([ADR-007](../../decisions/ADR-007-interactive-quiz.md)). It
does not reuse the exercise engine: that one is a rubric scored 0–100 with an oral defense.

## Decisions

1. **Always tied to a session.** A quiz is generated from one study session (summary, notes and
   profile context). There is no free-topic quiz.
2. **Question kinds.** `MULTIPLE_CHOICE` (2–6 options) and `TRUE_FALSE` (exactly 2 options). Each
   question has one correct option and an explanation.
3. **Grading is deterministic and lives in the domain.** `score = round(correct / total × 10, 1)`.
   A blank answer counts as wrong. The LLM never grades.
4. **An attempt exists only when submitted.** The frontend keeps the answers in memory and calls
   `SubmitQuizAttempt` once. No `IN_PROGRESS` state, no unique index. A quiz can be retaken, and
   each attempt is one history row.
5. **Answer position is shuffled in code**, through an injectable shuffler, because models place
   the correct option in predictable positions.
6. **Generation is one LLM call**, no retry, in the profile language. The output is strict JSON,
   validated by the domain: the count matches the request (3–20, default 10), every option is
   non-empty and unique inside its question, the correct index is in range, true/false has two
   options. Any failure wraps `ErrInvalidGeneration` and nothing is stored.
7. **Session context comes through a consumer-side port**, `SessionContextReader`, implemented by
   an adapter next to `internal/infrastructure/exercisesource/`. The quiz packages do not import
   `exercise`.
8. **Lifecycle is decoupled from the session.** `quizzes.session_id` has no cascade and the quiz
   keeps a snapshot of the session title. Only an explicit `DeleteQuiz` removes a quiz, with its
   questions, attempts and answers.
9. **LLM routing.** New `TaskQuiz` at the medium tier, with a model setting `models.quiz` in
   Settings. Generation is cancellable, like `CancelChallengeGeneration`.
10. **Persistence.** Tables `quizzes`, `quiz_questions`, `quiz_attempts`, `quiz_answers`, appended
    to the migration slice, and added to `reset.go` and `reset_test.go` (children first).
11. **Bindings** in `internal/interfaces/desktop/quiz.go`, errors mapped to `QuizError{Code}`:
    `GenerateQuiz`, `CancelQuizGeneration`, `ListQuizzes`, `GetQuiz` (no answer key),
    `SubmitQuizAttempt`, `ListQuizAttempts`, `GetQuizAttemptResult` (answer key and explanations),
    `DeleteQuiz`. Session choice reuses `ListSessionsForGeneration`. The `wailsjs` files are
    regenerated.

## Tests (TDD, in this order)

Domain scoring table (all right, all wrong, blank, rounding), question validation, prompt builder,
parser failures (bad JSON, wrong count, index out of range, bad true/false), the generation and
submit services with mockery mocks, SQLite repositories against a real database, the bindings, and
the reset test.

## Out of scope

The UI (3.22), the study chat offer (3.23), open-ended questions, a pass/fail threshold, timers,
resuming an unfinished attempt, and sharing or exporting a quiz.
