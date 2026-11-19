# 2.32 — Keep the message draft when leaving the chat

Issue #171 (follow-up of the PR #168 review).

## Problem

The composer `draft` is local state of `StudyChatScreen`. The shell unmounts the
screen whenever the section changes (mind map, lab, quiz...), so the typed
text is lost on return.

## Design

- `lib/study-drafts.ts`: module-level `Map<sessionId, string>` with
  `getDraft`, `setDraft` (an empty text removes the entry) and `clearDraft`.
- `StudyChatScreen` initialises `draft` from `getDraft(sessionId)` and mirrors
  every change into the store. The screen is already keyed by session, so the
  draft of A never shows in B.
- Sending clears the draft; the pre-persistence rejection path restores it
  through the same setter.
- On mount with a restored draft the textarea height is recalculated.
- In memory only: the draft does not survive an app restart.

## Acceptance

- The draft of session A is back after leaving and returning to A.
- The draft of A does not appear in session B.
- Sending clears the stored draft.
