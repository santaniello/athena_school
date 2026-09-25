# Phase 2.15 — Session-Scoped Knowledge

## Goal

Every piece of knowledge belongs to exactly one study session (which itself belongs to one
folder). Retrieval in a session only sees that session's knowledge (NotebookLM-style
"notebook per session"), and deleting a session or a folder deletes its knowledge items,
chunks, ingested-file records and in-memory index entries.

This lands the ownership model that spec 2.14 deferred. The right-hand Sources panel stays a
disabled shell: import and management from the UI come in a later increment.

## Decisions

- **No backward compatibility.** The app is not in production and its data was wiped, so the
  migration recreates the knowledge tables with the final schema and discards any rows.
- **Ownership:** `session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE` on
  `knowledge_items`, `knowledge_chunks`, `ingested_files` and
  `knowledge_reconciliation_proposals`. The folder → sessions → knowledge cascade comes from
  the existing per-connection `foreign_keys` pragma. `knowledge_item_evidence` and
  `knowledge_item_relations` already cascade from `knowledge_items`.
- **Retrieval scope:** current session only. `SearchFilters` gains `SessionID`.
- **`ingested_files` key** becomes `(session_id, source_path)`: the same file can be imported in
  several sessions, producing independent chunk sets.
- **Import:** backend only. `ImportFile` requires a `sessionID`; the global "Import notes"
  button leaves the Knowledge screen. Sources panel Add/Remove remain "Coming soon".

## What the FK does not cover

1. **In-memory vector index (ADR-004):** chunk IDs are collected before the delete and evicted
   with `VectorStore.Remove` after commit. A failed eviction is a warning, never a rollback;
   retrieval's existing orphaned-item drop remains the safety net.
2. **Orphaned `knowledge_evidence`:** `EvidenceRepository.DeleteUnreferenced` runs in the same
   transaction as the session delete.
3. **Layering:** `study` and `folder` reach knowledge through a small interface defined in the
   consuming package, implemented by the knowledge service (ADR-001).

## Tasks

- [x] Domain: `SessionID` on `Item`, `Chunk`, `IngestedFile`, `SearchFilters`;
      `ErrSessionRequired` (see Implementation notes for why `Item.Validate` does not enforce it)
- [x] SQLite: guarded migration recreating knowledge tables; repositories persist/filter
      `session_id`; `IngestedFileRepository.ListBySession`; `ChunkRepository.ListIDsBySession`;
      cascade tests
- [x] Vector store: `Search` honors `filters.SessionID`
- [x] Retrieval: `Retrieve` scopes the search to its `sessionID`
- [x] Ingest: `ImportFile(sessionID, …)` rejects a blank session and stamps ownership; binding
      and generated Wails bindings updated
- [x] Extraction/reconciliation/duplicates: saved items inherit the batch's session; lookups
      restricted to the same session (split into 15-01 if this grows)
- [x] Delete: `study.DeleteSession` and `folder.DeleteFolder` purge evidence and evict the index
- [x] Frontend: remove global "Import notes"; delete dialogs mention knowledge removal
- [x] Docs: CHANGELOG `[Unreleased]`, README if the import flow is described

## Implementation notes

Where the shipped code differs from the design above:

- `Item.Validate` does **not** require `SessionID`. Every write path stamps it and the
  `NOT NULL` + foreign key constraints are the backstop, so a domain rule would only have
  forced churn on unrelated tests. `ImportFile` alone rejects a blank session
  (`ErrSessionRequired`), before reserving the index.
- `ImportFile` does not look the session up first; that the session exists is enforced by
  the foreign key on the rows it writes. The trade-off is that a bad session id surfaces
  after the embedding calls rather than before.
- The cascade lives in one use case, `knowledge.Service.DeleteSessionsWithKnowledge(ctx,
  sessionIDs, deleteSessions)`, which wraps the caller's delete: chunk IDs are read first
  (they vanish with the sessions), orphaned evidence is cleaned in the same transaction,
  and the index is evicted after commit. `study` and `folder` each declare a
  `KnowledgeCascade` interface for it (consumer side). `DeleteFolder` deletes the folder
  row after the wrapped call because `FolderRepository.Delete` does not join a transaction
  (with a one-connection pool it would deadlock inside one).
- A failed post-commit eviction is logged, not returned: the sessions are gone and
  retrieval is scoped by session, so a leftover in-memory chunk is unreachable.
- Reconciliation proposals take their owner from `Candidate.SessionID`, which the server
  stamps from the receipt.
- `IngestProgressDialog` and `lib/ingest.importFile` stay (now taking a `sessionId`) even
  though nothing in the UI opens a `file` import until the Sources panel does.

## Acceptance Criteria

- Retrieval in session A never returns a chunk owned by session B.
- Deleting a session removes its items, chunks, ingested files, orphaned evidence and index
  entries; deleting a folder does the same for every session in it.
- The same file imported in two sessions yields two independent knowledge sets.
- A failed index eviction is logged and does not undo the delete.
- Go coverage ≥ 80%, no surviving mutants in changed domain/application/vectorstore code, and
  the frontend suite passes.

## Out of scope

Deprecating `ExtractFromSession`, removing `Knowledge` from the left nav, reading
`MessageSourceRepository.ListBySession` in the panel, and functional Add/Remove in the panel.
The Knowledge Explorer keeps listing every item across sessions until the panel takes over
management.
