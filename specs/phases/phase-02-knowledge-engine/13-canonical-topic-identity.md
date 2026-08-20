# Phase 2.13 — Canonical Topic Identity

> **Status: Draft — grilling required before implementation.**
>
> This document records the discovered problem, the already-agreed product
> intent, and the decisions that still need to be grilled. Candidate models,
> tasks, and acceptance criteria below are not implementation authorization.

## Goal

Treat spelling variants that differ only by topic identity normalization as one
logical topic across Athena. At minimum, `Go`, `go`, and edge-whitespace
variants such as ` Go ` must no longer produce separate Knowledge Explorer
branches or separate search/filter scopes.

The normalization rule belongs to the domain. Infrastructure adapters and the
frontend consume a canonical identity; they must not invent their own trimming
or case-folding behavior.

## Why this needs its own spec

Phase 2.4 deliberately keeps exact topic comparison after domain-owned
`strings.TrimSpace`. Expanding that adapter to compare case-insensitively would
hide a cross-cutting identity change in vector-search infrastructure while the
rest of the product remained inconsistent.

Today topic handling is fragmented:

- `knowledge.Item.Validate` rejects an edge-trimmed empty topic but does not
  canonicalize or mutate it;
- extraction/save paths edge-trim topics, while Explorer updates and imported
  path-derived topics are not uniformly normalized;
- SQLite filters and `SELECT DISTINCT topic` use exact binary equality;
- frontend grouping uses exact JavaScript string keys;
- imported folder `go/` and a study/extracted topic `Go` therefore become
  separate branches;
- vector filters in 2.4 are intentionally exact, so changing only that layer
  would make the Explorer and retrieval disagree;
- 2.10 scopes exact and semantic duplicate detection by topic, so changing
  topic identity can reveal duplicates that were previously considered to be
  in separate topics.

This is not a cosmetic lowercase operation. It affects persisted identity,
display names, migrations, duplicate handling, search metadata, future
progress aggregation, and every write boundary that accepts a topic.

## Settled intent

Only these requirements are settled before grilling:

1. `Go` and `go` represent the same logical topic.
2. Leading/trailing whitespace does not create a new identity.
3. The canonicalization rule lives in the domain and is reused by every
   application write/filter boundary.
4. User-facing topic text remains human-readable; a canonical lookup key must
   not force every label to be displayed as lowercase.
5. Existing user data is migrated without silently deleting or merging
   Knowledge Items.
6. Vector search continues to use exact equality, but over canonical topic
   identity rather than inconsistent display strings.

Everything else in this document remains open until the dedicated grilling is
confirmed.

## Identity versus display

The working concept separates two values:

```go
type Topic struct {
    Key         string // canonical identity used for equality, indexes, and filters
    DisplayName string // user-facing spelling
}
```

This is a candidate shape, not an approved API. The central invariant would be:

```text
CanonicalTopicKey("Go")  == CanonicalTopicKey("go")
CanonicalTopicKey(" Go ") == CanonicalTopicKey("go")
```

The grilling must decide whether `Topic` becomes a first-class entity, whether
rows store both values directly, or whether a separate topic registry owns the
display name.

## Impact map

| Area | Current identity | Required question |
|---|---|---|
| Knowledge Items | raw `topic` string | store a canonical key, reference a Topic entity, or derive on every boundary? |
| Imported shadow Items/chunks | folder/H1/basename spelling | which display spelling wins when it collides with an existing topic? |
| Study sessions | edge-trimmed raw topic | does canonical identity apply retroactively to sessions and extraction? |
| Vector chunks/filters | exact `Chunk.Topic` | add `TopicKey`, replace `Topic`, or guarantee canonical content in the existing field? |
| Explorer tree | `DISTINCT topic`, exact JS keys | where does the single display label come from? |
| Duplicate detection | candidate's exact topic scope | what happens to duplicates revealed by merging topic identities? |
| Revisions/evidence | Item snapshots contain Topic | does display rename create revisions, and what remains historically visible? |
| Future progress/flashcards | topic-based aggregation planned | which canonical identifier becomes the stable cross-phase key? |

## Design tree to grill

### 1. Canonical equivalence

The root decision is the exact equivalence algorithm. The grilling must settle:

- Unicode normalization: none, NFC, or NFKC;
- simple lowercasing versus Unicode case folding;
- whether repeated internal whitespace collapses (`Go  Concurrency` versus
  `Go Concurrency`);
- whether separators are identity-significant (`machine-learning`,
  `machine_learning`, and `machine learning`);
- whether punctuation and diacritics remain significant;
- locale-sensitive cases such as Turkish dotted/dotless I;
- maximum normalized/display lengths and behavior when normalization becomes
  empty.

The implementation must use one deterministic algorithm on every supported OS
and persist enough information to keep future algorithm changes migratable.

### 2. Persistence model

This depends on the equivalence rule. Candidate approaches:

1. **Canonical column beside every display string** — add `topic_key` to each
   topic-bearing table and keep display text denormalized.
2. **First-class topic registry** — introduce a topic table with stable ID,
   canonical key, and display name; dependent rows reference the ID.
3. **Canonicalize the existing `topic` column destructively** — simplest
   schema, but loses display spelling and makes user-friendly renaming
   difficult.

