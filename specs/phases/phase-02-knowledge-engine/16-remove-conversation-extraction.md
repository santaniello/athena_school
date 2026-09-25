# Phase 2.16 — Remove Conversation Extraction (Documents-Only Knowledge)

## Goal

Knowledge comes from one place only: documents the user imports into a study session,
as in NotebookLM. The "Extract knowledge" action, the draft → approved → deprecated
lifecycle, and everything that exists to support them are removed — UI and backend together,
in one vertical increment.

Spec 2.14 already recorded this as the agreed product direction ("Deprecate
session-extraction entirely"); spec 2.15 made every piece of knowledge owned by a session.
This increment deletes the second source of knowledge, so what remains is a single, simple
model: a session owns its imported documents, and retrieval searches their chunks.

Spec [2.17](17-session-sources-panel.md) then builds the functional Sources panel on top of
that model. Doing this first means the panel's Remove is just "remove a document" — there is
no extracted-item case, no evidence to preserve and no review state to reason about.

## Why

- An imported document is the source itself; there is nothing for the user to review. An
  extracted item is LLM output that can be wrong, which is the only reason drafts, review,
  evidence, duplicate detection and reconciliation exist.
- `status` no longer carries meaning: imported documents are created `approved` and
  retrieval searches only `approved` chunks, so it is an always-true filter.
- It deletes a large amount of code and its test/mutation surface (extraction, receipts,
  reconciliation, duplicates, evidence, relations, review UI, indexing backfill).

## Decisions

1. **The whole Knowledge section goes**: the `Knowledge` nav entry, the Explorer, the
   Review tab, the topic tree and the "not indexed" alerts. There is nothing left in them
   to show. Until 2.17 there is no UI to list, import or remove a session's documents; import
   has already been absent from the UI since 2.15. This is a deliberate gap in a personal,
   not-yet-productive app.
2. **Editing an item's concept/definition goes with the Explorer.** A source is a document,
   not editable prose.
3. **`user_note` is removed.** Nothing produces it; it is only a constant and a label.
4. **`Item` is not renamed to `Source` here.** The shadow `Item` stays the owner of a
   document's chunks. Renaming is a separate, mechanical follow-up so this spec stays a
   pure deletion.
5. **Use cases and bindings with no caller after this change are deleted, including
   `DeleteItem`.** 2.17 adds a session-scoped Remove that also clears `ingested_files`
   (today's `DeleteItem` deliberately does not — see spec 2.3). The repository methods it
   will build on (`Repository.Delete`, `ChunkRepository.DeleteByItemID`) stay.
6. **The migration discards all existing knowledge, imported documents included.** Nothing
   is deployed anywhere it must survive and the only database in use holds no imported
   documents, so there is no copy path. This is the same call spec 2.15 made
   (`migrateKnowledgeToSessionOwnership`), and the migration follows its shape: rebuild the
   tables in one transaction, guarded by a `hasColumn` check so it runs once. Sessions,
   messages and `message_sources` are untouched.

## What is removed

**Frontend**
- Composer's `Extract knowledge` button and `knowledge-extraction-dialog`, and
  `handleExtractKnowledge` in `StudyChatScreen`.
- `knowledge-section`, `KnowledgeExplorerScreen`, `knowledge-topic-tree`,
  `knowledge-delete-dialog`, `pending-reconciliation-section`, `reconciliation-decision-row`,
  `reconciliation-decision.ts` and the Knowledge badge/draft counts and topic selection owned
  by `AppShell` (and `NavItem`'s `badge` prop, which had no other caller).
  `IngestProgressDialog` loses its `reindex` kind and the `kind` prop (its only caller was
  the Explorer's alert) and becomes file-import only.
- The `knowledge` entry in `lib/navigation.ts` and its `AppSection`. The active section is
  plain in-memory state that always starts on `home`, so nothing persisted needs a fallback.
- `index-review-dialog` **stays**. It is not a reindex flow: it lists the chunks the index
  load isolated (`IndexStatusBanner`'s Review action). Its copy for the source/topic/status
  mismatch reasons is trimmed in the domain/SQLite slice, when the backend stops emitting them.
- The `MaxKnowledgeExtractionItems` control in Settings, and the extraction/review copy in
  `lib/documentation.ts`.
- The extraction/review/approval/reconciliation/reindex functions in `lib/knowledge.ts`.
  What the index-status screens and the study Sources panel still use stays.

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
`domainllm.TaskKnowledgeExtraction` and its routing go, and so does
`MaxKnowledgeExtractionItems` in the config domain, store and desktop bindings (see the
Settings task).

**Domain** (`domain/knowledge`): `evidence.go`, `reconciliation.go`, `relation.go`,
`duplicate.go`, `NormalizeConcept`; `Status` and its transitions on `Item` and `Chunk`;
`SourceAthena` and `SourceUserNote`; the `Topic`/`Source`/`Status` members of
`SearchFilters` (only `SessionID` remains) and of `Filter`. Any other field or method left
without a caller is deleted with them — the compiler and `deadcode` decide.

**SQLite**: drop `knowledge_evidence`, `knowledge_item_evidence`, `knowledge_item_relations`,
`knowledge_reconciliation_proposals`, `knowledge_reconciliation_evidence`; recreate
`knowledge_items` and `knowledge_chunks` empty, without `status` and (on items)
`normalized_concept`, and without the indexes that used them; empty `ingested_files`, whose
rows point at the discarded items (see decision 6); drop the now-unused
evidence/proposal/relation repositories. Deleting the old tables already cascades to their
children, as in 2.15. `reset.go` and
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
- The Sources panel exactly as 2.14/2.15 left it (a disabled shell listing cited sources);
  2.17 replaces it.

## Consequences for later phases

These specs assume extracted, approved knowledge items and must be revisited before they
are implemented; this spec does not rewrite them:

- Phase 3 `05-flashcards.md` (spaced repetition "of your approved knowledge") — flashcards
  would have to be generated from documents/chunks instead.
- Phase 3 `06-knowledge-promotion.md` (promoting challenge results into knowledge).
- Phase 7 `01-knowledge-graph.md` (built on Item relations).
- `specs/Athena.md` and `specs/Planning.md` sections that describe extraction and review.

Specs 2.2 (extraction), 2.7 (review), 2.9–2.12 (evidence, duplicates, reconciliation,
revision history) and 2.13 (canonical topic identity) are marked **Superseded by 2.16**
at their top rather than deleted; they remain the record of what was built and why.

## Tasks

Each slice keeps the build green and is committed on its own. UI slices go first, so the
backend they used to call has no caller left when it is deleted.

- [x] Composer: remove the `Extract knowledge` button, dialog and wiring
- [x] Remove the Knowledge section: nav entry, Explorer, Review, topic tree, delete dialog,
      reindex dialog and the `reindex` kind, badges, and the `AppShell` state that fed them
- [x] Settings: remove the Settings control, `lib/knowledge.ts`, the
      `Get`/`UpdateKnowledgeExtractionSettings` bindings and their tests; regenerate the
      Wails bindings. The `Config` field cannot go yet: `ExtractFromSession` still reads it,
      so it is removed with the extraction slice below
- [x] Backend, extraction and immediate reconciliation: extraction, receipts, parsing, prompt,
      duplicate detection, the immediate reconciliation actions (`Apply*`, `Resolve*`,
      `Acknowledge*`, `SaveReconciliationForReview`) and the classifier, with their bindings;
      `TaskKnowledgeExtraction` and `TaskKnowledgeReconciliation`. Receipts are shared by
      extraction and the immediate reconciliation, and `FindDuplicates` has no caller besides
      them, so none of these can go alone. Helpers the pending reconciliation still uses
      (`createReconciledItem`, `updateReconciledItem`, `checkReconciliationTargetFresh`,
      `reasonWithResolution`) stay until the next slice, and so do the pieces they lean on:
      `findExactDuplicates`, `truncateString`, `normalizeList` and the size limits (moved
      into `reconcile.go` when `parse.go` went). The `Service` collaborators only extraction
      used (`sessions`, `messages`, `configs`, the duplicate thresholds) stay in `NewService`
      until the lifecycle slice, which deletes the ~50 test call sites that build it. This also removes
      `MaxKnowledgeExtractionItems` entirely — the `Config` field,
      `DefaultMaxKnowledgeExtractionItems`, `ErrMaxKnowledgeExtractionItemsOutOfRange`, the
      `max_knowledge_extraction_items` yaml key in `configfile`, and
      `Config.WithDefaults`/`Validate` (delete them and their callers unless another setting
      needs them). Add one regression test that a `config.yaml` still containing the old key
      loads without error
- [ ] Backend, pending reconciliation: `reconcile_pending.go`, the remaining reconciliation
      helpers, relations, and their bindings and repositories
- [ ] Backend, lifecycle: approve/deprecate/update/list/review/backfill/indexing/`DeleteItem`
      and their bindings; shrink `Service` and `NewService`
- [ ] Domain and SQLite: drop status/source/topic filters and fields; migration; repository
      and reset cleanup; regenerate mocks and Wails bindings
- [ ] Docs: mark the superseded specs, update `Athena.md`, `Planning.md`, README,
      `lib/documentation.ts`, CHANGELOG (breaking, see below)

## Acceptance Criteria

- Importing a document into a session still works end to end (through the backend) and the
  chat still cites it; a session's chat never retrieves another session's chunks.
- No "Extract knowledge" action, Knowledge nav entry, Review, draft/approve/deprecate
  or reconciliation remains anywhere in the UI or in the Wails bindings.
- Opening an existing database drops the evidence/proposal/relation tables, leaves
  `knowledge_items`, `knowledge_chunks` and `ingested_files` empty with the new schema, keeps
  sessions, messages and `message_sources`, and passes `PRAGMA foreign_key_check`; reopening
  does not repeat the migration.
- Deleting a session or a folder still removes its documents, chunks, ingested files and
  index entries.
- The `max_knowledge_extraction_items` setting no longer exists in code, bindings or UI. An
  existing `config.yaml` that still contains the key loads normally (the loader ignores
  unknown keys) and the key disappears the next time the config is saved.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application/vectorstore code, and the frontend suite, lint, typecheck and Stryker on
  the changed files pass. Removing code must not lower the coverage ratio below the
  threshold; if it does, the remaining tests are strengthened, not new dead code added.

## Breaking change

This drops user data (extracted items, evidence, proposals, relations) and removes a
feature, so it is breaking in the sense of AGENTS.md's versioning rules. The version is
bumped manually at release; the CHANGELOG entry under `[Unreleased]` is written as
**Removed** and states that extracted knowledge is discarded by the migration.

## Out of scope

- Renaming `Item` to `Source` (and the related field/table cleanup) — a follow-up.
- The functional Sources panel: listing a session's documents, Add, Remove (2.17).
- Redesigning Phase 3/7 features that depended on extracted knowledge.
