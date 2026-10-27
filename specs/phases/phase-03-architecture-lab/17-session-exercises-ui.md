# Phase 3.17 — Exercise Card in the Study Chat

## Goal

Keep the accepted exercise visible in the conversation. After the user accepts an offer
([3.14](14-ai-challenge-generation.md)), the offer card becomes a persistent card in the chat that
shows the challenge, how the user is doing on it, and the buttons to start it or read its history.
It is the last piece that ties the study session to the Architecture Lab.

## Decisions

1. **The card lives in the conversation.** It is shown under the assistant message that carried
   the offer and stays there every time the session is opened. It is a view of the session's
   `SessionExercise` ([3.3](03-attempts-and-session-link.md)), not a chat message of its own, so
   it never reaches the model and does not affect sending.
2. **What the card shows:** the challenge title, its difficulty and time limit, and the progress
   label from [3.6](06-completion-rules.md): `Not started`, `Needs improvement` or `Completed`.
   When there are finished attempts it also shows their count and the latest result: Passed or
   Failed with the diagram and understanding scores. It never shows the scenario, requirements or
   rubric, as in the catalog ([3.7](07-challenge-catalog-ui.md)).
3. **Two buttons.** **Open in Architecture Lab** starts the exam through the flow of
   [3.15](15-open-lab-from-chat.md) and reads **Start again** once the challenge has a finished
   attempt. **History** appears when there is at least one finished attempt and opens the list of
   [3.13](13-evaluation-and-history-screen.md).
4. **One exercise per session in practice.** Accepting always reuses the session's latest challenge
   ([3.14](14-ai-challenge-generation.md) decision 8), so a session shows one card. The data model
   allows several; the screen renders every card it receives, each under its own message. There is
   no Exercises tab or list: with one card per session it would only repeat the chat.
5. **It refreshes by itself.** The card is reloaded after **Accept**, after the user returns from
   the lab ([3.15](15-open-lab-from-chat.md)) and whenever the session is opened, so a finished
   exam shows its result on return without any manual action.
6. **Deleting the session removes the card,** with its link and offers, and keeps the challenge
   and its attempts ([3.3](03-attempts-and-session-link.md)).
7. **No data compatibility to keep.**

## Changes to earlier specs

- **3.3:** `SessionExercise` gains `MessageID`, the assistant message the card hangs under, with
  `message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE`.
- **3.14:** `Accept` creates the `SessionExercise` with the offer's `MessageID`, and the card shown
  right after accept is the one defined here, replacing "Exercise ready: <title>".

## Application

`internal/application/exercise.SessionExerciseService`:

```go
type SessionExerciseCard struct {
    SessionExerciseID string
    MessageID         string
    ChallengeID       string
    Title             string
    Difficulty        Difficulty
    TimeLimitMinutes  int
    Status            ProgressStatus
    Attempts          int
    Latest            *LatestResult // nil when no finished attempt
}

type LatestResult struct {
    Outcome            AttemptResult // PASSED or FAILED
    DiagramScore       int
    UnderstandingScore int
}

func (s *SessionExerciseService) ListSessionExercises(ctx, sessionID string) ([]SessionExerciseCard, error)
```

It lists the session's links, reads each challenge, derives progress with `DeriveProgress`
([3.6](06-completion-rules.md)) from its finished attempts and takes the latest finished one for
`Latest`. The attempts counted are all finished attempts of the challenge, not only those started
from this session, so the card agrees with the catalog. Bulk loading follows the catalog's fixed
number of queries ([3.7](07-challenge-catalog-ui.md)). It is read-only and returns no scenario.

Binding in `internal/interfaces/desktop/exercise.go`: `ListSessionExercises(sessionId)`, thin.
`wailsjs` regenerated.

## Frontend

- `components/session-exercise-card.tsx`: the card of decisions 2 and 3, rendered in the study
  chat under the message with the matching `MessageID`. It takes the start and history handlers as
  props.
- `StudyChatScreen` loads the cards with the session's messages and reloads them as in decision 5.
- `exercise-offer-card.tsx` ([3.14](14-ai-challenge-generation.md)) hands over to this card once
  the accept succeeds.
- `lib/exercise.ts`: the wrapper and the DTO types.
- All UI text is in English.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks, Vitest for the frontend), keeps the
build green and is committed on its own.

- [ ] `SessionExercise.MessageID`: domain, SQLite column with the cascade, `Accept` passing it
- [ ] `ListSessionExercises`: status and counts from finished attempts, `Latest`, no scenario,
      bulk loading
- [ ] Binding, `wailsjs` and mocks regenerated
- [ ] Frontend: `session-exercise-card` (labels, scores, button states, never the scenario)
- [ ] Frontend: cards under their messages, reload after accept and after returning from the lab
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts`

## Acceptance Criteria

- After **Accept**, the offer card is replaced by the exercise card with the title, difficulty,
  time limit and `Not started`, under the same assistant message.
- Reopening the session or restarting the app shows the same card in the same place.
- With no finished attempt the card has **Open in Architecture Lab** and no **History**; with one
  or more it reads **Start again**, shows the attempt count and the latest outcome with both
  scores, and shows **History**.
- After finishing an exam from the card and returning to the session, the card shows the new
  result without any manual refresh.
- The progress label matches the catalog for the same challenge.
- The card never shows the scenario, requirements or rubric, and is never sent to the model.
- Deleting the session removes the card and keeps the challenge and its attempts.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- An Exercises tab or a list across sessions, and removing or hiding a card.
- Generating a different challenge from the card, and any edit of the challenge.
- The result detail ([3.13](13-evaluation-and-history-screen.md)) and the tutor's use of the
  result ([3.16](16-session-feedback.md)).
