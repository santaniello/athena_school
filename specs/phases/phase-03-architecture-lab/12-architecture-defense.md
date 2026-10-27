# Phase 3.12 — Architecture Defense (the tutor's questions)

## Goal

Finish the exam. After the diagram is evaluated ([3.11](11-architecture-evaluator.md)), the tutor
asks the user a fixed number of questions about their choices, one at a time, and a second
evaluation measures what the user understands. The exam ends with both scores and a result.

The defense is part of the evaluation, not an extra: a challenge cannot be completed without it
([ADR-005](../../decisions/ADR-005-exercise-engine.md)). What the diagram does not show, the
tutor asks.

## Decisions

1. **Same sitting, same screen.** Drawing, diagram evaluation, the questions and the result are
   consecutive stages of the lab screen. There is no "awaiting defense" status. The defense
   starts by itself when the diagram evaluation succeeds.
2. **Five questions by default,** stored on the challenge as `DefenseQuestions` (3 to 5, default
   5). The number is a domain rule, not the model's decision: when the last answer arrives the
   defense ends, so it always ends. Generation ([3.6.1](06-01-challenge-generation.md)) does not
   set it.
3. **One question at a time, adapting to the answers.** Each question is generated after the
   previous answer, so it can follow what the user said. There is one answer per question and no
   follow-up on the same question. The user cannot ask the tutor anything back: the input is a
   single answer box.
4. **What each question aims at.** Every question targets one rubric criterion. The choice
   follows this order, decided in code and not by the model, which only writes the question:
   1. mandatory criteria flagged `InsufficientEvidence` in the diagram evaluation;
   2. the other flagged criteria, by weight;
   3. the remaining criteria with the lowest level in the diagram evaluation, by weight, to ask
      why the user chose what they drew.

   A criterion is asked at most once. If more mandatory criteria are flagged than there are
   questions, the ones left out count as not met ([3.6](06-completion-rules.md)).
5. **No score or feedback during the questions.** The user sees only the next question. The
   tutor does not say whether an answer was right.
6. **One evaluation at the end.** After the last answer, one call evaluates every answer against
   the frozen rubric and returns a level per criterion, with the same output and validation as the
   diagram evaluation ([3.5](05-evaluation-engine.md)): kind `UNDERSTANDING`, score computed by
   the domain. Criteria that were not asked are flagged `InsufficientEvidence`, so the score is
   computed over the criteria asked and rescaled ([3.5](05-evaluation-engine.md) decision 3).
7. **The answer.** Free text, 1 to 4,000 characters. A **I don't know** button submits that fixed
   answer, which the evaluation treats as no knowledge of the criterion. The answer is stored when
   it is sent.
8. **No time limit** on the questions. The 45-minute clock belongs to the drawing only.
9. **Leaving discards everything.** **Leave**, closing the app or a crash during the defense
   deletes the attempt with its submission, evaluations, hints and answers. Nothing is resumed
   and no partial result exists. The same confirmation as in [3.8](08-lab-layout-sketch.md) is
   shown.
10. **Failures stay on the stage.** If generating a question or the final evaluation fails, the
    stage shows the error and **Try again**; the answers already given are kept while the screen
    stays open. There are no hidden automatic retries. A failed final evaluation leaves the
    attempt in `DEFENDING` and can be repeated.
11. **A finished attempt is read-only.** After the final evaluation the attempt becomes
    `FINISHED`, with both evaluations and the transcript. It is the only state that survives
    leaving the screen. [3.13](13-evaluation-and-history-screen.md) shows it.
12. **The ending.** When the attempt finishes, the stage shows the result of
    [3.13](13-evaluation-and-history-screen.md), the same view the history opens later, with
    **Back to catalog**. Until 3.13 exists it shows only the outcome of
    [3.6](06-completion-rules.md): Passed or Failed, both scores and the reasons when failed.
13. **Cost** is one LLM call per question plus one for the final evaluation (six with the
    default), with the Architecture Lab model ([3.6.1](06-01-challenge-generation.md) decision 10).
