# Phase 2.23 — Cheap Chat Rendering

## Goal

Stop the chat from re-rendering and re-parsing every message on each streamed chunk. Today
`StudyChatScreen` mirrors streamed text into state, so every chunk re-renders the whole screen
and every `MessageBubble` re-runs `react-markdown` + syntax highlighting. Cost grows linearly with
conversation length, which matters because study sessions are long-lived by design (2.24–2.25).

No behavior change visible to the user. The only backend touch is exposing the persisted message
id (see decision 1); everything else is frontend.

## Decisions

1. **Stable message identity.** Today `StudyMessageResult` (`interfaces/desktop/study.go`) carries
   no `id`, so the DTO gains `ID string \`json:"id"\`` populated from `domainstudy.Message.ID`
   and `wailsjs` is regenerated. `ChatMessage` gains an `id`: the persisted id for messages from
   `ResumeStudySession`; a client-generated id for the optimistic user message and the streamed
   assistant message (these keep their client id for the lifetime of the mounted screen — the
   `study:done` event does not need to carry the persisted id). Lists are keyed by `id`, not array
   index.
2. **`MessageBubble` is memoized** (`React.memo`) with props that are referentially stable for
   finished messages: `message`, `onOpenCitation` (wrapped in `useCallback`) and the sources.
   A finished bubble never re-renders while another message streams.
3. **Streaming state is isolated.** The streaming bubble lives in its own component that owns the
   accumulated text; `StudyChatScreen` no longer holds `streamingText` in state, so chunks re-render
   only that component. The `streamingTextRef` used for the final push on `study:done` is kept.
4. **Scroll-following follows the leaf.** The sticky-bottom effect (80px threshold) is driven by
   the streaming component notifying the parent (or a shared ref) instead of a screen-level
   `[messages, streamingText]` effect.
5. **No virtualization, no pagination here.** Those are 2.26–2.27 / 2.29. This spec only removes
   avoidable work.
6. **Measure, don't guess.** The spec's acceptance uses a render-count test, not a timing
   benchmark (jsdom has no layout).

## Tasks

1. Backend: add `ID` to `StudyMessageResult` and its mapping (test), regenerate `wailsjs`.
   Frontend: add `id` to `ChatMessage`; thread persisted ids through `resumeStudySession` /
   `lib/study.ts`.
2. Key by id; memoize `MessageBubble`; stabilize callbacks.
3. Extract the streaming bubble into its own component owning chunk state.
4. Rework the scroll-following effect.
5. Tests: render-count assertions (a finished bubble renders once while N chunks stream).
6. Docs: `CHANGELOG.md`.

## Acceptance Criteria

- While a response streams, finished `MessageBubble`s are not re-rendered (render-count test).
- Sticky-bottom behavior is unchanged: follows the stream when near the bottom, stops when the
  user scrolls up, resumes when they return.
- Citation chips, copy buttons and the sources strip behave exactly as before.
- Existing `StudyChatScreen` tests pass; coverage stays at or above the 80% thresholds and the
  mutation run on touched files does not regress.

## Out of Scope

- Backend changes beyond exposing the message id, pagination, virtualization, markdown pipeline
  changes.
