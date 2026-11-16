# Phase 3.23 — Interactive Quiz: Offer in the Study Chat

## Goal

Let the user ask for a quiz from inside the study session, the same way they ask for an exercise
([3.14](14-ai-challenge-generation.md)): the tutor answers with an **offer**, the user accepts or
declines, and an accepted offer becomes a persistent **quiz card** in the conversation
([3.17](17-session-exercises-ui.md)) that starts the quiz and returns to the session
([3.15](15-open-lab-from-chat.md)). The core is [3.21](21-interactive-quiz-core.md) and the screens
are [3.22](22-interactive-quiz-screen.md); every binding and screen of the quiz already exists.

This is the riskiest increment of the quiz because it touches the study chat, which the exercise
offer already uses. Two rules follow from that: the exercise offer behaves exactly as today, and
no lab or quiz code leaks into the `study` package.

## Decisions

1. **Only the user asks.** The study prompt gets a second marker, `[[ATHENA_QUIZ_OFFER]]`, with the
   same rule as the exercise one: the tutor emits it, on its own line after one or two short
   sentences, if and only if the user explicitly asks for a quiz or a test about the session. It
   never proposes one on its own. A missed request is a normal reply and the user can ask again or
   use the menu. A marker the user did not cause does no harm: the offer needs an explicit
   **Accept**.
2. **Two markers, one at a time.** The marker filter and the stripping handle both markers: the
   user never sees either, not even a fragment while streaming, and the stored text is clean. If a
   reply carries both, the exercise offer wins and the quiz marker is dropped, so existing behavior
   is preserved and a session never has two pending offers.
3. **The quiz offer has its own table, `quiz_offers`.** This **amends decision 4 of
   [ADR-007](../../decisions/ADR-007-interactive-quiz.md)**, which planned a `kind` column on
   `exercise_offers`. A `kind` there would put a quiz concept inside the exercise context, which is
   what the ADR forbids everywhere else. With a separate table the exercise offer code and data
   stay untouched. The columns are the same as `exercise_offers` (`id`, `session_id`, `message_id`,
   `created_at`), `UNIQUE (session_id)`, both foreign keys with `ON DELETE CASCADE`. The ADR is
   updated in this change.
4. **One pending offer per session, across both kinds.** The chat sees one `OfferGate`
   ([`study/offers.go`](../../../internal/application/study/offers.go)). Its `CreateOffer` gains
   the offer kind, and its `HasPending` is true when either table has a row. The gate is composed
   in `main.go` from the exercise offer service and the new quiz offer service, so `study` imports
   neither. Creating an offer while one is pending keeps today's behavior: logged, best effort, the
   reply stays a normal message.
5. **The composer stays blocked while any offer is pending.** The backend refuses with
   `ErrOfferPending` and the input is disabled, as in 3.14 decision 6. The text becomes
   kind-neutral: "Accept or decline the offer before sending another message."
6. **The quiz offer card** (`components/quiz/quiz-offer-card.tsx`) hangs under the assistant
   message that carries the offer, like the exercise one. It offers the number of questions (3 to
   20, default 10) and the difficulty (preselected from the profile, as in 3.22 decision 4), plus
   **Accept** and **Decline**. Its text uses the `quiz` namespace, so it follows the profile
   language like the rest of the quiz screens. Accepting shows progress and locks both buttons and
   both fields. A failure leaves the offer pending with the translated `QuizError` message, and
   **Accept** and **Decline** work again. There is no automatic retry.
7. **Accepting generates and links.** `QuizOfferService.Accept(offerID, spec)` takes a per-session
   lock, generates the quiz with the existing `GenerationService` for the offer's session, stores
   the offer's `MessageID` on the quiz and deletes the offer. Generation runs with no transaction
   open. A quiz is never reused: each accept writes a new one, because the user chose the size and
   difficulty on the card.
8. **Accept is idempotent.** Before generating, `Accept` looks for a quiz already linked to the
   offer's message. If one exists (the app closed after generating but before the offer was
   deleted, or a second accept lost the race) it deletes the offer and returns that quiz, so no
   duplicate appears. This replaces a transaction, since the generator saves the quiz itself.
9. **Declining deletes the offer** and nothing else; the tutor's text stays. Declining an offer
   that is already gone is not an error.
10. **Accepting shares the quiz generation slot.** `AcceptQuizOffer` uses the same
    `generationSlot` as `GenerateQuiz`, so only one quiz is generated at a time and
    `CancelQuizGeneration` cancels either.
11. **`quizzes.message_id`** is a new nullable column, set only for quizzes born from an offer; a
    quiz generated from the menu has none. It has **no foreign key**, because a cascade from the
    message would delete the quiz with its session and break the decoupled lifecycle of 3.21
    decision 8. A dangling id is harmless: the card is built from the quizzes of one open session.
