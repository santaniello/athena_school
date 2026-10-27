# Phase 3.13 — Result and History

## Goal

Show what a finished exam produced, now and later. One result view serves two entry points: it is
the last stage of the exam right after the tutor's questions ([3.12](12-architecture-defense.md)),
and it is what the user opens from a challenge's **History** to read a past attempt.

Only finished attempts exist ([3.12](12-architecture-defense.md) decision 11), so the history
lists complete attempts and never partial ones.

## Decisions

1. **One view, two entry points.** The component that ends the exam and the one opened from the
   history are the same, so the user sees the same thing right after finishing and weeks later.
   It replaces the temporary ending of [3.12](12-architecture-defense.md) decision 12.
2. **History is a button in the catalog.** A challenge with at least one finished attempt shows
   **History** on its row ([3.7](07-challenge-catalog-ui.md)). It opens a list of its attempts,
   newest first: date, diagram score, understanding score and outcome (Passed or Failed). Choosing
   one opens the result view. There is no separate history screen and no challenge page.
3. **Read-only, always.** A finished attempt has no edit, resubmit, re-evaluate or delete action.
   The history is permanent: nothing removes a finished attempt except an explicit future decision,
   and deleting a study session never does ([3.3](03-attempts-and-session-link.md)).
4. **What the result view shows,** in this order:
   - the header: challenge title, topic, difficulty, attempt date, time spent, whether it ended by
     the user or by time ([3.3](03-attempts-and-session-link.md) `SubmitReason`) and the hints
     used out of 3;
   - the outcome, Passed or Failed, with both scores and the minimum passing score frozen for that
     attempt ([3.6](06-completion-rules.md)), and the list of reasons when failed;
   - the diagram evaluation: summary, then one row per criterion with its name, weight, level
     (0 to 4), the justification and, when present, the evidence; a criterion that was not judged
     shows **Not judged from the diagram**;
   - the understanding evaluation, the same way, with **Not asked** for criteria the tutor did not
     reach;
   - the strengths, gaps and recommendations of each evaluation;
   - the tutor's questions and the user's answers, in order;
   - the submitted diagram, read-only.
5. **The statement is visible here.** The exam is over, so the scenario and the four lists are
   shown above the result. This is the only place outside the exam where they appear; the catalog
   still never shows them. It is the statement the user answered: a challenge's content is
   immutable after creation ([3.1](01-challenge-domain-and-persistence.md)), so a later read never
   shows a different one.
6. **The diagram opens in the same editor, locked.** The stored XML is shown with the draw.io embed
   of [3.9](09-diagram-editor-spike.md) in read-only mode, with no menus and no edit. It needs the
   internet like the exam. Without a connection the area says "The diagram needs an internet
   connection to display." and the rest of the page still works. The read-only configuration is
   confirmed by the spike.
7. **No comparison view and no trend.** Attempts are opened one at a time. The catalog already
   shows the count and the best diagram score.
8. **Nothing is computed on the screen.** The frontend renders what the backend returns; the
   outcome and reasons come from [3.6](06-completion-rules.md), never derived in the UI.
9. **No data compatibility to keep.**

## Backend

Read models in `internal/application/exercise`; both are read-only and return only finished
attempts (`ErrAttemptNotFinished` otherwise, `ErrAttemptNotFound` for an unknown id).

```go
type AttemptListItem struct {
    AttemptID          string
    FinishedAt         time.Time
    DiagramScore       int
    UnderstandingScore int
    Outcome            AttemptResult // PASSED or FAILED
}

type AttemptResultView struct {
    Challenge   ChallengeSnapshot   // title, topic, difficulty, scenario and the four lists
    Attempt     AttemptView         // dates, time spent, SubmitReason, hints used
    PassScore   int                 // frozen for the attempt
    Outcome     AttemptOutcome      // result and reasons
    Diagram     EvaluationView      // evaluation joined with criterion names and weights
    Understanding EvaluationView
    Defense     []DefenseTurn
    Submission  string              // the stored XML
}

func (s *HistoryService) ListHistory(ctx, challengeID string) ([]AttemptListItem, error)
func (s *HistoryService) GetAttemptResult(ctx, attemptID string) (AttemptResultView, error)
```

`HistoryService` composes the repositories of [3.3](03-attempts-and-session-link.md) to
[3.5](05-evaluation-engine.md), [3.12](12-architecture-defense.md) and the outcome of
[3.6](06-completion-rules.md); it adds no rule. `EvaluationView` joins each criterion result with
the frozen rubric's name and weight, so the screen needs no second lookup.

Bindings in `internal/interfaces/desktop/exercise.go`: `ListAttemptHistory(challengeId)` and
`GetAttemptResult(attemptId)`, thin. `wailsjs` regenerated.

## Frontend

- `components/lab/attempt-result-view.tsx`: the view of decision 4, built from small sections
  (`result-header`, `outcome-card`, `evaluation-table`, `defense-transcript`, `result-diagram`).
- `components/attempt-history-dialog.tsx`: the list of decision 2.
- `challenge-row.tsx`: the **History** button, shown when `Attempts` > 0.
- `diagram-editor.tsx` ([3.10](10-diagram-workspace.md)) gains a `readOnly` mode for
  `result-diagram`, and the offline message of decision 6.
- `LabExamScreen` ends on `attempt-result-view` with **Back to catalog**, replacing `exam-outcome`.
- `lib/exercise.ts`: wrappers and DTO types.
- All UI text is in English.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks, Vitest for the frontend), keeps the
build green and is committed on its own.

- [ ] Backend: `ListHistory` (only finished attempts, newest first, scores and outcome)
- [ ] Backend: `GetAttemptResult` (joins, frozen pass score, outcome and reasons, the not-judged
      and not-asked cases, `ErrAttemptNotFinished`)
- [ ] Bindings, `wailsjs` and mocks regenerated
- [ ] Frontend: `evaluation-table`, `outcome-card`, `defense-transcript`, `result-header`
- [ ] Frontend: `diagram-editor` read-only mode and the offline message
- [ ] Frontend: `attempt-result-view` and the exam ending in `LabExamScreen`
- [ ] Frontend: `attempt-history-dialog` and the **History** button
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts` describe the result and history

## Acceptance Criteria

- Finishing an exam shows the result view with the outcome, both scores, the minimum score used,
  and the reasons when failed; **Back to catalog** leaves it.
- **History** appears on a challenge with finished attempts, lists them newest first with date,
  both scores and outcome, and each one opens the same view.
- The view shows, per criterion, the weight, the level, the justification and the evidence;
  criteria not judged from the diagram or not asked in the defense are labeled as such and not
  shown as zero.
- It shows the tutor's questions and the user's answers in order, the hints used, and whether the
  exam ended by the user or by time.
- The scenario and the four lists appear in the result view and never in the catalog.
- The diagram displays locked, with no way to edit it; with no connection the diagram area shows
  the offline message and the rest of the page still renders.
- No action in the history or the view changes or deletes an attempt.
- Asking for an attempt that is not finished fails with `ErrAttemptNotFinished`.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- Deleting, exporting, renaming or re-evaluating attempts, and comparing or charting attempts.
- A challenge page and a global history across challenges.
- Sending the result to the study chat ([3.16](16-session-feedback.md)) and the session's
  exercises list ([3.17](17-session-exercises-ui.md)).
- Editing the stored diagram.
