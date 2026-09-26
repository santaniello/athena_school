# Phase 2.18 — NotebookLM-Style Citations (Inline Citations and Source Viewer)

## Goal

An answer that used the session's documents says exactly where each claim came from, and the
reader can check it in one click, as in NotebookLM:

- the reply carries **numbered citations** inline (`…the scheduler multiplexes M:N goroutines [2].`);
- **hovering** a citation previews the passage it points to;
- **clicking** it opens that document in the Sources panel, scrolled to the passage and
  highlighted;
- a row in the Sources list opens the same document viewer, so the panel is also where a
  document is read, not only where it is managed.

Today the backend already retrieves passages and every reply carries its sources (a file path,
a heading, the excerpt and a score), shown in a collapsed "Local sources" strip under the
message. What is missing is the link between a sentence in the reply and one of those passages,
and any way to see a passage in its document. Spec [2.17](17-session-sources-panel.md) makes
the panel list, add and remove documents; this spec makes it a reader and wires the chat to it.

**Depends on 2.17** (the real panel, `RemoveSource`, `ingested_files` cleared on removal) and on
[2.16](16-remove-conversation-extraction.md) (documents are the only source of knowledge).

## Layout

```
┌──────────────┬──────────────────────────────┬────────────────────────┐
│ Sidebar      │ Chat                         │ ← Sources   ds.md      │
│              │                              │ Distributed Sys        │
│              │  The scheduler multiplexes   │ ┌────────────────────┐ │
│              │  M:N goroutines [2]. ◄───────┼─┤ ## Scheduler        │ │
│              │        ┌──────────────┐      │ │ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ │ │
│              │        │ 2 · ds.md    │      │ │ ▓ The runtime     ▓ │ │
│              │        │ Scheduler    │      │ │ ▓ multiplexes M   ▓ │ │
│              │        │ "The runtime │      │ │ ▓ goroutines on N ▓ │ │
│              │        │  multiplex…" │      │ │ ▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓ │ │
│              │        └──────────────┘      │ │ Channels are …      │ │
│              │  [ composer ]                │ └────────────────────┘ │
└──────────────┴──────────────────────────────┴────────────────────────┘
        hover [2] → preview            click [2] → viewer, passage highlighted
```

The panel has two views: the **list** from 2.17 and the **viewer** described here, with a
`← Sources` button to go back.

## Decisions

1. **Citation syntax is `[n]`, and `n` is the passage's position in that reply's source list.**
   `n` is 1-based and equals `message_sources.position + 1` — the same order as the "Local
   sources" strip today. No new column and no new table are needed to tie a marker to a source:
   the persisted `message_sources` row (which already holds `chunk_id`, `item_id`, the heading
   and the excerpt) is the target. Numbers are per reply, not per session, so `[1]` in one reply
   and `[1]` in another usually name different passages.
2. **The model is asked to emit the markers; Go never trusts them.** Each entry of the JSON
   context block sent to the model gains an `id` (its `n`), and the retrieval framing tells the
   model to cite the passages it used with `[n]` right after the claim, and never to invent a
   number. On the client, a `[n]` becomes a citation only when that reply actually has an `n`-th
   source; anything else stays as plain text. A marker is only ever a link to a source of the
   *same* message, so a marker forged by text inside a document (prompt injection) cannot
   point anywhere new. A reply with no markers (the model ignored the instruction, `web` mode, a
   reply from before this spec) simply keeps today's behavior: no chips, the strip only.
3. **Markers are stripped from the history sent back to the model.** Numbers in an older
   assistant message refer to that message's context, not to the current one, and would invite
   wrong citations. The stored message keeps its markers (that is what the user reads); the
   prompt builder removes them from prior assistant turns.
4. **Hover uses the persisted excerpt; click needs the document.** The preview is built from the
   `message_sources` row, so it works after a resume and even when the document has since been
   removed. Opening the viewer needs the document text, so it goes through the backend.
