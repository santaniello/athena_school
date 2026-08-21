# Phase 2.9 — Persistent Provenance

## Goal

Every Knowledge Item records the evidence that supports it, and every RAG-assisted answer keeps the exact sources used after the session is resumed or the app restarts.

This spec adds provenance, not automatic trust. Evidence explains **where a claim came from**; the existing `draft → approved → deprecated` lifecycle still determines whether Athena trusts it.

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
<content>`. The LLM envelope adds `evidence: [{message_id, quote}]` per candidate,
with at most `maxEvidencePerItem = 5` references and
`maxEvidenceQuoteChars = 1000` Unicode characters per quote. A quote must be copied
verbatim from that message. The application trims its edges and verifies it with
`strings.Contains(message.Content, quote)`; it never asks another LLM whether the
quote is faithful.

The application accepts only message IDs present in the capped transcript and
belonging to the requested session. Unknown, duplicated, cross-session, blank,
over-limit, or non-verbatim references are rejected from that candidate. A candidate
without at least one valid evidence reference is invalid and is skipped without
discarding valid siblings.

`SaveDrafts` still regenerates every client-controlled ID. In addition, it reloads
the referenced messages, repeats the verbatim-quote checks, creates immutable quote
snapshots, and persists the item plus its evidence links in one SQLite transaction.
A later edit or deletion of the original session cannot silently change what
supported the item at approval time.

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
    created_at   DATETIME NOT NULL
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

Evidence is deleted only when it has no remaining `knowledge_item_evidence`
reference; after 2.12, cleanup also checks `knowledge_revision_evidence`. Item
deletion removes the links, not evidence still used elsewhere. Message-source rows
belong to the assistant message and cascade when that message is deleted.

## Tasks

- [ ] `internal/domain/knowledge/evidence.go` — evidence types, validation, repositories, origin constants
- [ ] `internal/domain/study/repository.go` — extend the 2.6 atomic completed-assistant-message/context update with sources
- [ ] `internal/infrastructure/sqlite/migrations.go` — the three tables and evidence lookup index
- [ ] `internal/infrastructure/sqlite/evidence_repository.go`, `message_source_repository.go`
- [ ] `internal/application/knowledge/extraction.go` / `prompt.go` / `parse.go` — message markers, evidence IDs, server-side ownership validation, transactional save
- [ ] `internal/application/study/send_message.go` — persist the final sources with the completed assistant message
- [ ] `internal/interfaces/desktop/knowledge.go` — `ListKnowledgeItemEvidence(id)`
- [ ] Resume-session result and frontend message model — attach historical sources by assistant message ID
- [ ] Knowledge Item detail — an **Evidence** section with source label and excerpt

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
