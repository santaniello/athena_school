# Phase 3.15 — Open the Lab from the Study Chat

## Goal

After the user accepts an exercise offer ([3.14](14-ai-challenge-generation.md)), let them start
the exam from the chat and come back to the same study session when it ends. The exam is the one of
[3.10](10-diagram-workspace.md); this spec adds the way in, the link to the session and the way
back.

## Decisions

1. **The way in is a button on the chat card.** After **Accept**, the offer card of
   [3.14](14-ai-challenge-generation.md) shows the challenge title and **Open in Architecture
   Lab**. Nothing opens by itself: the user decides when to start.
2. **Same window.** The lab opens in the main window, like any other section, never in a second
   window.
3. **The usual start flow runs.** The button opens the rules dialog, the editor check and
   `StartAttempt` of [3.10](10-diagram-workspace.md), with the same in-progress warning. The only
   difference is that the attempt is started with the session: `StartAttempt` receives the
   `SessionID` and the attempt is linked to the session's `SessionExercise`
   ([3.3](03-attempts-and-session-link.md), [3.4](04-exercise-service.md)). Refusing a session
   that does not exist or is not linked to that challenge is the service's rule, not the screen's.
4. **The way back is the session.** For an exam started from a session, the screen remembers
   where it came from and returns there:
   - the final button of the result ([3.13](13-evaluation-and-history-screen.md)) reads **Back to
     study session** instead of **Back to catalog**;
   - **Leave** and the other exits that discard the exam ([3.10](10-diagram-workspace.md)) return
     to the session too, after the discard.

   An exam started from the catalog keeps returning to the catalog.
5. **The chat is the same conversation.** Returning reopens the same session with its persisted
   messages, summary and sources. An unsent text in the input is not kept: the chat screen
   unmounts while the lab is open, as with any other section. The session list and the selected
   session in `AppShell` are preserved.
6. **The offer card keeps working.** After coming back, the card shows the challenge and
   **Open in Architecture Lab** again, so the user can start another attempt of the same
   challenge from the chat. How finished attempts show on the card is
   [3.17](17-session-exercises-ui.md).
7. **No new backend rules.** Everything needed exists in [3.4](04-exercise-service.md) and
   [3.14](14-ai-challenge-generation.md). The only addition is that `AppShell` carries a "return
   to" target while an exam runs.
8. **No data compatibility to keep.**

## Frontend

- `AppShell` holds `labReturn: { kind: 'catalog' } | { kind: 'study', sessionId: string }` next to
  `section` and `activeSession`. Starting an exam sets it from the entry point; leaving the lab
  by any exit applies it and clears it. It is not persisted.
- `exercise-offer-card.tsx` ([3.14](14-ai-challenge-generation.md)): the **Open in Architecture
  Lab** button after accept, calling the start flow with the session id.
- `LabExamScreen` and `attempt-result-view` take the return target as a prop and show the right
  label ([3.10](10-diagram-workspace.md), [3.13](13-evaluation-and-history-screen.md)).
- The navigation guard of [3.10](10-diagram-workspace.md) decision 12 is unchanged.
- `lib/exercise.ts`: `startAttempt` already accepts a session id; no new wrapper.
- All UI text is in English.

No backend and no binding in this spec.

## Tasks

Each slice is TDD (Vitest), keeps the build green and is committed on its own.

- [ ] `AppShell`: `labReturn` set from the catalog and from a session, applied and cleared on exit
- [ ] Offer card: **Open in Architecture Lab** starting the exam with the session id
- [ ] `LabExamScreen` and the result view: the return target and the label
- [ ] Leave and discard exits return to the origin
- [ ] Docs: CHANGELOG, README and `lib/documentation.ts`

## Acceptance Criteria

- After **Accept**, the card shows the challenge title and **Open in Architecture Lab**; the lab
  does not open by itself.
- The button opens the rules dialog; confirming starts an attempt linked to that session and
  shows the exam in the same window.
- Finishing the exam shows **Back to study session**, and it opens the same session with its
  messages as they were.
- **Leave** and a discard after a failure return to the session; an exam started from the
  catalog still returns to the catalog.
- An unfinished exam elsewhere triggers the same discard warning as in
  [3.10](10-diagram-workspace.md).
- Starting from a session that does not exist or is not linked to the challenge is refused by the
  backend and the screen shows the error.
- The card still shows **Open in Architecture Lab** after coming back.
- Frontend tests, coverage ≥ 80%, lint, typecheck, `npx prettier --check .` and Stryker on the
  changed files pass.

## Out of scope

- Opening the lab in a second window, and keeping an unsent chat draft.
- Sending the result back into the conversation ([3.16](16-session-feedback.md)) and the
  persistent card and Exercises tab ([3.17](17-session-exercises-ui.md)).
- Starting the exam automatically on accept, and any change to the exam itself.
