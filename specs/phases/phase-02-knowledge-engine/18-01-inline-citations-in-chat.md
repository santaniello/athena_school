# Phase 2.18.1 — Inline Citations in Chat (Spec 2.18, PR 2)

## Goal

PR 1 of spec [2.18](18-notebooklm-style-citations.md) shipped the stored source, its cascade
delete, and a read-only document viewer opened from a Sources-panel row click (no passage
highlighted). This spec covers **PR 2**: the model cites `[n]` inline in its replies, a citation
is a clickable/hoverable chip, and clicking one expands the Sources panel and opens the viewer at
the exact cited passage, highlighted.

## Status

A first attempt at this PR was fully implemented on a branch stacked on PR 1
(`feature/inline-citations`) through the frontend citation-chip slice (backend done, chat UI
done) — every quality gate green (`go test -race ./...`, `make mutation-go`, frontend
tests/typecheck/lint/format, Stryker on changed files) — then **intentionally discarded**
unmerged, to review it in its own window rather than bundled with this planning session. Nothing
of it survives in git history or on any branch. This spec exists so the redo is mechanical, not
exploratory: every file, signature, and gotcha discovered doing it once is recorded below.
`app-shell.tsx` was never reached before the discard — that part (the last third of the "Frontend
— not yet attempted" section) is still real design work, not a redo.

Depends on PR 1 already merged to `develop` (uses `knowledge_documents`, `GetSourceDocument`,
`source-viewer.tsx`, `useSourceDocument`).

## Decisions carried over from spec 2.18 (unchanged)

See the parent spec's own numbered decisions (1–14) — this PR implements decisions 1–4, 11–14
specifically. Notably decision 1 (`[n]` syntax, `n` = 1-based position, no new table), decision 2
(the model is asked, never trusted — a marker only becomes a citation when that reply's `n`-th
source actually exists), decision 3 (markers stripped from history sent back to the model, never
from the stored/displayed message), decision 4 (hover uses the persisted excerpt; click needs the
document), decision 10 (a citation to a removed document still previews on hover; clicking
explains it was removed — no new code needed for this, see "Already covered by PR 1" below),
decision 11 (`AppShell` controls the panel from the chat), decision 12 (Copy strips markers),
decision 13 (viewer renders plain text), decision 14 (a source needs a stable id — `chunkId`).

## Backend — redo verbatim, this exact shape worked and passed every gate

### 1. `StudySourceResult` gains `chunkId`/`itemId`/`excerpt`

`internal/interfaces/desktop/study.go`: the domain `Source`
(`internal/domain/knowledge/retrieval.go:82-91`) already carries `ChunkID`/`ItemID`/`Excerpt` —
only the desktop DTO was withholding them (its doc comment literally said "deliberately omits
internal IDs and the full excerpt", written for spec 2.9 and now stale). Add:

```go
type StudySourceResult struct {
	ChunkID    string  `json:"chunkId"`
	ItemID     string  `json:"itemId"`
	SourceType string  `json:"sourceType"`
	FilePath   string  `json:"filePath"`
	Heading    string  `json:"heading"`
	Concept    string  `json:"concept"`
	Score      float32 `json:"score"`
	Excerpt    string  `json:"excerpt"`
}
```

`toStudySourceResults` maps the three new fields straight from `s.ChunkID`/`s.ItemID`/`s.Excerpt`.
This single helper backs both the live `study:sources` event and `ResumeStudySession`'s history
DTO, so one change covers both. Update `TestApp_SendStudyMessage_emitsPostCapSourcesEvent_notes`
(`internal/interfaces/desktop/study_test.go`) — its retriever mock already returns a `Source` with
`ChunkID: "chunk-1", ItemID: "item-1", Excerpt: "..."`, the assertion just needs to expect them.
Regenerate `wailsjs` (`wails generate module`; `chmod 644` the three `frontend/wailsjs/runtime/*`
files it touches — that command flips their mode to 755 with no content change, unrelated noise
worth reverting before committing).

### 2. `contextEntry` gains a 1-based `id`

`internal/application/knowledge/retrieval.go`: `contextEntry` gains `ID int \`json:"id"\`` (first
field). In `renderContext`, `entries[i].ID = i + 1` — `entries[i]` and the later
`sources[i]` (built from the same `capped[i]`) are already the same index, so this is the only
line needed; no new column, no new table. Update
`TestRetrieve_excludesEmbeddingAndScoreFromRenderedJSON`'s expected key set to six keys (add
`"id"`). Add a dedicated test proving `id` matches `Sources`' own order
(`TestRetrieve_numbersContextEntriesByOnebasedPosition_matchingSourcesOrder` in the discarded
branch — two chunks, assert `entries[0]["id"] == 1` pairs with `Sources[0].ChunkID`, etc.).

### 3. The citation instruction

`internal/application/study/prompt_context.go`: a new shared constant, appended to whatever
`instructionFor` returns inside `buildKnowledgeContext` (not duplicated into its three branches):

```go
const citationInstruction = "Cite the passage(s) you used for each claim with [n] right after it, where n is that passage's \"id\" field in the JSON block below. Only cite an id that is actually present in the JSON block; never invent one."
```

```go
Content: fmt.Sprintf(
	"%s\n\n%s %s\n\n%s",
	untrustedDataFraming, instructionFor(sourceMode, result.Sufficient), citationInstruction, result.Context,
),
```

Test: mirror `TestBuildKnowledgeContext_alwaysIncludesUntrustedDataFraming_regardlessOfModeOrSufficiency`
(same four mode/sufficiency cases) asserting `citationInstruction` is present, plus loose
`Contains(message.Content, "[n]")`/`Contains(..., "id")` smoke checks.

### 4. Strip `[n]` from prior assistant turns before resending

New file `internal/application/study/citations.go`:

```go
var citationMarker = regexp.MustCompile(`\[\d+\]`)

func stripCitationMarkers(content string) string {
	return citationMarker.ReplaceAllString(content, "")
}
```

`internal/application/study/send_message.go`, in the history-building loop: strip only when
`message.Role == domainstudy.RoleAssistant`, never for `RoleUser`, and only in the copy sent to
the model — the persisted/displayed message is untouched:

```go
for _, message := range history {
	content := message.Content
	if message.Role == domainstudy.RoleAssistant {
		content = stripCitationMarkers(content)
	}
	llmMessages = append(llmMessages, domainllm.Message{Role: message.Role, Content: content})
}
```

New `citations_test.go` (five direct unit tests: single marker, multiple/multi-digit markers, no
markers, empty string, a bracketed non-digit word left alone). New test in
`send_message_test.go`: a prior assistant turn containing `[2]` is stripped from what
`ChatStream` receives, while a prior *user* turn's literal `[3]` survives verbatim — **gotcha**:
`len(req.Messages)` in the mock matcher must equal `1 (system) + len(history)` exactly; miscounting
this was the one bug hit redoing this slice.

## Frontend — implemented once, redo verbatim (citation chip in chat)

### `frontend/src/lib/citations.ts` (new)

A remark plugin plus the Copy helper:

```ts
export const CITATION_TAG_NAME = 'citation-chip'
const citationPattern = /\[(\d+)\]/g

export function remarkCitations() { /* unist-util-visit over 'text' nodes */ }
export function stripCitationMarkers(text: string): string {
	return text.replace(citationPattern, '')
}
```

`remarkCitations` visits `text` nodes, splits each match into `text`/`citationChip` pieces via
`parent.children.splice(index, 1, ...replacement)`, and returns `index + replacement.length` so
`visit` skips re-scanning the inserted plain-text pieces. It never needs special-casing for code
spans, fenced code blocks, or `[n](url)` link syntax — mdast already represents the first two as a
`value` string (never `text` children, so a `text`-only visitor never reaches inside), and the
third is already a separate `link` node with its own `text` child by the time this plugin runs (a
literal `"]("` never appears inside a `text` node's value here).

**Gotcha (real, not optional): a custom mdast node type needs a type-level declaration or
`parent.children.splice(...)` fails `tsc`.** mdast's `RootContent`/`PhrasingContent` are closed
unions; TypeScript rejects splicing in an unknown `type: 'citationChip'`. Fix with module
augmentation (the same pattern `remark-directive`/`remark-emoji` use), declaring **both** maps —
`PhrasingContentMap` alone is not enough, `RootContent` is checked separately during `visit`:

```ts
export interface CitationChipNode extends Node {
	type: 'citationChip'
	data: Data & { hName: typeof CITATION_TAG_NAME; hProperties: { citationIndex: number } }
	children: []
}
declare module 'mdast' {
	interface PhrasingContentMap { citationChip: CitationChipNode }
	interface RootContentMap { citationChip: CitationChipNode }
}
```

`@types/mdast` and `unist-util-visit` (with its own `@types/unist`) are already resolvable —
transitive deps of `react-markdown`/`remark-gfm`, confirmed present in `node_modules` and
confirmed sufficient for `tsc --noEmit` to pass without adding either to `package.json`.

Test file `citations.test.ts`: build the tree with
`unified().use(remarkParse).use(remarkGfm).use(remarkCitations)`, then
`processor.runSync(processor.parse(markdown))` (not `.process`/`.processSync`, which would need a
stringifier that knows how to serialize the custom node — unnecessary, just inspect the tree).
~13 cases: single/multiple/adjacent markers, document order, the exact text split around a chip,
`hName`/`hProperties` values, no markers, code span skip, fenced block skip, `[n](url)` skip
(assert a real `link` node exists), plus `stripCitationMarkers`'s own cases.

### `frontend/src/components/citation-chip.tsx` (new)

```ts
interface CitationChipProps {
	citationIndex: number       // 1-based, same index into sources
	sources: StudySource[]      // the reply's full list; a miss renders plain text
	onOpenCitation?: (source: StudySource) => void  // omitted while streaming
}
```

`sources[citationIndex - 1]`; renders `` {`[${citationIndex}]`} `` (plain text) when that index has
no source, so a model-invented number or a stale `[n]` in an old reply never links anywhere.
Reuses `sourceLabel()` from `lib/study.ts` for title/subtitle — already returns exactly
`{title: filePath, subtitle: heading}` for `imported_doc`, which is what the hover preview needs.

**No dedicated hover-card primitive exists in this codebase** — only `Tooltip`
(`components/ui/tooltip.tsx`, Radix-backed). Reuse `Tooltip`/`TooltipTrigger`/`TooltipContent`
(already used for rich multi-line content elsewhere, e.g. the Sources panel's "Rebuilding
knowledge index…" tooltip) instead of adding a new shadcn primitive — `npx shadcn add hover-card`
needs registry/network access this environment may not have, and Tooltip already does the job.
`EXCERPT_PREVIEW_LENGTH = 300`, truncate with `…`. Render as
`<sup><button type="button" data-slot="citation-chip" onClick={() => onOpenCitation?.(source)}>{citationIndex}</button></sup>`
inside the Tooltip trigger.

### `frontend/src/components/message-bubble.tsx`

New props `sources?: StudySource[]` (default `[]`) and `onOpenCitation?: (source: StudySource) => void`.
`remarkPlugins={[remarkGfm, remarkCitations]}`. The `components` map (previously a static
module-level `markdownComponents` constant) must become per-render (`useMemo`, deps
`[sources, onOpenCitation]`) merging the static map with
`{ [CITATION_TAG_NAME]: (props) => <CitationChip citationIndex={props.citationIndex ?? 0} sources={sources} onOpenCitation={onOpenCitation} /> }`.
Confirmed `react-markdown`'s `Components` type accepts an arbitrary custom tag-name key like
`'citation-chip'` with no cast needed. `handleCopy` calls `stripCitationMarkers(content)` before
`navigator.clipboard.writeText`. ~8 new tests: chip renders for a valid index; plain text for an
out-of-range index; plain text with no `sources` prop at all; hover shows title/heading/excerpt
start; click calls `onOpenCitation` with the resolved source; a *streaming* bubble (`isStreaming`
+ `sources` set) still resolves chips; Copy strips markers; `[n]` inside a code span stays
unconverted.

### `frontend/src/components/local-sources-strip.tsx`

New prop `onOpenCitation?: (source: StudySource) => void`. Each entry's title/subtitle/score
block wraps in a `<button type="button" onClick={() => onOpenCitation?.(source)}>`, and gains its
1-based number as a prefix. **Gotcha**: write the number and title as *one* interpolated string —
`` {`${index + 1}. ${title}`} `` — not `{index + 1}. {title}` as separate JSX expressions; the
latter creates multiple text nodes and breaks every existing exact-match `getByText('Athena
Knowledge')`-style query in `local-sources-strip.test.tsx` and `StudyChatScreen.test.tsx` (they
all needed switching to a regex substring match, e.g. `getByText(/Athena Knowledge/)`, once the
prefix was added — do that alongside this change, not as an afterthought). Two new tests:
numbering by position, click calls `onOpenCitation` with the resolved source.

### `frontend/src/screens/StudyChatScreen.tsx`

New prop `onOpenCitation?: (source: StudySource) => void`, forwarded to both `MessageBubble` (with
`sources={message.sources}` for a completed message) and `LocalSourcesStrip` calls.

**Gotcha (a real `eslint-plugin-react-hooks` `react-hooks/refs` error, not a style nit):** the
streaming bubble needs the turn's already-announced sources so a citation resolves before
`study:done`, and the existing `pendingSourcesRef.current` (a ref) held exactly that — but reading
`ref.current` during render is flagged as an error by this rule. Fix: add a parallel **state**,
`const [streamingSources, setStreamingSources] = useState<StudySource[]>([])`, set alongside the
ref in the `onStudySources` handler (`setStreamingSources(event.sources)`) and cleared alongside
it in `onStudyDone` (`setStreamingSources([])`). Keep the ref too — `onStudyDone` still needs a
synchronous read to attach sources to the persisted message object; only the *streaming bubble's
render* switches from the ref to the state. Four new tests: a chip resolves mid-stream (sources
announced, then a chunk citing `[1]` arrives, before `study:done`); clicking a chip in a completed
message calls `onOpenCitation`; clicking a Local-sources entry calls it too; existing
sources-related tests updated for the numbering-prefix regex change above.
`renderNewSession`/`renderStartedSession` test helpers need an optional `props` parameter to
forward `onOpenCitation` into the new tests.

### `frontend/src/lib/study.ts`

`StudySource` gains `chunkId: string`, `itemId: string`, `excerpt: string`. This breaks every
existing test file that builds a `StudySource` object literal without them —
`local-sources-strip.test.tsx`, `study-source-key.test.ts`, `StudyChatScreen.test.tsx` all needed
a small `testSource(overrides: Partial<StudySource>): StudySource` factory (harmless defaults,
`...overrides` spread) added locally, then every literal wrapped in `testSource({...})`, rather
than hand-editing each one.

## Frontend — not yet attempted (real design work, not a redo)

`app-shell.tsx` was never opened this round. Per spec 2.18 decision 11, it already owns the
Sources panel's collapse state and a ref to it (from spec 2.17) — read it first to learn that
shape before designing the rest of this section; nothing below should be taken as verified against
the real file.

- **`study-sources-panel.tsx`** (PR 1 shape: internal `viewingItemId: string | null` state, set
  only by its own row clicks) needs to also accept an *external* open request — e.g. a prop like
  `openRequest?: { itemId: string; chunkId?: string } | null` that a `useEffect` turns into the
  panel's own view state, so a citation click from the chat can drive it the same way a row click
  does today.
- **`source-viewer.tsx`** (PR 1 shape: renders segments with no highlight) needs an optional
  `chunkId` to highlight: find the segment whose `chunkId` matches, scroll it into view, apply a
  brief emphasis (fade-out class) — segments already carry their own `chunkId`
  (`data-chunk-id` in the DOM already, from PR 1), so this is a targeted addition, not a rewrite.
- **`app-shell.tsx`**: owns the pending "open citation" request (`itemId`/`chunkId`) set by
  `StudyChatScreen`'s `onOpenCitation`; expands the panel if it is currently collapsed; passes the
  request down to `study-sources-panel.tsx`; clears the pending request when the active session
  changes (spec 2.18 decision 11).
- **A citation to a removed document** (spec 2.18 decision 10) needs **no new code** here:
  `source-viewer.tsx` already detects `ErrSourceNotFound` (message-text match, PR 1) and shows
  "This source was removed from the session" — once `AppShell` routes a citation's `itemId` into
  the same viewer, this state is reachable for free. Worth one end-to-end test confirming it,
  not new production code.
- The error-discrimination convention this all depends on (plain Go-error message-text matching,
  not a status-discriminated DTO) was already decided and implemented in PR 1's `source-viewer.tsx`
  — carry it forward, don't relitigate it.

## Docs

- `CHANGELOG.md`: a new `[Unreleased] > Added` entry for this PR, same tone/format as the PR 1
  entry already there.
- `frontend/src/lib/documentation.ts`: add the citations + viewer explanation (read the file first
  — its current shape/sections were never inspected this round).
- Mark this spec's own task checklist (below) done, and spec 2.18's PR 2 checklist
  (`18-notebooklm-style-citations.md`'s "Tasks" section) done.
- Spec 2.17 (`17-session-sources-panel.md`) has a line noting "citation → panel" as a follow-up —
  find and mark it delivered.

## Tasks

- [ ] Backend: `StudySourceResult` chunkId/itemId/excerpt; regenerate `wailsjs`
- [ ] Backend: `contextEntry.id`, the citation instruction, history marker-stripping
- [ ] Frontend: `lib/citations.ts` (+ module augmentation), `CitationChip`
- [ ] Frontend: `message-bubble.tsx`, `local-sources-strip.tsx`, `StudyChatScreen.tsx` wiring
      (including the `lib/study.ts` `StudySource` fields and the resulting test-literal fallout)
- [ ] Frontend: read `app-shell.tsx`; design and implement the open-citation request flow,
      `study-sources-panel.tsx`'s external-open prop, `source-viewer.tsx`'s highlight
- [ ] Docs: CHANGELOG, `lib/documentation.ts`, this spec + 2.18 + 2.17 checklists

## Acceptance Criteria

Same as spec 2.18's own (the citation-specific subset): a reply that used the session's documents
shows numbered citations after its claims, matching the "Local sources" strip's numbering; hovering
shows title/heading/excerpt start, including after a resume and after the source was removed;
clicking expands the panel if collapsed and opens the document at the cited passage, highlighted;
a `[n]` with no matching source, one inside a code span/block, and `[n](url)` all stay plain text;
prior assistant messages sent to the model carry no markers; Copy yields marker-free text.

## Risks (carried from spec 2.18, plus one found doing this once already)

- **Model compliance** and **marker collisions in prose** — see spec 2.18's own "Risks and open
  questions"; unchanged by this addendum.
- **A blunt `\[\d+\]` strip** (backend and frontend both) removes any bracketed number from a
  prior assistant turn even when it wasn't a citation (e.g. a hand-written "[1] step one") — from
  the text resent to the model only, never from what's displayed. Low impact, already accepted in
  PR 1's equivalent code; carried forward rather than re-litigated.