12. **The quiz card** (`components/quiz/session-quiz-card.tsx`) replaces the offer card after
    accept and stays under the same message every time the session opens. It is a view of the
    quiz, not a chat message, so it never reaches the model and never blocks sending. It shows the
    quiz title (the session title), difficulty, number of questions and, when there are attempts,
    the attempt count and the latest grade out of 10 ("Not taken yet" otherwise). Buttons: **Take
    quiz** (**Take again** once it has attempts) and **History** (only when it has attempts). It
    never shows questions or answers.
13. **Taking it from the chat returns to the chat.** `AppShell` holds
    `quizReturn: { kind: 'menu' } | { kind: 'study', sessionId: string }` next to `takingQuizId`,
    set by the entry point, applied and cleared on every exit, and not persisted. This mirrors
    `labReturn` ([3.15](15-open-lab-from-chat.md) decision 4):
    - the result's final button reads **Back to study session** instead of **Back to tests**;
    - **Leave** after the confirmation of 3.22 decision 7 returns to the session too.

    A quiz started from the menu keeps returning to the menu. Returning reopens the same session
    with its persisted messages, as in 3.15 decision 5; the session list and the selected session
    are preserved.
14. **The card refreshes by itself**: when the session opens, after **Accept** and after the user
    returns from a quiz, so a new grade shows without any manual action.
15. **Deleting a quiz from the menu removes its card** at the next load of the session. Deleting
    the session leaves the quiz in the menu, as 3.21 decision 8 says; its card goes with the chat.
16. **Nothing about generation quality changes here.** A session with neither summary nor notes
    still generates from the topic alone, as in 3.21. That product question is out of this spec.
17. **No data compatibility to keep.**

## Changes to earlier specs

- **ADR-007, decision 4:** `quiz_offers` replaces the `kind` column on `exercise_offers`.
- **3.14, decision 6:** the blocked-composer text becomes kind-neutral, and `ErrOfferPending`
  covers both offer kinds.
- **3.21:** `Quiz` gains an optional `MessageID`, and `Repository` gains the lookups the card
  needs (below).

## Backend

### Study package (`internal/application/study`)

- `offer_marker.go` generalizes to a list of markers. `stripOfferMarkers(content)` removes every
  known marker and returns the text and the offer it found (`none`, `exercise` or `quiz`, the
  exercise winning when both are present). `offerMarkerFilter` holds back a trailing fragment that
  is a proper prefix of **any** marker, and flushes it when the stream ends. The existing
  exercise-only tests stay green and still pin the behavior.
- `offers.go`: `OfferKind` (`OfferKindExercise`, `OfferKindQuiz`), `OfferGate.CreateOffer(ctx,
  sessionID, messageID, kind)` and the unchanged `HasPending`.
- The study prompt appends the quiz instruction next to the exercise one, as a pure string pinned
  by a test, like `exerciseOfferInstruction`.

### Quiz package (`internal/domain/quiz`, `internal/application/quiz`)

- Domain: `Offer{ID, SessionID, MessageID, CreatedAt}`, `OfferRepository` (`Create`, `Get`,
  `GetBySession`, `Delete`) and `ErrOfferNotFound`, `ErrOfferPending`. `Quiz.MessageID` optional.
  `Repository.FindByMessage(ctx, messageID) (Quiz, error)` and
  `ListBySession(ctx, sessionID) ([]Summary, error)`, where `Summary` carries the attempt count
  and the latest score already used by the menu list.
- `application/quiz.OfferService`: `CreateOffer`, `GetPending`, `Accept`, `Decline`, with the
  per-session lock pattern of `exercise.OfferService`, copied in shape and not imported. It
  depends on the generator through a consumer-side port, `QuizGenerator.GenerateForMessage(ctx,
  sessionID, messageID, spec)`, which `GenerationService` implements by sharing its internals with
  `Generate`.
- `Service.ListSessionQuizzes(ctx, sessionID)` returns the cards of decision 12 in message order,
  with no questions.

### Persistence (`internal/infrastructure/sqlite`)

- Migration appended to the slice: table `quiz_offers`, and `ALTER TABLE quizzes ADD COLUMN
  message_id TEXT` guarded like the existing `addXColumn` migrations. `reset.go` and
  `reset_test.go` list `quiz_offers` before its parents.
- `quiz_offer_repository.go`, and `FindByMessage` and `ListBySession` on the quiz repository,
  tested against a real SQLite.

### Bindings (`internal/interfaces/desktop/quiz.go`)

`GetPendingQuizOffer(sessionID)`, `AcceptQuizOffer(offerID, count, difficulty)`,
`DeclineQuizOffer(offerID)` and `ListSessionQuizzes(sessionID)`, thin, with errors mapped to
`QuizError` (new code `offer_not_found`). `wailsjs` is regenerated.

