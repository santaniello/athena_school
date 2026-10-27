# Phase 3.14 — Exercise Offer in the Study Chat

## Goal

Let the user ask the tutor for an architecture exercise while studying. The tutor answers with an
**offer** that the user accepts or declines. Accepting links a challenge to the session, generated
now or reused, ready to open ([3.15](15-open-lab-from-chat.md)).

Generation itself is [3.6.1](06-01-challenge-generation.md); this spec adds the chat entry point
and avoids creating duplicates. The catalog route of 3.6.1 and 3.7 stays as it is.

## Decisions

1. **Only the user asks.** The tutor never proposes an exercise on its own. An offer appears only
   after the user asks to practice, in their own words ("I want to practice this", "give me an
   exercise").
2. **The model recognizes the request.** The study prompt gains an instruction: when the user
   explicitly asks for an exercise on the topic, answer briefly and end the reply with a fixed
   marker. The backend reads the marker, removes it from the stored and displayed text, and creates
   the offer. Recognition depends on the model and can miss; a missed request is a normal reply,
   and the user can ask again or use the catalog. A marker the user did not cause cannot do harm: an
   offer needs an explicit **Accept**.
3. **The offer is a persisted pending item of the chat.** It belongs to the assistant message that
   carries it and survives closing the app: on reopening the session it is still pending. One
   pending offer per session at most.
4. **Accept and Decline, in the assistant language.** The offer shows a short title and two
   buttons: **Accept** and **Decline** when the assistant language is English, **Aceitar** and
   **Recusar** when it is Portuguese ([1.4](../phase-01-desktop-mvp/04-onboarding.md)). The rest of
   the interface stays in English.
5. **Declining removes the offer.** The offer card disappears from the chat and its record is
   deleted. The tutor's reply text stays as a normal message. Nothing is created.
6. **The user cannot send messages while an offer is pending.** The backend refuses a message with
   `ErrOfferPending` and the input is disabled with "Accept or decline the exercise offer to keep
   chatting." Resolving the offer, either way, re-enables it. This also holds after reopening the
   app.
7. **Accepting creates the link, not the attempt.** Accept makes the challenge available for the
   session and creates the `SessionExercise` ([3.3](03-attempts-and-session-link.md)). It does
   not start the exam: opening the lab is [3.15](15-open-lab-from-chat.md). The link keeps the
   offer's message id, so the card of [3.17](17-session-exercises-ui.md) stays under that message.
8. **Reuse before generating.** On accept, if a challenge already exists whose source session is
   this one (from the chat or from the catalog), the most recent one is reused and no AI call is
   made. Otherwise a challenge is generated through
   [3.6.1](06-01-challenge-generation.md) for this session, with the difficulty from the profile's
   experience level and no question to the user. To get another challenge for the same session the
   user uses **Generate challenge with AI** in the catalog.
9. **Failure leaves the offer pending.** If generation fails (provider error, invalid output), the
   offer stays pending with the error shown and **Accept** available again, and **Decline** still
   works. Nothing is created and there is no automatic retry. While generating, the card shows
   progress and both buttons are disabled.
10. **Closing during generation.** The offer is still pending when the session reopens and the
    user accepts again. A challenge that finished generating before the app closed is found by the
    reuse rule of decision 8, so no duplicate appears.
11. **Same model and cost rules** as the rest of the lab: one generation call at most per accept,
    with the Architecture Lab model ([3.6.1](06-01-challenge-generation.md) decision 10).
12. **No data compatibility to keep.**

## Domain and application

`internal/application/exercise.OfferService`, next to the other services; the chat package stays
free of lab code and talks to it through a narrow port.

```go
type Offer struct {
    ID        string
    SessionID string
    MessageID string // the assistant message that carries it
    CreatedAt time.Time
}

func (s *OfferService) CreateOffer(ctx, sessionID, messageID string) (Offer, error)
func (s *OfferService) GetPending(ctx, sessionID string) (Offer, bool, error)
func (s *OfferService) Accept(ctx, offerID string) (SessionExerciseView, error)
func (s *OfferService) Decline(ctx, offerID string) error
```

- `CreateOffer` refuses a second pending offer for the session (`ErrOfferPending`).
- `Accept` takes an in-process lock per session (the desktop app is one process) before anything
  else, so two concurrent accepts cannot both generate. Inside it: load the offer
  (`ErrOfferNotFound`; a second call finds it already resolved and returns the existing link);
  look for a challenge by source session
  (`ChallengeRepository.FindLatestBySourceSession`, added here); if none, call
  `GenerationService.GenerateChallenge` with the session and the profile's difficulty, with no
  transaction open; then, in one transaction, create the `SessionExercise` (an existing link for
  the same session and challenge is accepted as is) and delete the offer. `SessionExerciseView`
  carries the challenge id, title, topic and difficulty.
- `Decline` deletes the offer.

In `internal/application/study`:

- the study prompt builder (`prompt.go`) adds the marker instruction, a pure function tested for
  the instruction and the marker text ([1.6](../phase-01-desktop-mvp/06-study-mode.md));
- a `OfferDetector` port, implemented by the lab, is called after the assistant message is stored:
  it strips the marker from the text and asks for `CreateOffer`; a failure to create the offer
  never fails the reply;
- `SendMessage` checks `GetPending` through a port and returns `ErrOfferPending` before anything
  is stored or sent to the model;
- while streaming, a trailing partial marker is held back so the user never sees it.

## SQLite

```sql
CREATE TABLE exercise_offers (
  id         TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
  message_id TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  created_at TEXT NOT NULL,
  UNIQUE (session_id)
);
```

`UNIQUE (session_id)` holds the one-pending-offer rule at the database level. A resolved offer is
deleted, so every stored row is pending.

## Desktop bindings and frontend

- Bindings in `internal/interfaces/desktop/exercise.go`: `GetPendingOffer(sessionId)`,
  `AcceptOffer(offerId)`, `DeclineOffer(offerId)`, thin, an error code per sentinel. The
  existing send-message binding maps `ErrOfferPending`. `wailsjs` regenerated.
- `components/exercise-offer-card.tsx`: the card under the assistant message, with the localized
  buttons, the progress state and the error with the retry.
- The chat input is disabled while an offer is pending ([1.6](../phase-01-desktop-mvp/06-study-mode.md)),
  with the message of decision 6.
- What happens after **Accept** is the card of [3.17](17-session-exercises-ui.md) and the opening
  of the lab in [3.15](15-open-lab-from-chat.md); until 3.17 exists the card shows "Exercise ready: <title>"
  and nothing else.
- `lib/exercise.ts` wrappers and the localized labels.

## Tasks

Each slice is TDD (Go tests with testify and Mockery mocks, Vitest for the frontend), keeps the
build green and is committed on its own.

- [ ] Study prompt: the marker instruction and the marker parser (strip, detect, partial marker)
- [ ] SQLite: `exercise_offers`, `OfferRepository`, `FindLatestBySourceSession`
- [ ] Application: `CreateOffer`, `GetPending`, `Decline`
- [ ] Application: `Accept` (reuse, generate, both create the link and delete the offer, failure
      keeps the offer, existing link accepted)
- [ ] Study integration: `OfferDetector` after the assistant message, `ErrOfferPending` before
      sending, streaming without a visible marker
- [ ] Bindings, `wailsjs` and mocks regenerated
- [ ] Frontend: `exercise-offer-card` (language, progress, error and retry), input disabled while
      pending, reopening a session with a pending offer
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts`

## Acceptance Criteria

- Asking for an exercise leads the tutor's reply to carry an offer card with **Accept** and
  **Decline** (**Aceitar** and **Recusar** for a Portuguese assistant); the marker text is never
  visible, not while streaming and not after.
- With no request from the user, no offer appears.
- While an offer is pending, sending a message fails with `ErrOfferPending`, nothing is stored or
  sent to the model, and the input is disabled; this still holds after reopening the app.
- **Decline** removes the card and its record, keeps the tutor's text, and re-enables the input.
- **Accept** with no earlier challenge for the session generates one with the profile's
  difficulty and links it; with an earlier one it reuses the latest and makes no AI call. Either
  way the offer is deleted and the input is enabled.
- A generation failure leaves the offer pending with the error and **Accept** available again;
  nothing is created.
- Accepting twice for the same session creates a single challenge and a single link.
- A second pending offer for a session is refused.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks, and the frontend tests, coverage, lint,
  typecheck, `npx prettier --check .` and Stryker on the changed files pass.

## Out of scope

- The tutor proposing an exercise by itself, a "Practice this topic" button and a typed command.
- Choosing the difficulty in the offer, and generating more than one challenge per session from
  the chat.
- Opening the lab and returning to the chat ([3.15](15-open-lab-from-chat.md)), the persistent
  exercise card and the Exercises tab ([3.17](17-session-exercises-ui.md)), and the result flowing
  back into the chat ([3.16](16-session-feedback.md)).
- Guaranteeing that the model recognizes every request.