5. **The viewer needs the whole document, so ingestion now stores it.** Today only chunks are
   stored, and a chunk's text is not a literal slice of the file (paragraph gaps are normalized
   when paragraphs are packed), so a passage cannot be found in the file by searching for its
   text — and the file may have moved or changed since. Ingestion therefore also stores:
   - the document's full text, in a new table `knowledge_documents(item_id PRIMARY KEY,
     session_id NOT NULL REFERENCES sessions(id) ON DELETE CASCADE, content NOT NULL)`, kept
     apart from `knowledge_items` so item reads (retrieval touches them) never drag the text
     along;
   - for each chunk, where it sits in that text: `start_offset` / `end_offset` (byte offsets of
     the source text, at UTF-8 boundaries), computed by the chunker.
6. **The chunker records offsets; chunk content does not change.** `ChunkCandidate` gains `Start`
   and `End`. Heading sections take the offsets of their trimmed slice; packed paragraphs take the
   first paragraph's start and the last one's end; merging undersized pieces takes the first
   piece's start and the second's end. Chunks stay ordered and non-overlapping, and `Content`
   (what is embedded and cited) is byte-for-byte what it is today, so no embedding is
   invalidated.
7. **The backend returns the document already cut into segments.** Highlighting must not depend on
   Go byte offsets meaning anything to JavaScript (whose strings index UTF-16 units), so the
   backend slices the text at chunk boundaries and returns ordered `{text, chunkId}` segments;
   text between chunks (trimmed whitespace) has an empty `chunkId`. The client highlights the
   segment whose `chunkId` is the cited one and never does offset arithmetic.
8. **Documents ingested before this spec have no stored text and degrade, not break.** The
   migration is additive (a new table, new columns): nothing is discarded. Such a document opens
   in the viewer as "Re-import this document to open it here", its citations still preview, and
   `ImportFile` treats an item with no stored text as not up to date, so importing the same
   unchanged file again re-ingests it instead of being skipped by its modification time.
9. **Removing a document removes its stored text too**, in the same transaction as the chunks, the
   item and the `ingested_files` record (2.17's `RemoveSource`); re-importing replaces the stored
   text and chunk offsets together with the chunks.
10. **A citation to a removed document is still shown, and says so.** Hover still previews the
    persisted excerpt; click shows an inline "This source was removed from the session" instead
    of opening the viewer. Nothing is deleted from old messages.
11. **The panel is controlled from the chat through `AppShell`.** `StudyChatScreen` reports "open
    citation (itemId, chunkId)"; `AppShell`, which already owns the panel's collapse state and
    ref, expands the panel if it is collapsed and passes the request down. A row click in the
    list opens the same viewer without a highlighted passage.
12. **Copying a reply copies clean text.** The message's copy button strips the markers, so
    pasted text has no stray `[n]`.
13. **The viewer shows the document as text.** Plain text with line breaks preserved, for both
    `.md` and `.txt`; rendering Markdown inside the viewer is a later refinement (highlighting
    over rendered HTML is a different problem).
14. **Sources need stable ids on the client.** `StudySource` (frontend) and its DTO gain `chunkId`
    and `itemId`, which the domain `Source` and `message_sources` already carry. This also lets
    `sourceKey`'s "unique enough" tuple stop being the identity of a source.

## New backend surface

Chunker and ingestion changes live in `application/ingest`; the reader lives next to
`ListSources`/`RemoveSource` (2.17). Bindings stay thin adapters (ADR-001).

- `application/ingest/chunking.go`: `ChunkCandidate.Start/End` as in decision 6.
- `domain/knowledge`: `Chunk` gains `StartOffset`/`EndOffset`; new `DocumentRepository` port
  (`Save(ctx, itemID, sessionID, content)`, `Get(ctx, sessionID, itemID)`,
  `DeleteByItemID(ctx, sessionID, itemID)`); read model
  `SourceDocument{ItemID, Title, Path, Segments []DocumentSegment}` and
  `DocumentSegment{Text, ChunkID}`.
- SQLite: `knowledge_documents`, `knowledge_chunks.start_offset`/`end_offset` (nullable, so
  pre-2.18 rows load), the repository, and a migration step that adds them without touching
  existing rows.
- `ingest.Service.ImportFile`: stores the text and offsets in the same transaction as the chunk
  replacement; treats an item with no stored text as stale (decision 8).
- `ingest.Service.GetSourceDocument(ctx, sessionID, itemID)`: checks the item belongs to
  `sessionID` (otherwise `ErrSourceNotFound`), loads text and chunks, returns the segments; a
  document with no stored text returns `ErrSourceTextUnavailable`.
- `ingest.Service.RemoveSource` also deletes the stored text (decision 9).
- `application/knowledge` retrieval: `contextEntry` gains `id`; `application/study`: the
  retrieval framing asks for `[n]` citations (both source modes that use the context) and the
  history builder strips markers from prior assistant messages.
- Bindings: `GetSessionSourceDocument(sessionID, itemID)`; `StudySourceResult` gains `chunkId` and
  `itemId`; `wailsjs` regenerated.

## Frontend design

- **`lib/citations.ts`** (new): a remark plugin that turns `[n]` in text nodes into citation
  nodes — skipping code spans, code blocks and `[n](…)` link syntax — plus the pure helper that
  strips markers (used by Copy).
- **`CitationChip`** (new): a small superscript-style chip with a hover card (number, document
  title, heading, first ~300 characters of the excerpt). Renders as plain text when the reply has
  no `n`-th source.
- **`MessageBubble`** takes the reply's `sources` and `onOpenCitation(index)`. While a reply is
  streaming it uses the sources the turn already announced (`study:sources` is always emitted
  before the first `study:chunk`), so chips resolve as their text arrives.
- **`LocalSourcesStrip`**: each entry shows its number and is clickable, opening the same viewer.
- **`lib/sources.ts`**: `getSessionSourceDocument` and the `SourceDocument`/`DocumentSegment`
  types. **`useSourceDocument(sessionId, itemId)`** loads on demand and ignores a response for a
  document that is no longer open.
- **`source-viewer.tsx`** (new): header with `← Sources`, the title and the display path; the
  segments in order, the cited one highlighted and scrolled into view (and briefly emphasized);
  loading, error-with-retry, "removed" and "re-import to open" states.
- **`study-sources-panel.tsx`**: switches between the list and the viewer from a `view` state fed
  by `AppShell`; a row click opens the viewer.
- **`AppShell`**: owns the pending "open citation" request, expands the collapsed panel and clears
  the request when the session changes.
- **Copy** in `MessageBubble` strips markers; **`lib/documentation.ts`** explains citations and
  the viewer.

## Tasks

Each slice keeps the build green and is committed on its own; backend slices are TDD with
`_test.go`, frontend slices with Vitest.

- [ ] Chunker: `Start`/`End` on `ChunkCandidate` for headings, packed paragraphs, merges and the
      plain-text fallback (pure, no persistence yet)
- [ ] Domain and SQLite: `Chunk` offsets, `DocumentRepository`, `knowledge_documents`, additive
      migration; regenerate mocks
- [ ] Ingest: `ImportFile` stores text and offsets, treats a text-less item as stale;
      `RemoveSource` deletes the text
- [ ] Reader: `ingest.Service.GetSourceDocument` (ownership, segments, `ErrSourceNotFound`,
      `ErrSourceTextUnavailable`) and the `GetSessionSourceDocument` binding; `chunkId`/`itemId`
      on `StudySourceResult`; regenerate `wailsjs`
- [ ] Prompting: context `id`s, the `[n]` instruction, markers stripped from history
- [ ] Frontend: `lib/citations.ts`, `CitationChip`, `MessageBubble` wiring, streaming, Copy
- [ ] Frontend: `lib/sources.ts` additions, `useSourceDocument`, `source-viewer.tsx`, panel views,
      `AppShell` request flow, strip entries clickable
- [ ] Docs: CHANGELOG, README, `lib/documentation.ts`, and mark 2.17's "citation → panel" line
      as delivered

## Acceptance Criteria

- A reply that used the session's documents shows numbered citations after its claims; the
  number matches the entry of the same number in the "Local sources" strip.
- Hovering a citation shows the document title, heading and the start of the passage — also
  after leaving the session and resuming it, and also when the document was removed since.
- Clicking a citation expands the Sources panel if collapsed and opens that document at the
  cited passage, highlighted and in view; `← Sources` returns to the list.
- Clicking a row in the list opens the document without a highlighted passage.
- A `[n]` with no `n`-th source, a marker inside a code span or block, and `[n](url)` stay plain
  text; a reply with no markers looks and behaves as before.
- Prior assistant messages sent to the model contain no markers.
- Copying a reply yields text without markers.
- A document imported before this spec opens as "re-import to open it here"; importing the same,
  unchanged file again re-ingests it and the viewer then works.
- Removing a document also removes its stored text; a citation to it previews from the persisted
  excerpt and clicking it explains the source was removed; another session's document id is
  rejected by `GetSourceDocument`.
- Chunk `Content` and embeddings are unchanged by the chunker's offset tracking, and the segments
  of any document reassemble to its stored text exactly.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code (the chunker is mutation-tested); frontend tests, coverage, lint,
  typecheck and Stryker on the changed files pass.

## Risks and open questions

- **Model compliance.** Citing is a request, not a guarantee; a model may skip markers, cite
  the wrong `n`, or put `[n]` where it is not needed. The design degrades to today's strip, but
  how often the markers show up has to be checked against the models the tiers route to before
  this is called done; if `notes` mode is too unreliable, the instruction may need to be firmer
  there than in `strict-notes`.
- **Document size.** The full text is now stored and sent to the viewer, and `ImportFile` has no
  size limit today (neither the ingest service nor the file-picker binding checks one). A cap
  on the file, or on what the viewer loads, must be settled before the reader ships; this spec
  does not pick a number.
- **Marker collisions in prose.** A model's own footnote-style `[1]` in `web` mode has no sources
  and stays text; in `notes` mode it would link to source 1. The framing tells the model to use
  `[n]` only for the provided passages.
- **Merged chunks span more than one heading.** A highlighted passage can cover the tail of one
  section and the start of the next, because undersized pieces are merged; the viewer highlights
  the whole chunk, exactly what the model saw.

## Out of scope

- Per-document enable/disable, multi-file import, PDF/web/YouTube sources (2.17's list and
  beyond).
- Rendering Markdown inside the viewer, search inside the viewer, and a "changed on disk"
  indicator.
- A source guide or summary of a document, suggested questions, and notes/audio features of
  NotebookLM.
- Citation ranges (`[1-3]`) and grouped citations; one marker per passage.
- Renaming `Item` to `Source`.
