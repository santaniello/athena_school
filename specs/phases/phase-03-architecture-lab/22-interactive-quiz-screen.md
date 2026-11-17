# Phase 3.22 — Interactive Quiz: Menu Section

## Goal

Give the Interactive Quiz ([3.21](21-interactive-quiz-core.md)) its screens: a sidebar section where
the user asks for a quiz about a study session, takes it, sees the grade with the answer key and
explanations, and reopens past attempts later. The offer inside the study chat is 3.23.

This spec is frontend only. Every binding it needs exists: `GenerateQuiz`, `CancelQuizGeneration`,
`ListQuizzes`, `GetQuiz`, `SubmitQuizAttempt`, `ListQuizAttempts`, `GetQuizAttemptResult`,
`DeleteQuiz` and, for the session choice, the existing `ListSessionsForGeneration`.

## Decisions

1. **A new sidebar section, `quiz`, labeled "Interactive Test"**, unlocked, right after the
   Architecture Lab. It extends `AppSection` and `NAVIGATION` and gets its own branch in
   `app-shell.tsx`. The icon is `ClipboardCheck`.
2. **The quiz screens follow the profile language**, like the lab, because the questions are
   written in that language and labels in another would mix them. A new i18next namespace `quiz`
   (`frontend/src/i18n/locales/quiz-en.ts` and `quiz-pt.ts`, the Portuguese one typed against the
   English one, plus a test that keys and placeholders match) uses the same language switch as
   `setLabLanguage`. The menu label stays English. `AGENTS.md` gets this one-line update.
3. **The catalog screen** (`screens/QuizScreen.tsx`) lists quizzes newest first, one row each: the
   session title, difficulty badge, number of questions, creation date, and the latest score out of
   10 (or "Not taken yet"). Each row has **Take** (**Take again** once it has attempts), **History**
   (only when it has attempts) and **Delete**, which asks for confirmation. An empty list explains
   how to start. The list reloads after a generation, a submission and a deletion.
4. **Generating** opens a dialog: the session (searchable list from `ListSessionsForGeneration`,
   like the lab's dialog), the number of questions (3 to 20, default 10) and the difficulty (easy,
   medium, hard; preselected from the profile's experience level: beginner, intermediate, advanced).
   It shows progress while the model works and a **Cancel** that calls `CancelQuizGeneration`. The
   `QuizError` codes map to translated messages (`invalid_generation` says to try again,
   `generation_in_progress`, `source_session_not_found`, anything else a generic one).
5. **Taking a quiz** (`screens/QuizTakeScreen.tsx`) is a full-screen state of the section, like the
   lab's exam. One question at a time, with **Previous** and **Next**, a numbered strip that shows
   which are answered, and radio options (two for true/false). An answer can be changed or cleared.
   Nothing is saved until the user submits: the answers live in memory in `hooks/use-quiz-session.ts`.
6. **Submitting** is **Finish** on the last question or from the strip. When some questions are
   blank, a confirmation names how many ("2 questions are blank and count as wrong"). It calls
   `SubmitQuizAttempt` and then shows the result of the attempt it returns. If the call fails the
   answers stay and the user can retry.
7. **Leaving mid-quiz discards it.** While a quiz is open, `navigate()` waits for a leave
   confirmation, the same pattern as the exam (`pendingSection`). The answers are not stored, so
   leaving loses them, and the dialog says so.
8. **The result view** (`components/quiz/quiz-result-view.tsx`) shows the grade ("7.5 / 10" and
   "6 of 8"), the session title and difficulty, and every question in order with the user's choice,
   the correct option, whether it was right, blank or wrong, and the explanation. A filter limits
   the list to the questions the user missed. It is read only, and **Take again** is offered.
9. **History** is a dialog (`components/quiz/quiz-history-dialog.tsx`) that lists a quiz's attempts
   newest first (date, grade out of 10, correct of total) and opens the same result view. It never
   offers editing.
10. **No pass or fail.** The grade is only shown out of 10. A color band (below 5, 5 to 7, above 7)
    may help reading it but carries no meaning in the backend.
11. **API client** in `lib/quiz.ts` wraps the bindings, parses `QuizError` from the rejected text
    (the same way `lib/exercise.ts` does for `ExerciseError`) and exposes the types and the pure
    helpers (`scoreBand`, `formatScore`, `countBlank`).

## Tests (TDD)

- `lib/quiz.ts`: wrappers, error parsing for each code and for plain text, the pure helpers
  (including score formatting at 0, 10 and one decimal).
- `hooks/use-quiz-session.ts`: answering, changing and clearing, navigation bounds, blank count,
  submit success and failure (answers kept), no call after a failed load.
- Components and screens with Testing Library and user-event, mocking `@/lib/quiz`: the empty state,
  a populated row with and without attempts, generate (success, cancel, each error code), take,
  blank confirmation, leave confirmation, result view with right, wrong and blank answers, the
  wrong-only filter, history, delete confirmation.
- `app-shell`: the new section opens, and leaving a running quiz asks first.
- The i18n parity test for the `quiz` namespace, in both languages.
- `npx prettier --check .` in `frontend/`, `tsc --noEmit`, vitest coverage at 80% and
  `make mutation-frontend` for the new files.

## Out of scope

The offer inside the study chat (3.23), resuming an unfinished quiz, a timer, a pass or fail
threshold, retaking only the missed questions, and exporting or sharing a quiz.
