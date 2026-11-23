# Phase 4.5 — Session Materials Panel

## Goal

Show, inside the study session, everything that belongs to it: the right-hand panel that today lists
only **Sources** becomes **Session materials** and also lists the session's **Mind maps**,
**Arch Labs** and **Quizzes**. Today the user has to leave the session, open the menu item and filter
by session to reach them. Frontend only: the bindings that list them per session already exist.

A mockup was approved before this spec (sections stacked, each with its own scroll).

## Decisions

1. **Four collapsible sections, in this order:** Sources, Mind maps, Arch Labs, Quizzes. Each header
   has a caret, the title and a count badge.
2. **Default state: only Sources is open.** Every time a session is opened (the active session
   changes) the other three start closed. A section opened or closed by the user keeps that state
   until the session changes.
3. **Each open section scrolls on its own.** Open sections share the panel height equally; a closed
   one takes only its header, so the others grow. An open section has a minimum height; if the
   open sections do not fit, the whole panel scrolls.
4. **Sources keeps working exactly as before:** Add (Markdown or link), Example, per-row Remove, the
   viewer, citations that open it, the "Rebuilding knowledge index…" lock. Its Add and Example
   buttons move into the Sources section header. It is the only place to manage sources.
5. **The other three sections are read-only lists.** No create or delete: those stay in the chat
   (offers) and in the menu pages.
6. **The search field is removed.** The old Sources-only search box goes away; no shared search
   replaces it.
7. **A row opens the item** the same way the chat cards already do:
   - Mind map → `onOpenMindMap(id, sessionId)`.
   - Quiz → `onStartQuiz(id, sessionId)`.
   - Arch Lab → the same Start Exam rules dialog as the chat card, then `onStartExam(exam, sessionId)`.
8. **Rows.** Mind map: title and "Updated <date>". Quiz: "Quiz N · <difficulty>" (N follows creation
   order), question count, date and "Not taken" or attempts; badge with the latest score or "New".
   Arch Lab: title, kind/difficulty, time limit, attempts; badge with the status (Passed, Failed or
   the challenge status).
9. **Freshness.** The three lists load when the panel mounts for a session (so coming back from a
   quiz, map or lab shows new attempts). While the study chat is open, `StudyChatScreen` tells the
   shell when its own cards change (`onMaterialsChanged`), and the shell bumps a revision the panel
   reloads on. A failed reload keeps the previous list; a failed first load shows a per-section
   error with Retry.
10. **Strings.** The new panel strings follow the profile's assistant language through the
    `materials` i18next namespace (`materials-en.ts` / `materials-pt.ts`). The Sources internals keep
    their current English copy.

## Out of scope

Attempt history for labs and quizzes, deleting items from the panel, a search over the materials,
resizing individual sections.