The grilling must choose ownership, uniqueness constraints, foreign keys,
indexes, and whether the key is a stable value or a versioned derivation.

### 3. Display-name policy

When these existing labels collide:

```text
Go
go
GO
```

the product needs one visible label. Options still open include:

- preserve the oldest spelling;
- prefer an explicitly user-edited spelling;
- use deterministic title casing;
- ask the user during migration;
- allow aliases while one Topic owns the primary display name.

The grilling must also define whether renaming only the display name changes
identity, updates every Item, creates revisions, or remains metadata on a
first-class Topic.

### 4. Migration and collisions

The migration must be local, deterministic, transactional, and idempotent. It
must inventory at least:

- `knowledge_items`;
- `knowledge_chunks`;
- study sessions whose topic feeds extraction;
- any topic filters, indexes, revisions, relations, provenance, progress, or
  flashcard data present when this spec is implemented.

Merging topic identity can reveal same-concept Items that 2.10 previously kept
apart. The migration must never silently delete, overwrite, approve, deprecate,
or reconcile those Items. The grilling must choose whether such collisions:

- remain separate Items under one Topic and enter the existing duplicate-review
  workflow;
- block migration pending user review;
- create explicit reconciliation proposals;
- are merely reported for later action.

Rollback/recovery behavior, backup expectations, and what happens when one row
cannot be normalized are also open.

### 5. Write and query boundaries

Every creation, edit, import, extraction, filter, and lookup path must use the
same domain operation. The grilling must decide the public contract, for
example:

```go
func NewTopic(displayName string) (Topic, error)
func CanonicalTopicKey(value string) (string, error)
```

It must also settle:

- whether APIs accept a topic ID, canonical key, display name, or a typed
  `Topic`;
- how user-entered filters resolve display text to identity;
- how imported directory names select or create a Topic;
- how an edited shadow Item interacts with the file-as-source-of-truth rule;
- which layer returns an unknown-topic error;
- how generated Wails bindings represent the identity without duplicating
  business logic in TypeScript.

### 6. Vector and cache synchronization

Phase 2.4 stores topic metadata in SQLite and memory. This spec must decide:

- whether normalization changes only metadata or requires any re-embedding
  (the expected answer is no, but it is not approved until grilling);
- how all affected chunks are updated transactionally;
- how the active vector snapshot is updated after commit;
- how failures use 2.4's retryable index-warning policy;
- how topic aliases or display renames affect exact `SearchFilters` values.

## Candidate migration shape

The following sequence exists only to expose decisions; it is not yet an
approved implementation plan:

```text
Open database
    ↓
Apply schema capable of storing canonical identity
    ↓
Compute keys for every existing topic-bearing row
    ↓
Detect identity and duplicate collisions
    ↓
Persist mappings transactionally without deleting Items
    ↓
Rebuild/update topic indexes and vector metadata
    ↓
Show migration/review outcome
```

The grilling must decide whether migration happens automatically at database
open, behind a blocking screen, or through an explicit consent/review flow.

## Candidate task areas — not approved

- [ ] domain Topic identity/value object and canonicalization tests
- [ ] SQLite schema, constraints, indexes, migration, and rollback tests
- [ ] migration collision report and reconciliation integration
- [ ] knowledge/study/import application boundaries migrated to the domain API
- [ ] chunk metadata and active vector snapshot synchronization
- [ ] repository filters/listing grouped by canonical identity
- [ ] Wails result/input types carrying stable identity plus display text
- [ ] Explorer tree, edit flow, and collision/review UI
- [ ] cross-phase audit for duplicate detection, revisions, provenance,
      progress, and flashcards
- [ ] README migration/user-visible behavior and `[Unreleased]` changelog entry

## Candidate acceptance criteria — not approved

- `Go`, `go`, and edge-whitespace variants resolve to one logical Topic
- the Explorer renders one branch with a deterministic human-readable label
- exact repository and vector filters find every Item/chunk under that identity
- imports, extraction, manual edits, and direct backend calls all use the same
  domain normalization rule
- existing Items survive migration with stable IDs, content, status, evidence,
  and revision history
- collisions never cause silent deletion, overwrite, lifecycle transition, or
  automatic reconciliation
- topic metadata changes do not request a paid embedding unless the grilled
  design identifies a content change that genuinely requires one
- SQLite and the active in-memory vector snapshot remain coherent after
  migration and later topic edits
- migration is transactional, idempotent, recoverable, and covered by tests
- Unicode and locale edge cases match the exact algorithm selected during
  grilling on every supported OS
- frontend grouping and backend filtering cannot disagree about topic identity

## Dependency and delivery placement — open

This draft is numbered 2.13 because it was discovered after specs 2.9–2.12 were
authored. Numbering does not yet settle implementation order. The grilling must
decide whether canonical topic identity:

- ships after 2.12 as a migration/refinement;
- moves before 2.10 because duplicate detection depends on topic scope; or
- is split into an earlier identity migration and later cross-phase adoption.

No implementation may begin until the grilling closes every branch above and
the resulting approved spec replaces these candidate sections with concrete
contracts, tasks, and acceptance criteria.
