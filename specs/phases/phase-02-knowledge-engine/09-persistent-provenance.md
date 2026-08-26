# Phase 2.9 — Persistent Provenance

## Goal

Every Knowledge Item created by an evidence-bearing extraction flow records the
evidence that supported its extracted version, and every RAG-assisted answer keeps
the exact sources used after the session is resumed or the app restarts. Lightweight
imported-file shadow Items remain the explicit exception described below.

This spec adds provenance, not automatic trust. Evidence explains **where the
extracted version came from**; the existing `draft → approved → deprecated`
lifecycle still determines whether Athena trusts it. If a user later edits the
Item, its original Evidence remains historical extraction provenance and is not a
claim that the excerpt still supports the edited content. Phase 2.12 will attach
Evidence to the exact revision it supported.

## Dependencies

- 2.2 supplies extracted candidates and the bounded transcript
- 2.3 supplies stable chunk IDs and file metadata
- 2.5 supplies the final, context-capped `RetrievalResult.Sources`

## Evidence model

```go
const (
    OriginSessionMessage = "session_message"
    OriginKnowledgeChunk = "knowledge_chunk"
)

type Evidence struct {
    ID          string
    OriginType  string
    OriginID    string // message ID or chunk ID; no FK, because the snapshot must survive source deletion
    SourceLabel string // session topic or file path + heading
    Excerpt     string // immutable bounded snapshot of the supporting text
    CreatedAt   time.Time
}

type ItemEvidence struct {
    ItemID     string
    EvidenceID string
}

type EvidenceRef struct {
    MessageID string
    Quote     string
}
```

The extraction transcript renders every included turn as `[message:<id>] <role>:
<content>`. The LLM envelope adds `evidence: [{message_id, quote}]` per candidate.
The application collects, in response order, at most the first
`maxEvidencePerItem = 5` valid references; references after the fifth valid one are
ignored. Each quote may contain at most `maxEvidenceQuoteChars = 1000` Unicode
characters and must be copied verbatim from that message. The application trims
its edges and verifies it with `strings.Contains(message.Content, quote)`; it never
asks another LLM whether the quote is faithful.

The application accepts only message IDs present in the capped transcript and
belonging to the requested session. Unknown, duplicated, cross-session, blank,
over-limit, or non-verbatim references are rejected from that candidate. A candidate
without at least one valid evidence reference is invalid and is skipped without
discarding valid siblings.

After parsing, the backend creates a transient extraction batch receipt. For every
candidate, the receipt stores the source session, the source label, and the exact
validated EvidenceRefs. The frontend receives the opaque batch and candidate IDs,
but never becomes authoritative for provenance. Receipts exist only in memory:
successful saves consume only their candidate receipt, `Dismiss` discards the
remaining batch, and closing the app loses all unsaved candidates and receipts.

`SaveDrafts` and `SaveAndApprove` accept the extraction batch ID and still
regenerate every durable, client-controlled ID. The candidate ID is used only to
look up its backend receipt. Save reloads the receipt session's Messages and repeats
the ownership, bound, and verbatim-quote checks. A Message edited around a quote is
still valid if it continues to contain that exact quote; a missing Message or a
Message that no longer contains it makes that candidate unsavable.

Each Item, its immutable Evidence snapshots, and its links are persisted in one
SQLite transaction. Batches are not atomic: if A commits and B fails, A remains
saved and only B and unattempted siblings remain available for retry. Indexing stays
post-commit under Phase 2.8 semantics, so an indexing failure does not roll back the
durable Item or Evidence and does not make the candidate eligible for save retry.
A later edit or deletion of the original Message or Study Session cannot change the
snapshot fixed during extraction and persisted during save.

`OriginKnowledgeChunk` is available for a future flow that promotes a chunk into a **richer, LLM-structured** Knowledge Item — distinct from the lightweight shadow Item every imported file already gets automatically, heuristically, with no LLM call (2.3). That shadow Item has no evidence trail of its own; `OriginKnowledgeChunk` is reserved for the day a chunk (imported or otherwise) is deliberately promoted through real extraction.