### Composition (`main.go`)

A small adapter implements `study.OfferGate` over both offer services: `HasPending` is the OR of
the two, `CreateOffer` routes by kind and, before creating, refuses when the other kind is pending,
so the one-offer rule holds across tables.

## Frontend

- `StudyChatScreen` keeps one `offer` state with a `kind`. `fetchPendingOffer` asks for the
  exercise offer and then the quiz one, and renders `ExerciseOfferCard` or `QuizOfferCard` under
  the matching message. Quiz cards load with the session and reload per decision 14.
- `lib/quiz.ts`: the four wrappers and the card DTO. `offerPendingMessage()` is neutral.
- `app-shell.tsx`: `quizReturn`, applied by `handleQuizExit`; the chat gets an `onStartQuiz` prop,
  as it has `onStartExam`.
- `quiz-result-view` and `QuizTakeScreen` take the return target and show the right label.
- `quiz-en.ts` and `quiz-pt.ts` get the new keys, with the parity test.

## Tests (TDD)

- **Marker, pure:** both markers alone, together (exercise wins), a marker split across chunks, a
  fragment of one marker that turns out to be text, a fragment of each marker held back at the end
  of the stream and flushed.
- **Study:** the quiz marker creates a quiz offer with the right message id and kind; a reply to a
  non-user turn creates none; a creation failure leaves a normal reply; the prompt contains the
  instruction; the composer is refused while either kind is pending.
- **Quiz offer service:** create, pending, decline (and decline of a missing one), accept
  (generates once, stores `MessageID`, deletes the offer), accept after a lost race returns the
  existing quiz without a second generation, a failed generation leaves the offer, serialization
  per session.
- **Repositories:** real SQLite, including the `UNIQUE (session_id)` conflict, the cascade with the
  session and with the message, `FindByMessage`, `ListBySession`, and a quiz that survives its
  session.
- **Gate adapter:** `HasPending` for each combination, routing by kind, refusal when the other
  kind is pending.
- **Bindings:** thin mapping and error codes.
- **Frontend:** the offer card (accept success, each error, locked fields while busy, decline),
  the quiz card (with and without attempts, buttons), the chat rendering each kind under its
  message and blocking the composer, `AppShell` returning to the session, and the result label.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Marker generalization and the quiz instruction in the study prompt (pure, no wiring)
- [ ] `OfferKind` on the study gate, and the kind-neutral composer text
- [ ] Domain: `Offer`, `OfferRepository`, `Quiz.MessageID`, `FindByMessage`, `ListBySession`
- [ ] SQLite: `quiz_offers`, `message_id` column, repositories, reset
- [ ] `quiz.OfferService`, `GenerateForMessage` and `ListSessionQuizzes`
- [ ] Bindings, `wailsjs` and mocks regenerated; composition adapter in `main.go`
- [ ] Frontend: `quiz-offer-card`, `session-quiz-card`, chat integration
- [ ] Frontend: `quizReturn`, the labels and the return flow
- [ ] Docs: ADR-007 amendment, CHANGELOG, README and `lib/documentation.ts`

## Acceptance Criteria

- Asking the tutor for a quiz shows the offer card under its reply, with no trace of the marker
  while streaming, after the reply and after reopening the session.
- Asking for an exercise behaves exactly as before, and a reply with both markers produces the
  exercise offer only.
- While either offer is pending the input is disabled, also after restarting the app.
- **Accept** generates a quiz of the chosen size and difficulty, replaces the card with the quiz
  card under the same message and keeps it across reopening the session.
- A failed generation leaves the offer pending with the error, and **Accept** works again.
- Closing the app after generating but before the offer is removed, then accepting again, shows one
  quiz card and one stored quiz.
- **Decline** removes the card and creates nothing.
- **Take quiz** from the card opens the quiz; finishing it shows **Back to study session**, and
  returning shows the new grade on the card. **Leave** also returns to the session. From the menu
  the quiz still returns to the menu.
- A quiz from the offer appears in the menu list like any other, and deleting it there removes its
  card.
- Deleting the session removes the offer and the card but keeps the quiz in the menu.
- The card never shows questions or answers and is never sent to the model.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  `internal/domain` and `internal/application` code, lint and vulnerability checks, and the
  frontend tests, coverage, lint, typecheck, `npx prettier --check .` and Stryker on the changed
  files pass.

## Out of scope

- A tutor that proposes a quiz on its own, or offers one after an exercise.
- Quiz cards outside the study chat, and removing a card without deleting the quiz.
- Blocking or warning when a session has no summary or notes (3.21 open question).
- Resuming an unfinished quiz, a timer, retaking only missed questions, and sharing a quiz.
