# Phase 4.4 — Evidence and Session Progress

## Goal

Make each demonstration of the user traceable to the messages that justify it, and let the user
see, on demand, what the tutor observed in the session: what they demonstrated apart from what
was only explained. The stored evidence feeds that view; nothing else reads it yet.

Originally drafted as "Evidence and Session Summary". It is called **Session progress** in code
and UI so it is never confused with the conversation summary of compaction
([`summary-dialog`](../../../frontend/src/components/summary-dialog.tsx)), which is another thing.

## Decisions

1. **One evidence row per turn in which the user's own message showed something** (`userEvidence
   = true` and a valid `assessment`), not only when the status changes. Three correct answers in a
   row are three rows: the repetition is what supports "demonstrated", and it keeps any later
   ratio rule computable. A row holds the concept, the assessment (`CORRECT`,
   `PARTIALLY_CORRECT`, `INCORRECT`), a description written by the model, `assisted`, and the ids
   of the user message and of the reply. Rows without both messages are never created.
2. **The messages are foreign keys with `ON DELETE CASCADE`,** both `NOT NULL`. Today no message
   is deleted on its own and compaction deletes none, so the cascade loses nothing; if a "delete
   message" ever exists, the foreign-key tests are where that gets revisited.
3. **The opening turn writes no evidence.** It answers no user message, so there is nothing to
   point to (and 4.2 already forces `userEvidence = false` there).
4. **The row follows its concept.** A composite key `(session_id, concept)` references
   `session_concepts` with `CASCADE`: when the 30-concept limit of 4.2 evicts a concept, or the
   session or the local data go, its evidence goes too. The Session progress view and the tutor's
   own state therefore always describe the same set of concepts. Consequence: in a session with
   more than 30 concepts the view shows at most 30.
5. **At most 10 rows per concept** (`MaxEvidencePerConcept`), the most recent; the oldest is
   deleted in the same transaction as the insert. A session holds at most 30 × 10 rows.
6. **Same marker, extended.** The 4.2 block gains optional `assessment` and `description`. A
   missing or unknown `assessment` (case-insensitive) is not an error: the report still applies
   to the state, it only writes no row. `description` is cut to 200 runes. No extra LLM call.
7. **Evidence is best effort and apart from the state.** The row is written in its own
   transaction, after the concept's, so a failure (logged without message content) never undoes
   the concept's update and never fails the turn. No row is written for a concept that was not
   stored (a `NOT_ASSESSED` report, or a failed concept write).
8. **The status rule of 4.2 is unchanged:** the last report with evidence wins; there is no ratio
   (for example "70% correct") in code. Evidence is stored so such a rule can be added later.
9. **Session progress is derived, with no LLM call.** `BuildSessionProgress` (domain, pure)
   groups the session's concepts by status: `DEMONSTRATED` (demonstrated), `INTRODUCED` (explained
   only), `PARTIALLY_UNDERSTOOD` (difficulties), `NEEDS_REVIEW` (to review), most recently updated
   first, each with its proofs newest first. The model's `description` is already in the
   assistant language.
10. **On demand, no "end of session".** The domain has no ended session, so a **Session
    progress** button in the chat opens a read-only dialog at any time. It states that it is an
    estimate by the AI tutor, and renders concept names and descriptions as plain text only.
11. **Language.** The dialog has its own i18next namespace (`progress`, `en` and `pt`) that
    follows the profile's assistant language, like the quiz and the mind map.
12. **Local only.** The Learn Intelligent Engine does not exist; no event, bus or schema version
    is built, and evidence is not exportable. An idempotent event (`LearningEvidenceRecorded`) is
    specified when that engine is.

## Domain

`internal/domain/study/`: `Assessment` (three values, `IsValid`), `Evidence`, `NewEvidence(report,
sessionID, userMessageID, replyMessageID, now)` (false without evidence, a valid assessment, a
concept or both message ids), `MaxEvidencePerConcept`, `MaxEvidenceDescriptionRunes`,
`EvidenceRepository` (`Add`, `TrimOldest`, `ListBySession`), `StateReport` gains `Assessment` and
`Description`, and `SessionProgress` / `ProgressConcept` / `BuildSessionProgress`.

## Application

- `tutorStateInstruction` asks for `assessment` and `description` only with `userEvidence`.
- `parseTutorState` reads both (decision 6).
- `streamAndPersist` receives the user message id (empty on the opening turn, replacing the old
  `repliesToUser` flag) and passes it, with the stored reply id, to `recordTutorState`, which
  writes the evidence after the concept (decision 7). Option `WithConceptEvidence`.
- `Service.GetSessionProgress` reads concepts and evidence and builds the progress.

## SQLite

`concept_evidence` (`id` autoincrement, `session_id`, `concept`, `assessment` with a CHECK,
`description`, `assisted`, `user_message_id` and `reply_message_id` both `NOT NULL` referencing
`messages` with `CASCADE`, `created_at`, composite foreign key to `session_concepts`), an index
on `(session_id, concept, created_at)`, an idempotent migration, and the reset clears it.

## Interface

Wails `GetStudySessionProgress(sessionID)` returns four lists (never null), each concept with
its proofs. Frontend: `getStudySessionProgress`, `ProgressDialog`, and a **Session progress**
button next to **View summary**, always enabled.

## Out of scope

- The Learn Intelligent Engine, its events and metrics; exporting evidence.
- A ratio-based status rule, cross-session mastery and spaced-review scheduling.
- An "ended session" state, and a summary written by the model (it can replace the derived view
  later if the derived one reads poorly with real sessions).
- Showing which message a proof points to (the ids are stored, the dialog does not use them yet).

## Open follow-up

How reliable the model's self-assessment is cannot be answered without real sessions. The dialog
says it is an estimate; evaluate it with use before building anything that depends on it.