## Persisted answer sources

```go
type MessageSource struct {
    MessageID  string
    Position   int
    ChunkID    string
    ItemID     string
    SourceType string
    FilePath   string
    Heading    string
    Concept    string
    Score      float64
    Excerpt    string
}
```

Only chunks that survive 2.5's similarity threshold and context cap are saved.
`Position` preserves their exact rendered order. Source type, file path, heading,
concept, score, and the exact chunk content used in the context are snapshots, so
historical answers remain explainable after a note is re-imported or a Knowledge
Item is edited.

The completed assistant message and its sources extend Phase 2.6's existing
assistant-message/context-state transaction through a study-domain repository
operation. A failed final transaction writes neither the assistant message,
context state, nor sources. `study:sources` remains for immediate rendering,
while `ResumeStudySession` loads persisted sources for historical assistant
messages.

## Schema

```sql
CREATE TABLE IF NOT EXISTS knowledge_evidence (
    id           TEXT PRIMARY KEY,
    origin_type  TEXT NOT NULL,
    origin_id    TEXT NOT NULL,
    source_label TEXT NOT NULL,
    excerpt      TEXT NOT NULL,
    created_at   DATETIME NOT NULL,
    UNIQUE (origin_type, origin_id, excerpt)
);

CREATE TABLE IF NOT EXISTS knowledge_item_evidence (
    item_id     TEXT NOT NULL REFERENCES knowledge_items(id) ON DELETE CASCADE,
    evidence_id TEXT NOT NULL REFERENCES knowledge_evidence(id),
    PRIMARY KEY (item_id, evidence_id)
);

CREATE TABLE IF NOT EXISTS message_sources (
    message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    chunk_id    TEXT,
    item_id     TEXT,
    source_type TEXT NOT NULL,
    file_path   TEXT NOT NULL DEFAULT '',
    heading     TEXT NOT NULL DEFAULT '',
    concept     TEXT NOT NULL DEFAULT '',
    score       REAL NOT NULL,
    excerpt     TEXT NOT NULL,
    PRIMARY KEY (message_id, position)
);

CREATE INDEX IF NOT EXISTS idx_knowledge_item_evidence_evidence
    ON knowledge_item_evidence(evidence_id);
```

Evidence with the same `OriginType`, `OriginID`, and edge-trimmed `Excerpt` is shared
globally; the first immutable snapshot, including its `SourceLabel` and `CreatedAt`,
remains authoritative. Evidence is deleted only when it has no remaining
`knowledge_item_evidence` reference; after 2.12, cleanup also checks
`knowledge_revision_evidence`. Item deletion removes the links, not evidence still
used elsewhere. Message-source rows belong to the assistant message and cascade
when that message is deleted.

## Tasks

Knowledge Item Evidence (increment 1 — complete):

- [x] `internal/domain/knowledge/evidence.go` — evidence types, validation, repositories, origin constants
- [x] `internal/infrastructure/sqlite/migrations.go` — `knowledge_evidence`/`knowledge_item_evidence` and the evidence lookup index (`message_sources`, increment 2, remains)
- [x] `internal/infrastructure/sqlite/evidence_repository.go`
- [x] `internal/application/knowledge/extraction.go` / `prompt.go` / `parse.go` — message markers, evidence IDs, server-side ownership validation, transactional save
- [x] `internal/interfaces/desktop/knowledge.go` — `ListKnowledgeItemEvidence(id)`
- [x] Knowledge Item detail — an **Evidence** section with source label and excerpt

Persisted answer sources (increment 2 — not started):

- [ ] `internal/domain/study/repository.go` — extend the 2.6 atomic completed-assistant-message/context update with sources
- [ ] `internal/infrastructure/sqlite/message_source_repository.go` and the `message_sources` table
- [ ] `internal/application/study/send_message.go` — persist the final sources with the completed assistant message
- [ ] Resume-session result and frontend message model — attach historical sources by assistant message ID

## Acceptance Criteria

