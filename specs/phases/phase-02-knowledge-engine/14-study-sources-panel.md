# Phase 2.14 — Study Sources Panel

## Goal

Reshape the Study screen into a NotebookLM-style three-pane layout — sidebar, chat,
and a right-hand Sources panel — as a visual/structural shell only. No new schema, no
new backend surface, no change to any behavior that already works today. This lands
the interface ahead of a larger, separate pivot the product is moving toward (see
"Deferred to a future increment" below), so that pivot only has to wire data into an
already-shaped screen instead of building the screen too.

## Scope decision: presentational shell, not a functional feature

The Sources panel's "Add" and per-source "Remove" controls render, styled per the
approved mockup, but are **disabled** with a "Coming soon" tooltip — they do not call
into any backend action this increment. This was an explicit, deliberate choice
("não precisa estar funcional agora, é só deixar a tela pronta"), not an oversight:

- There is no session↔document ownership in the domain today. `knowledge.Item` has no
  `FolderID`/`SessionID`; an imported document's `Topic` is derived from the picked
  import folder (`ingest.BuildShadowItem`), unrelated to any Study session. Building a
  real "attach a file to this session" action means adding that ownership first — out
  of scope here, see below.
- Making Add/Remove actually mutate global knowledge (today's only real primitive —
  the existing global import dialog, the existing `KnowledgeDeleteDialog`) from a
  panel whose UI implies "this session's sources" would be actively misleading:
  deleting from one session's panel would silently remove the item from every other
  session that happens to cite it too, with no warning surfaced anywhere in this
  screen.

Given both, disabled controls with a "Coming soon" tooltip are more honest than a
half-wired action that does the wrong thing.

## Design

- **`frontend/src/components/study-sources-panel.tsx`** (new) — presentational. Takes
  `sources: StudySource[]`, dedupes nothing itself (its caller already deduped),
  renders a client-side-searchable list via `sourceLabel` (moved to `lib/study.ts` so
  `LocalSourcesStrip` and this panel share one mapping instead of two). Add/Remove
  buttons are `disabled`, wrapped in the same `Tooltip` pattern the composer's Send
  button already uses for a disabled-button tooltip.
- **`frontend/src/screens/StudyChatScreen.tsx`** — gains an optional
  `onSourcesChanged?: (sources: StudySource[]) => void` prop. A `useMemo` over the
  existing `messages` state (no new fetch) dedupes every `StudySource` seen across the
  session's messages so far, keyed by `${sourceType}|${filePath}|${concept}` (Source
  has no stable id — see `StudySource` in `lib/study.ts`); an effect reports that list
  upward whenever it changes. `messages` itself is untouched — still owned exactly
  where it is today.
- **`frontend/src/components/app-shell.tsx`** — owns `sourcesOpen` (mirrors the panel's
  own reported size) and `sessionSources` (fed by `onSourcesChanged`). The Study
  section's main pane is a real nested `ResizablePanelGroup` — `StudyChatScreen`'s
  column as one `ResizablePanel` (`minSize={360}`), a `ResizableHandle`, then
  `StudySourcesPanel` as a second `ResizablePanel` (`defaultSize={340}`,
  `minSize={280}`, `maxSize={520}`, `groupResizeBehavior="preserve-pixel-size"`,
  matching the sidebar's own pattern) — so it drags to resize exactly like the
  sidebar does. The header toggle button drives it via the panel's own
  `collapsible`/`collapsedSize={0}` + `panelRef` imperative API
  (`collapse()`/`expand()`), not mount/unmount: `onResize` reports the panel's new
  pixel size back into `sourcesOpen`, and the panel's own state (e.g. its search
  query) survives being hidden. The whole nested group only renders for
  `section === 'study' && activeSession`; every other screen (and Study with no
  session open) renders the same shared `studyChatColumn` JSX directly, with no
  group at all — panel count inside a `Group` never changes while one is mounted,
  avoiding any risk from this codebase's one precedent for dynamic panels.
  `src/test/setup.ts` forces every `data-panel` element's
  `getBoundingClientRect` to a zero-size rect (a deliberate fix for a real
  divider-hit-testing bug elsewhere), so the collapse/expand round trip's actual
  pixel math cannot be exercised by this suite — `app-shell.test.tsx` only asserts
  the toggle renders and doesn't throw; the visual round trip needs a real window.
- Existing chat behavior — composer, `Extract knowledge`, `SourceMode` select, context
  banners, streaming — is untouched.

## Tasks

- [x] `frontend/src/lib/study.ts` — export `sourceLabel(source): { title, subtitle }`
- [x] `frontend/src/components/local-sources-strip.tsx` — import `sourceLabel` instead
      of defining its own copy
- [x] `frontend/src/components/study-sources-panel.tsx` — new presentational component
- [x] `frontend/src/screens/StudyChatScreen.tsx` — `onSourcesChanged` prop, derived via
      `useMemo` over `messages`
- [x] `frontend/src/components/app-shell.tsx` — `sourcesOpen`/`sessionSources` state,
      header toggle button, Study main pane restructured to render the panel
- [x] Test mocks: `app-shell.test.tsx` and `StudyChatScreen.test.tsx` mock
      `@/lib/study` wholesale (no `importOriginal`) — both updated to spread the real
      module so the newly-shared `sourceLabel` stays real under test, matching the
      existing `@/lib/knowledge` mock's pattern in `app-shell.test.tsx`

## Acceptance Criteria

- Opening a Study session shows sidebar, chat, and the new Sources panel; collapsing
  it via the header toggle leaves chat filling the space, and the toggle reopens it.
- The panel lists every distinct Source cited so far in the open session's messages,
  searchable client-side; it never calls a backend endpoint.
- "Add" and each row's "Remove" are visibly disabled with a "Coming soon" tooltip —
  not silently inert, not hidden.
- The left nav's `Knowledge` item, the Review tab, and the composer's
  `Extract knowledge` button are unchanged.
- No SQL migration, no new Go application/domain code, no new IPC binding.
- Existing test suites (Go and frontend) still pass; `study-sources-panel.tsx` gets
  its own rendering tests.

## Deferred to a future increment

This increment deliberately stops short of the actual product pivot already discussed
and agreed on, but not yet designed in detail. Session↔document ownership and the
decision on documents already imported today are now specified in
[15-session-scoped-knowledge.md](15-session-scoped-knowledge.md); the functional panel (Add,
Remove, the real source list, removing the Knowledge nav entry) in
[16-session-sources-panel.md](16-session-sources-panel.md); and the removal of session
extraction in [17-remove-conversation-extraction.md](17-remove-conversation-extraction.md):

- Deprecate session-extraction entirely: remove `ExtractFromSession`, the
  `Extract knowledge` composer button, and the draft/reconciliation review workflow.
- Move "Import notes" from a global action to a session-scoped one: importing a file
  attaches it to the currently open session.
- Make Add/Remove real: define the session↔document ownership (schema + domain), wire
  the panel's Add to session-scoped import and Remove to a real detach/delete, with an
  explicit decision on hard-delete vs. detach semantics.
- Decide what happens to documents already imported today (no session owner) once
  session-scoping exists — do they stay globally visible everywhere, or need a
  one-time migration decision?
- Remove `Knowledge` from the left nav once Explorer/Review have nothing left to
  review under the new model.
- Replace the panel's in-memory (`messages`-derived) source list with a real read of
  `knowledge.MessageSourceRepository.ListBySession` (already exists, unused by the
  frontend today) once the panel needs to survive a session resume with no new replies
  yet.
