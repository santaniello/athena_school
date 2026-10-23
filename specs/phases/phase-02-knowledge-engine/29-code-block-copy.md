# Phase 2.29 — Code block copy

> **Status: scoped.** ChatGPT-style code blocks in the study chat.

## Goal

Let the user copy a single code block (or markdown snippet) from a tutor reply without selecting
text by hand. Today only the whole message can be copied.

## Scope

1. **Block header.** Every fenced code block gets a header bar above the code: the language name on
   the left and a "Copy code" button on the right.
2. **Copy.** The button writes exactly the block's code (no trailing newline) to the clipboard,
   switches to "Copied" for 2 seconds, then returns to "Copy code". A clipboard failure leaves the
   button unchanged and does not throw.
3. **Fenced blocks without a language** render as a block too (label `text`), instead of falling
   into the inline-code style. Inline `` `code` `` is unchanged.
4. **`markdown` highlighting.** ` ```markdown ` blocks are syntax highlighted.

## Constraints carried from earlier specs

- 2.23: a finished `MessageBubble` must not re-render while another message streams. The copied
  state lives inside the code block, never in the bubble or its `components` map.
- The highlight stays lazy (`IntersectionObserver`); the header is cheap and always rendered.
- UI text is English; a single fixed dark theme with the existing tokens.

## Not planned

- Line numbers, word-wrap toggle, download button.
- Copying with citation markers stripped: code blocks never contain them.

## Acceptance

- A ` ```go ` block shows `go` and a "Copy code" button; clicking it copies the code verbatim.
- A fenced block with no language shows `text` and the same button.
- Inline code has no header and no button.
- The "Copy message" button is still unique by accessible name.