- Every saved extracted item has at least one evidence snapshot from a message in the source session
- A fabricated message ID, a message from another session, and an ID outside the capped transcript are rejected
- Blank, over-limit, and non-verbatim evidence quotes are rejected; a literal quote at the limit is accepted unchanged
- One invalid candidate does not prevent valid candidates in the same extraction from being saved
- Saving an item and its evidence is atomic; a failure leaves neither behind
- Editing or deleting the original message does not change the evidence snapshot
- Sources shown during a RAG response are exactly the sources restored after an app restart
- A failed final transaction persists neither the assistant message, its context-state update, nor `message_sources`
- Restored sources retain `SourceType`, so historical `user_note`, `imported_doc`, and `athena` labels match their live 2.5 rendering
- Re-importing a changed note does not rewrite the exact chunk snapshot attached to historical answers
- Deleting one item does not delete evidence still referenced by another item

## Implementation handoff — 2026-08-26

### Current increment and authorization boundary

The spec was decomposed into two independently deliverable increments:

1. **Knowledge Item Evidence** — the active, approved increment documented below.
2. **Persisted answer sources** — `message_sources`, atomic assistant-message source persistence,
   resume restoration, and historical RAG source UI. This has not been designed or authorized for
   implementation yet.

Before starting Knowledge Item Evidence, the required global SQLite foreign-key prerequisite was
completed and committed as `0970781 fix(sqlite): enforce foreign key integrity`. It enables foreign
keys on every connection, migrates `messages` to `ON DELETE CASCADE`, migrates `usage` to
`ON DELETE SET NULL`, removes known orphaned test rows, and blocks `Open` on unexpected
`foreign_key_check` violations.

The Knowledge Item Evidence work described below is **complete**: every behavior was implemented
Red → Green → Refactor, all quality gates pass, and it is ready to commit.

### Approved design decisions

- Evidence is fixed at extraction time for precision, then its Message and literal quote are
  revalidated at save time.
- Extraction candidates and their receipts are transient. Closing the app before save loses them.
- Each Knowledge Item is saved in its own SQLite transaction. If A commits and B fails, A stays
  saved and only B and C remain for retry.
- Evidence is globally shared when `OriginType`, `OriginID`, and the edge-trimmed `Excerpt` are the
  same. Distinct Items may therefore reference the same Evidence row.
- Deleting an Item removes its link. The Evidence row is deleted only after its final Item reference
  is gone; Phase 2.12 will also protect revision references.
- The frontend is not trusted to return provenance. The backend keeps a transient extraction receipt
  containing the source session, source label, and validated EvidenceRefs. Save resolves provenance
  exclusively from that receipt.
- A successful per-Item transaction consumes only that candidate's receipt. Pending and unattempted
  siblings retain theirs for retry. `Dismiss` will discard the remaining batch.
- Editing an Item preserves its original Evidence link. The Explorer must label it
  **Extraction Evidence** and explain that later edits may no longer be supported by the excerpt;
  it represents the origin of the extracted version until Phase 2.12 attaches it to a revision.
- Only one extraction batch is exposed by the current UI. A new extraction starts after the previous
  batch has been saved or dismissed.
- Indexing remains post-commit under Phase 2.8 semantics. An indexing failure does not roll back the
  durable Item or Evidence and does not make that candidate eligible for save retry.

### Implemented (complete)

All production changes below were introduced after a failing test and brought back to Green in the
affected package:

- `internal/domain/knowledge/evidence.go` — `OriginSessionMessage`, `OriginKnowledgeChunk`,
  `Evidence`, `ItemEvidence`, `EvidenceRef`, validation errors, `Evidence.Validate`, and the
  `EvidenceRepository` port.
- `internal/application/knowledge/prompt.go` — renders transcript turns as
  `[message:<id>] <role>:\n<content>` and requires the LLM envelope to return
  `evidence: [{message_id, quote}]` (1–5 refs, ≤1000 Unicode chars each, verbatim).
- `internal/application/knowledge/parse.go` — parses and validates EvidenceRefs (unknown/out-of-cap
  IDs, blanks, over-limit or non-verbatim quotes, duplicates all rejected); caps accepted references
  at `maxEvidencePerItem`, mutation-protected by a 6-reference boundary test.
