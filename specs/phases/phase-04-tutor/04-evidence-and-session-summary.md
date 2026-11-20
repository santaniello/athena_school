# Phase 4.4 — Evidence and Session Summary (draft, not scheduled)

> **Status: draft.** Do not start before 4.1 to 4.3 have been used for a while and shown the state
> is reliable. Written now only to fix the direction; decisions below are provisional and must be
> re-reviewed before implementation.

## Goal

Make each state change traceable to the messages that justify it, and give the user a summary
when a session ends that separates concepts the tutor explained from concepts the user
demonstrated. The stored evidence is also the integration point for the future Learn Intelligent
Engine.

## Provisional decisions

1. **An evidence row per state change with `userEvidence = true`,** holding the concept, the
   assessment (`CORRECT`, `PARTIALLY_CORRECT`, `INCORRECT`), a short description written by the
   model, the assistance level and the ids of the user message and the reply that justify it.
   Rows without a supporting message are rejected, never stored.
2. **Same marker, extended.** The 4.2 block gains optional `assessment` and `description` fields;
   no extra LLM call.
3. **Local only.** The Learn Intelligent Engine does not exist in the codebase, so no event, bus
   or schema version is built. The evidence table is the hand-off: the future engine reads it. An
   idempotent event (`LearningEvidenceRecorded`) is specified when that engine is.
4. **Session summary** at the end of a session: concepts studied, demonstrated vs. only explained,
   observed difficulties, concepts to review. Derived from `session_concepts` and the evidence rows
   with at most one LLM call for the wording, in the assistant language, triggered by the user.
5. **Failures never stop the conversation;** an evidence write error is logged and the turn goes on.
6. **Recommendations stay as they are.** Quiz, mind map and lab are already offered through the
   offer markers; wiring them to the state is a prompt tweak to evaluate afterwards, not part of
   this spec.

## Open questions before this becomes a real spec

- Is the model's self-assessment reliable enough to show a summary to the user? Decide with real
  sessions from 4.2 and 4.3.
- Where does the summary live in the UI (end-of-session dialog, or a panel in the chat)?
- Should evidence be exportable, or only used by the future engine?

## Out of scope

- The Learn Intelligent Engine, its events and metrics.
- Cross-session mastery and spaced-review scheduling.