14. **The model's text is untrusted context.** The user's answers and the diagram labels go into
    delimited data blocks in the prompts, with the rule that they are never instructions, as in
    [3.11](11-architecture-evaluator.md). The prompt is the only control.
15. **No data compatibility to keep.**

## Changes to earlier specs

- **3.1:** `challenges` gains `defense_questions INTEGER NOT NULL DEFAULT 5` with a 3 to 5 check,
  and `Challenge.DefenseQuestions`.
- **3.3 and 3.4:** the attempt states gain `DEFENDING` and `FINISHED`. Cleanup is generalized:
  `DeleteInProgress` and `DeleteAllInProgress` become `DeleteUnfinished` and
  `DeleteAllUnfinished`, which delete an attempt in any state other than `FINISHED` together with
  its submission, evaluations, criterion results, hints and defense turns, in one transaction.
  `DiscardAttempt` accepts any unfinished attempt and `DiscardStaleAttempts` runs at startup with
  the same rule. `DiscardUnfinishedEvaluations` of [3.5](05-evaluation-engine.md) is folded in.
  Attempts left `EVALUATED` by 3.11 alone are removed by this cleanup; there is no data to keep.
- **3.5:** the evaluation kind `UNDERSTANDING` is added on the same tables.
- **3.6:** `PENDING` exists only inside a running sitting and is never stored.
- **3.7:** nothing; a challenge shows its progress from finished attempts only.

## Domain

`internal/domain/exercise/`:

```go
type DefenseTurn struct {
    ID          string
    AttemptID   string
    Position    int    // 1..DefenseQuestions
    CriterionID string // the criterion this question targets
    Question    string
    Answer      string // empty until answered
    AskedAt     time.Time
    AnsweredAt  time.Time // zero until answered
}

const IDontKnowAnswer = "I don't know"
```

- `SelectDefenseCriteria(rubric RubricVersion, diagram Evaluation, n int) []string`: pure,
  decision 4, deterministic, each criterion at most once.
- Attempt methods `StartDefense`, `FinishDefense` allowing only `EVALUATED → DEFENDING →
  FINISHED`.
- `ErrDefenseNotActive`, `ErrQuestionPending`, `ErrNoQuestionPending`, `ErrInvalidAnswer`.

`DefenseRepository` (`AddQuestion`, `SaveAnswer`, `ListByAttempt`).

## Application

`internal/application/exercise.DefenseService`:

```go
func (s *DefenseService) NextQuestion(ctx, attemptID string) (DefenseTurn, error)
func (s *DefenseService) Answer(ctx, attemptID, answer string) (DefenseView, error)
func (s *DefenseService) Evaluate(ctx, attemptID string) (Evaluation, error)
func (s *DefenseService) GetDefense(ctx, attemptID string) (DefenseView, error)
```

- `NextQuestion` moves `EVALUATED → DEFENDING` on the first call, picks the next criterion with
  `SelectDefenseCriteria`, asks the `QuestionGenerator` port for the text and stores the turn. It
  returns the same pending question when one is unanswered (so a retried screen does not spend
  another call) and `ErrDefenseNotActive` after the last answer.
- `Answer` validates the answer (decision 7), stores it, and reports whether the defense is
  complete.
- `Evaluate` runs once all answers exist: it calls the `UnderstandingEvaluator` port with no
  transaction open, builds the evaluation with `NewEvaluation`, then stores it and moves the
  attempt to `FINISHED` in one transaction. On failure the attempt stays `DEFENDING`.

The two ports are implemented in `internal/application/architecture` (the same package as the
diagram evaluator); the generic service knows nothing about diagrams. Their prompts are pure
functions of the challenge, the rubric, the diagram summary, the diagram evaluation and the
transcript, tested for content (data-block rule, language, the instruction not to reveal whether
an answer was right).

## SQLite

