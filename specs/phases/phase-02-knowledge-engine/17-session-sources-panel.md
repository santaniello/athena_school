# Phase 2.17 — Session Sources Panel (NotebookLM-Style Knowledge UI)

## Goal

Knowledge is managed where it is used: in the Sources panel on the right of a study
session, as in NotebookLM. The panel lists the documents the session owns, imports a new one,
and removes one.

Spec 2.14 built the panel as a disabled shell; spec 2.15 made every document owned by a
session; spec [2.16](16-remove-conversation-extraction.md) removed conversation extraction,
the item lifecycle and the whole Knowledge section (including its left-menu entry). This spec
makes the panel real, on top of that documents-only model: a session's knowledge is exactly
its imported documents, so the panel has one kind of thing to list and remove. It also closes
the gap 2.16 left on purpose — until now the UI has had no way to import, list or remove a
session's documents.

## Layout

```
┌──────────────┬──────────────────────────────┬────────────────────────┐
│ Sidebar      │ Chat                         │ Sources          [+ Add]│
│              │                              │ [ Search sources     ] │
│ ▸ Folder A   │  …messages…                  │ ┌────────────────────┐ │
│   · Session  │  ┌─ Local sources (2) ─┐     │ │ Distributed Sys    │⋮│
│   · Session  │                              │ │ notes/ds.md · 12 c │ │
│ ▸ Folder B   │                              │ ├────────────────────┤ │
│              │  [ composer ]                │ │ CAP theorem        │⋮│
│ Home Study   │                              │ │ cap.md · 4 chunks  │ │
│ Documentation│                              │ └────────────────────┘ │
└──────────────┴──────────────────────────────┴────────────────────────┘
```

Left nav (already so after 2.16): Home, Study, (locked Phase 3+ entries), Documentation,
Settings — no `Knowledge`.

## Decisions