- `internal/application/knowledge/receipt_store.go` — mutex-protected, in-memory receipt store
  grouped by extraction batch (`Create`/`Get`/`Consume`/`Discard`).
- `internal/application/knowledge/extraction.go` and `service.go` — `ExtractFromSession` returns
  `ExtractionBatch{ID, Items}` and stores backend receipts; `Service` now takes an injected
  `EvidenceRepository`; `SaveDrafts`/`SaveAndApprove` accept the batch ID, resolve each candidate's
  receipt, reload and revalidate its Study Session Messages, and save the regenerated Item plus its
  Evidence snapshots/links in one `Transactor.WithinTx` per Item — the receipt is consumed only after
  commit, so a failed or unattempted candidate's receipt stays retryable; `DiscardExtraction(batchID)`
  drops a batch's remaining receipts.
- `internal/application/knowledge/delete.go` — `DeleteItem` now also runs
  `EvidenceRepository.DeleteUnreferenced` inside its existing Item/chunk transaction.
- `internal/application/knowledge/list.go` — `ListItemEvidence(ctx, itemID)`.
- `internal/infrastructure/sqlite/migrations.go` — `knowledge_evidence`/`knowledge_item_evidence`
  plus the evidence lookup index and the `UNIQUE (origin_type, origin_id, excerpt)` sharing identity.
- `internal/infrastructure/sqlite/evidence_repository.go` — `GetOrCreate`, `LinkToItem`,
  `ListByItem`, `DeleteUnreferenced`; a real-SQLite integration test (a `BEFORE INSERT` trigger that
  always fails on `knowledge_item_evidence`) proves the Item+Evidence+link transaction rolls back as
  one unit, not just through mocks.
- `internal/interfaces/desktop/knowledge.go` — `ExtractionResult.batchId`; `SaveExtractedKnowledge`/
  `SaveAndApproveExtractedKnowledge` take the batch ID; `DiscardExtraction(batchID)`;
  `ListKnowledgeItemEvidence(id)`.
- `main.go` — wires `sqlite.NewEvidenceRepository(db)` into the knowledge `Service`.
- Frontend (`frontend/src/lib/knowledge.ts`, `knowledge-extraction-dialog.tsx`,
  `KnowledgeExplorerScreen.tsx`) — `batchId` threaded through extract/save; every true dialog-close
  path (Dismiss button, the dialog's own close control, Escape/click-outside, and closing after a
  fully successful save) calls `discardExtraction`, but never from `handleSave`'s error branch, so a
  partial-save failure leaves the remaining receipts retryable; the Knowledge Explorer item detail
  gained an **Extraction Evidence** section (source label, exact excerpt, an empty state for
  legacy/shadow Items, and the meaning-preserving warning `Evidence captured during extraction. Later
  edits may no longer be supported by this excerpt.`). Wails bindings regenerated via `wails generate
  module`.

Quality gates run against the complete change: `go test ./...` (race-free, no `-race` flag needed —
matches existing CI), `golangci-lint run` (0 issues), `govulncheck ./...` (0 vulnerabilities in this
code), combined coverage 89.5% (`.githooks/pre-commit`'s own computation, mocks excluded), `npx tsc
--noEmit` and `npx vitest run` (548 passed) on the frontend, and `gremlins unleash` against
`internal/domain` and `internal/application` with every mutant in changed code killed (the two that
first survived — the parse.go cap-boundary and a missing `LinkToItem`-failure path in extraction.go —
were closed by strengthening the test suite, not by weakening the mutation run).

### Still out of scope for this increment

- `message_sources`, persisted RAG answer sources, resume restoration, and their frontend rendering.
- The close-app warning for unsaved candidates. It remains a separate follow-up increment; ordinary
  `Dismiss` cleanup is in scope here.
- Persisting receipts/candidates across restarts.
- A real `OriginKnowledgeChunk` promotion flow.
- Phase 2.12 revision history or manual replacement of Evidence after an Item edit.