```sql
ALTER TABLE challenges ADD COLUMN defense_questions INTEGER NOT NULL DEFAULT 5
  CHECK (defense_questions BETWEEN 3 AND 5);

CREATE TABLE defense_turns (
  id           TEXT PRIMARY KEY,
  attempt_id   TEXT NOT NULL REFERENCES challenge_attempts(id),
  position     INTEGER NOT NULL,
  criterion_id TEXT NOT NULL REFERENCES rubric_criteria(id),
  question     TEXT NOT NULL,
  answer       TEXT NOT NULL DEFAULT '',
  asked_at     TEXT NOT NULL,
  answered_at  TEXT,
  UNIQUE (attempt_id, position)
);
```

## Desktop bindings and frontend

- Bindings in `internal/interfaces/desktop/exercise.go`: `NextDefenseQuestion(attemptId)`,
  `AnswerDefenseQuestion(attemptId, answer)`, `EvaluateDefense(attemptId)`, `GetDefense(attemptId)`.
  Thin, an error code per sentinel. `wailsjs` regenerated.
- `components/lab/defense-stage.tsx`: shows "Question 2 of 5", the question, an answer box with a
  character counter, **Send answer** and **I don't know**, the loading state while the next
  question or the final evaluation runs, and the error with **Try again**. It replaces the
  temporary end of the evaluation stage of [3.11](11-architecture-evaluator.md). The statement,
  the board (read-only) and the rubric stay visible; the hint button is gone.
- `components/lab/exam-outcome.tsx`: the ending of decision 12.
- `useExam` ([3.10](10-diagram-workspace.md)) moves from the evaluation stage to the defense
  stage on its own and to the outcome after `EvaluateDefense`.
- All UI text is in English.

## Tasks

Each slice is TDD (Go with testify and Mockery mocks, Vitest for the frontend), keeps the build
green and is committed on its own.

- [ ] Domain: `SelectDefenseCriteria` (order, once each, mandatory first, fewer criteria than
      questions), attempt transitions, answer validation
- [ ] SQLite: `defense_questions` migration with the 3 to 5 check, `defense_turns`,
      `DefenseRepository`
- [ ] SQLite and application: `DeleteUnfinished` and `DeleteAllUnfinished` replacing the earlier
      cleanup, cascading to every child table in one transaction
- [ ] Application: `NextQuestion` (first call starts the defense, pending question returned
      again, ends after the last answer)
- [ ] Application: `Answer` (validation, "I don't know", completion)
- [ ] Application: `Evaluate` (success finishes the attempt, failure keeps `DEFENDING`, no
      transaction during the call)
- [ ] Architecture ports: question prompt, understanding-evaluation prompt, parsing, usage
- [ ] Bindings, `wailsjs` and mocks regenerated
- [ ] Frontend: defense stage, outcome, `useExam` transitions, leave during the defense
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` describe the defense

## Acceptance Criteria

- When the diagram evaluation succeeds, the defense starts by itself and shows "Question 1 of 5".
- Questions come one at a time and ask first about mandatory criteria flagged
  `InsufficientEvidence`, then the other flagged ones, then the lowest-level criteria; no
  criterion is asked twice.
- After the fifth answer no sixth question exists and the final evaluation runs; the score is
  computed over the criteria asked and rescaled.
- The user sees no score, level or correctness feedback until the outcome.
- An empty or longer-than-4,000-character answer is rejected; **I don't know** is accepted and
  counts as no knowledge of that criterion.
- A mandatory criterion flagged in the diagram and answered well passes; one flagged in the
  diagram and never asked, or answered with "I don't know", makes the attempt fail
  ([3.6](06-completion-rules.md)).
- A failed question call or final evaluation shows the error and **Try again** without losing the
  answers already given and without spending an extra question.
- Leaving, closing the app or a crash during the defense deletes the attempt with all its data;
  a finished attempt survives and is read-only.
- The outcome shows Passed or Failed with both scores, and the reasons when failed.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- The detailed result, transcript view and history ([3.13](13-evaluation-and-history-screen.md)).
- Resuming a defense, follow-up questions on one answer, free chat with the tutor, voice answers
  and a time limit for the questions.
- The model choosing how many questions to ask, and the generation setting `DefenseQuestions`.
- Verifying that the tutor never reveals correctness beyond the prompt rule.