1. **The panel lists the session's imported documents, not the sources cited so far.** The
   `messages`-derived, in-memory list from 2.14 is removed (`StudyChatScreen`'s
   `onSourcesChanged` and `AppShell`'s `sessionSources`). A document is listed from the moment
   it is imported, survives a resume with no new replies, and is what the chat can actually
   retrieve. The per-message "Local sources" strip is unchanged; it still shows what backed
   each answer.
2. **One document per Add.** The picker and `ImportFile` handle a single `.md`/`.txt` file;
   the panel reuses them as they are. Multi-select is out of scope.
3. **Import progress reuses `IngestProgressDialog`** (kind `file`, now with the session id). No
   new inline-progress UI in this increment.
4. **Remove is a hard delete of that document from this session only.** It deletes the
   session's `Item`, its chunks and its `ingested_files` record, and evicts the chunks from the
   in-memory index. It never touches the file on disk and never touches another session's copy
   of the same file. The confirmation says so.
5. **Removing a document also forgets it was imported.** So importing the same, unchanged
   file again re-ingests it instead of being skipped as "unchanged". (The old `DeleteItem`,
   deleted by 2.16, deliberately kept the `ingested_files` row — spec 2.3 — which is wrong for
   an explicit Remove, so this is a new use case built on the repository methods that stayed.)
6. **What a row shows:** title (the document's H1, falling back to its file name), the
   root-relative display path, and the chunk count. Never the absolute source path.
   Ordered oldest-imported first, so a new document appears at the bottom.
7. **The Knowledge section and the extraction UI are already gone** (2.16); nothing here
   removes or re-adds any of it.
8. **Not in this increment:** enabling/disabling a document for the chat, opening a
   citation in the panel, multi-file import, and a "changed on disk" indicator.

## New backend surface

Read model and use cases live in `application/ingest`, next to `ImportFile`, which already owns
documents and their `ingested_files` records. Bindings stay thin adapters (ADR-001).

- `domain/knowledge`: `IngestedFile` gains `IngestedAt` (the column already exists), and a
  small read model, `SessionSource{ItemID, Title, Path, ChunkCount, IngestedAt}`.
- `IngestedFileRepository.ListSourcesBySession(ctx, sessionID) []SessionSource` — `ingested_files`
  joined to `knowledge_items` on `item_id`, oldest first. A record whose item no longer exists
  is not listed (re-importing restores it, as today).
- `IngestedFileRepository.DeleteByItemID(ctx, sessionID, itemID)`, plus the existing
  `ChunkRepository.DeleteByItemID` and `Repository.Delete` that 2.16 kept.
- `ingest.Service.ListSources(ctx, sessionID)` and
  `ingest.Service.RemoveSource(ctx, sessionID, itemID)`. `RemoveSource` reserves the index like
  `ImportFile`, checks the item belongs to `sessionID` (otherwise `ErrSourceNotFound`, so one
  session can never delete another's document), then in one transaction deletes the chunks,
  the item and the `ingested_files` record, and evicts the chunk IDs from the vector store
  after commit. A failed eviction is logged, not returned — same rule as
  `DeleteSessionsWithKnowledge` (2.15).
- Bindings `ListSessionSources(sessionID)` and `RemoveSessionSource(sessionID, itemID)`;
  `wailsjs` regenerated.

## Frontend design

- **`frontend/src/lib/sources.ts`** (new): `listSessionSources`, `removeSessionSource` and the
  `SessionSource` type. Named apart from `StudySource` (a citation on a message) on purpose.
- **`useSessionSources(sessionId)`** (new hook): loads on mount and when the session changes,
  exposes `sources`, `loading`, `error`, `reload`. Ignores a response for a session that is no
  longer open. The panel owns this data; `AppShell` no longer does.
- **`study-sources-panel.tsx`**: takes `sessionId` and `mutationsDisabled` instead of `sources`.
  Header "Sources (N)" and **Add** (enabled); client-side search over title and path; a row per
  document with a `⋮` menu → **Remove**; loading, error-with-retry and empty states (the empty
  state carries its own "Add a source" action and a one-line explanation that the chat searches
  these documents).
- **Add**: `pickNotesFile()` → open `IngestProgressDialog` (`kind="file"`, `sessionId`,
  `path`) → on close, `reload()`. A cancelled picker does nothing; a rejected picker shows the
  existing inline error copy.
- **Remove**: `AlertDialog` "Remove *title*?" — "It will no longer be searched in this session.
  The file on your computer is not changed." → `removeSessionSource` → `reload()`; a failure
  keeps the row and shows the error inside the dialog.
- **`mutationsDisabled`** (the index is retrying) disables Add and Remove, exactly as the old
  Knowledge screen disabled its mutating actions, with the same reason as a tooltip.
- **`AppShell`** keeps only the panel's open/size state; it stops passing `sources` and drops
  `sessionSources` and the `onSourcesChanged` plumbing.
- **Copy**: `lib/documentation.ts` says that documents are added from a session's Sources
  panel.

## Tasks

Each slice keeps the build green and is committed on its own; frontend slices are TDD with
Vitest, backend slices with `_test.go`.

- [ ] Domain and SQLite: `IngestedAt`, `SessionSource`, `ListSourcesBySession`,
      `DeleteByItemID`; regenerate mocks
- [ ] Application: `ingest.Service.ListSources` and `RemoveSource` (ownership check, index
      reservation, transaction, post-commit eviction), plus `ErrSourceNotFound`
- [ ] Bindings `ListSessionSources`, `RemoveSessionSource`; regenerate `wailsjs`
- [ ] `lib/sources.ts` and `useSessionSources`
- [ ] Panel: real list, search and the loading/error/empty states; remove
      `onSourcesChanged`/`sessionSources` and the `messages`-derived dedupe
- [ ] Panel: Add (picker, `IngestProgressDialog` with `sessionId`, reload)
- [ ] Panel: Remove (menu, confirmation, reload, error)
- [ ] Docs: CHANGELOG, README, `lib/documentation.ts`, and mark spec 2.14's placeholders as
      delivered

## Acceptance Criteria

- Opening a session shows its imported documents in the panel — including right after a resume,
  with no replies yet.
- **Add** imports a `.md`/`.txt` file into the open session; the panel lists it when the dialog
  closes, and the next chat turn can cite it. Adding the same, unchanged file again reports it
  as skipped; adding it after a **Remove** re-imports it.
- **Remove** deletes only that session's copy: the document disappears from the panel and from
  retrieval, its chunks are gone from SQLite and the in-memory index, the same file in another
  session is untouched, and a document id from another session is rejected.
- Add and Remove are disabled, with a reason, while the index is retrying.
- Deleting a session or folder still removes its documents (2.15).
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code; frontend tests, coverage, lint, typecheck, and Stryker on the
  changed files pass.

## Out of scope

- Anything about conversation extraction, statuses, evidence, reconciliation or duplicates —
  gone since 2.16.
- Renaming `Item` to `Source`.
- Per-document enable/disable, citation → panel navigation, multi-file import.
