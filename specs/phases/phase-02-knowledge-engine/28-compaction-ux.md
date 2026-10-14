# Phase 2.28 — Compaction UX (optional)

> **Status: optional.** Do this only after 2.25 is in use and the transient notice proves
> insufficient. Re-grill the scope before implementing.

## Goal

Make compaction legible and controllable. After 2.25 the model "forgets" older turns verbatim and
works from a summary; the user should be able to see where that happened, read what Athena
remembers, trigger it deliberately, and be re-oriented when they return to an old session.

## Candidate scope

1. **Transcript marker.** A divider at the point covered by the summary ("Earlier messages were
   summarized") derived from `covers_until_message_id`. Requires returning the summary metadata
   with the resume DTO.
2. **View summary.** A read-only panel/dialog showing the current summary (the fixed sections from
   2.24: progress, open difficulties, pending). Read-only first; editing is a separate decision
   because user-edited LLM memory raises its own questions.
3. **Compact now.** A manual action next to the context meter (2.22) that runs the same compaction
   path as 2.25 on demand, with the same fallback behavior.
4. **Session recap on reopen.** When a session is reopened after a configurable idle period, the
   tutor may open with a short recap built from the summary's `Open difficulties` / `Pending`
   sections ("Last time you struggled with X; there is a pending exercise on Y"). This is a new
   kind of opening turn and needs its own prompt and a rule against fabricating progress that is
   not in the summary.

## Constraints carried from earlier specs

- The summary is untrusted LLM output and never a citable source (2.24, 2.25).
- No structured learning progress table and no cross-session memory (2.24 product premise).
- Business logic stays in `domain`/`application`; bindings stay thin.

## Open questions (resolve before implementing)

- Is the marker enough, or is the viewer needed?
- Should "compact now" exist at all if automatic compaction is reliable?
- Idle threshold and opt-out for the recap.
