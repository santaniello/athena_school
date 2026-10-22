# Phase 2.30 — Find in conversation

> **Status: scoped.** Ctrl+F search inside the study chat.

## Goal

Let the user search the whole conversation of a study session with Ctrl+F: matches are
highlighted in a different colour and the user can jump between them, as in a browser's find bar.

## Scope

1. **Open / close.** Ctrl+F (Cmd+F on macOS) opens an inline find bar at the top of the transcript
   and focuses its input, preventing the webview's default. Esc closes it, clears the highlights and
   returns focus to the composer. The shortcut is only active while the study chat is mounted.
2. **Search.** Case- and accent-insensitive substring search over the text of every message
   (user and assistant, including code blocks). Headers, buttons and other controls ("Copy code",
   "Copy message", citation chips' chrome) are not searched.
3. **Highlight.** All matches get one highlight colour and the current match a stronger one, using
   the CSS Custom Highlight API so the DOM is not mutated. Without that API the search and the
   navigation still work, only without colour.
4. **Navigate.** The bar shows "N of M" or "No results". Enter / the Next button go to the next
   match, Shift+Enter / the Previous button to the previous one; both wrap around. The current
   match is scrolled to the centre of the transcript.
5. **Whole history.** Opening the bar loads every earlier page of the history (the chat is
   paginated by 50), showing "Loading history…" meanwhile. A failed page shows a warning and the
   search keeps working over what is loaded. Closing the bar stops the loading.
6. **Live content.** Matches are recomputed when the content changes (streaming reply, prepended
   page) without moving the current position unexpectedly.

## Constraints carried from earlier specs

- 2.23: a finished `MessageBubble` must not re-render because of the search; highlighting lives in
  the DOM layer, not in bubble props or state.
- 2.27: prepended pages keep the viewport on the message being read; the find bar must not fight
  that, nor the sticky-bottom follow (navigating a match stops following the bottom).
- UI text is English; a single fixed dark theme with the existing tokens. Accessible names are
  unique ("Find in conversation", "Next match", "Previous match", "Close find").
- No business logic in React components: matching lives in `lib/`, state in a hook.

## Not planned

- Regex, whole-word or case-sensitive toggles.
- Searching other screens, the sources panel or the summary dialog.
- Server-side full-text search (the separate message search / FTS topic of 2.26).

## Acceptance

- Ctrl+F shows the find bar focused; typing a term highlights every occurrence in the user and
  assistant bubbles and in code blocks, and shows the match count.
- Enter / Shift+Enter move the strong highlight forward / backward, wrapping, and scroll to it.
- A term that only appears in a message of an earlier page is found after the history loads.
- Esc closes the bar, removes all highlights and refocuses the composer.
- A streaming reply that contains the term is picked up without breaking auto-scroll.
- Finished bubbles still render once while another message streams.
