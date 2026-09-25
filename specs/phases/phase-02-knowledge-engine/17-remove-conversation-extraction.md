# Phase 2.17 — Remove Conversation Extraction (Documents-Only Knowledge)

## Goal

Knowledge comes from one place only: documents the user imports into a study session,
as in NotebookLM. The "Extract knowledge" action, the draft → approved → deprecated
lifecycle, and everything that exists to support them are removed.

Spec 2.14 already recorded this as the agreed product direction ("Deprecate
session-extraction entirely"); spec 2.15 made every piece of knowledge owned by a session.
This increment deletes the second source of knowledge, so what remains is a single, simple
model: a session owns its imported documents, and retrieval searches their chunks.

This is the backend half. Spec [2.16](16-session-sources-panel.md) ships first: it makes
the Sources panel functional and removes every UI entry point to conversation extraction (the
composer button, the Knowledge section and its nav entry). By the time this spec runs, nothing
in the UI calls what it deletes.

## Why

- An imported document is the source itself; there is nothing for the user to review. An
  extracted item is LLM output that can be wrong, which is the only reason drafts, review,
  evidence, duplicate detection and reconciliation exist.
- `status` no longer carries meaning: imported documents are created `approved` and
  retrieval searches only `approved` chunks, so it is an always-true filter.
- It deletes a large amount of code and its test/mutation surface (extraction, receipts,
  reconciliation, duplicates, evidence, relations, review UI, indexing backfill).

## Decisions

1. **The UI is already gone.** 2.16 removed the `Knowledge` nav entry, the Explorer, the
   Review tab, the topic tree, the extraction dialog and the "not indexed" alerts. This spec
   deletes only what they used to call.
2. **Editing an item's concept/definition is gone with the Explorer.** A source is a
   document, not editable prose.
3. **`user_note` is removed.** Nothing produces it; it is only a constant and a label.
4. **`Item` is not renamed to `Source` here.** The shadow `Item` stays the owner of a
   document's chunks. Renaming is a separate, mechanical follow-up so this spec stays a
   pure deletion.
5. **Use cases and bindings with no caller after this change are deleted, including
   `DeleteItem`.** The panel's Remove is `ingest.Service.RemoveSource` (2.16), which also
   clears `ingested_files`; today's `DeleteItem` deliberately does not (spec 2.3).
6. **Existing imported documents survive the migration; everything extracted does not**
   (rows with `source` other than `imported_doc`, and their chunks).

## What is removed

**Frontend**: nothing here except the Settings field (2.16 removed the rest): the
`MaxKnowledgeExtractionItems` control in Settings and its wrapper, since the setting only
ever bounded extraction.

**Wails bindings** (`interfaces/desktop`): `ExtractKnowledge`, `SaveExtractedKnowledge`,
`SaveAndApproveExtractedKnowledge`, `DiscardExtraction`, the whole reconciliation family
(immediate and pending), `Approve`/`Deprecate`/`UpdateKnowledgeItem`, `ListKnowledgeItems`,
`ListKnowledgeTopics`, `ListKnowledgeItemEvidence`, `DeleteKnowledgeItem`, and the
reindex/backfill bindings. Generated `wailsjs` bindings are regenerated.

**Application** (`application/knowledge`): `extraction.go`, `parse.go`, `prompt.go`,
`receipt_store.go`, `reconcile.go`, `reconcile_pending.go`, `duplicates.go`, `approve.go`,
`deprecate.go`, `update.go`, `list.go`, `review.go`, `backfill.go`, `indexing.go`
(`indexKnowledgeItem` embeds *extracted* items) and `DeleteItem`. `Service` sheds the
collaborators only they used (sessions, messages, configs, evidence, reconciliations,
relations, duplicate thresholds), and `NewService` shrinks accordingly.
`domainllm.TaskKnowledgeExtraction` and its routing go.

**Domain** (`domain/knowledge`): `evidence.go`, `reconciliation.go`, `relation.go`,
`duplicate.go`, `NormalizeConcept`; `Status` and its transitions on `Item` and `Chunk`;
`SourceAthena` and `SourceUserNote`; the `Topic`/`Source`/`Status` members of
`SearchFilters` (only `SessionID` remains) and of `Filter`. Any other field or method left
without a caller is deleted with them — the compiler and `deadcode` decide.

**SQLite**: drop `knowledge_evidence`, `knowledge_item_evidence`, `knowledge_item_relations`,
`knowledge_reconciliation_proposals`, `knowledge_reconciliation_evidence`; delete the
non-imported items and their chunks; drop `knowledge_items.status`,
`knowledge_items.normalized_concept` and `knowledge_chunks.status` together with the indexes
that use them; drop the now-unused evidence/proposal/relation repositories. `reset.go` and
its test stop mentioning the dropped tables. The chunk load query
(`chunkLoadCurrentQuery`) keeps checking that the owning item exists and the embedding
model matches, and drops its status/source/topic mismatch reasons.

## What stays

- Imported documents end to end: `ImportFile`, chunking, embeddings, the shadow `Item`,
  `ingested_files`, the vector store and `IndexLoader` (index readiness and retry).
- Session ownership and the cascade from 2.15, including `DeleteSessionsWithKnowledge`
  — minus the evidence cleanup step, which has nothing left to clean.
- `Retrieve` and the study source modes; `message_sources` and the persisted "Local sources"
  strip. Rows written before this change that name an extracted item keep rendering from
  their own stored columns; they point at nothing after the migration and that is fine.

## Consequences for later phases

These specs assume extracted, approved knowledge items and must be revisited before they
are implemented; this spec does not rewrite them:

- Phase 3 `05-flashcards.md` (spaced repetition "of your approved knowledge") — flashcards
  would have to be generated from documents/chunks instead.
- Phase 3 `06-knowledge-promotion.md` (promoting challenge results into knowledge).
- Phase 7 `01-knowledge-graph.md` (built on Item relations).
- `specs/Athena.md` and `specs/Planning.md` sections that describe extraction and review.

Specs 2.2 (extraction), 2.7 (review), 2.9–2.12 (evidence, duplicates, reconciliation,
revision history) and 2.13 (canonical topic identity) are marked **Superseded by 2.17**
at their top rather than deleted; they remain the record of what was built and why.

## Tasks

Each slice keeps the build green and is committed on its own.

- [ ] Precondition: 2.16 is merged — no UI calls extraction, review, approval, reconciliation
      or reindex
- [ ] Settings: remove `MaxKnowledgeExtractionItems` (config field, store, binding, screen,
      test); an existing `config.yaml` that still has the key must keep loading
- [ ] Backend, extraction: extraction, receipts, parsing, prompt, evidence and their
      bindings; `TaskKnowledgeExtraction`
- [ ] Backend, reconciliation: reconciliation (immediate and pending), duplicates, relations
      and their bindings and repositories
- [ ] Backend, lifecycle: approve/deprecate/update/list/review/backfill/indexing/`DeleteItem`
      and their bindings; shrink `Service` and `NewService`
- [ ] Domain and SQLite: drop status/source/topic filters and fields; migration; repository
      and reset cleanup; regenerate mocks and Wails bindings
- [ ] Docs: mark the superseded specs, update `Athena.md`, `Planning.md`, README,
      CHANGELOG (breaking, see below)

## Acceptance Criteria

- Importing a document into a session still works end to end and the chat still cites it;
  a session's chat never retrieves another session's chunks.
- No extraction, draft/approve/deprecate or reconciliation remains in the Wails bindings
  (the UI already lacks them after 2.16).
- Opening an existing database drops the extracted items and the dropped tables, keeps
  imported documents and their chunks, and passes `PRAGMA foreign_key_check`; reopening
  does not repeat the migration.
- Deleting a session or a folder still removes its documents, chunks, ingested files and
  index entries.
- An existing `config.yaml` containing `max_knowledge_extraction_items` still loads.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application/vectorstore code, and the frontend suite, lint and typecheck pass.
  Removing code must not lower the coverage ratio below the threshold; if it does, the
  remaining tests are strengthened, not new dead code added.

## Breaking change

This drops user data (extracted items, evidence, proposals, relations) and removes a
feature, so it is breaking in the sense of AGENTS.md's versioning rules. The version is
bumped manually at release; the CHANGELOG entry under `[Unreleased]` is written as
**Removed** and states that extracted knowledge is discarded by the migration.

## Out of scope

- Renaming `Item` to `Source` (and the related field/table cleanup) — a follow-up.
- The functional Sources panel (2.16).
- Redesigning Phase 3/7 features that depended on extracted knowledge.
