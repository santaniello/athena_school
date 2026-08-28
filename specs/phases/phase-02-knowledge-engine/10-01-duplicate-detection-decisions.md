# Phase 2.10.1 — Duplicate Detection: Implementation Decisions

## Goal

`10-duplicate-detection.md` specifies the match types, thresholds, and save
policy for duplicate detection, but leaves three implementation questions open
that materially shape the code: where the exact-match comparison actually
lives, how a batch behaves when the semantic stage's embedding call fails
partway through, and how the save-time re-check signals a blocked duplicate
back to the caller. This addendum records the decisions and the reasoning
behind them, the same way `09-01-foreign-key-migration-gaps.md` did before its
own implementation.

## Decision 1 — Exact-match comparison: persisted column, not an in-memory scan

`10-duplicate-detection.md` asks for a new `Repository` method for
"normalized-concept lookup within a topic" and a pure `NormalizeConcept` in
`internal/application/knowledge/normalize.go`. Those two requirements conflict
with the project's dependency rule (`interfaces/desktop → application →
domain ← infrastructure`): `internal/infrastructure/sqlite` may depend on
`internal/domain`, never on `internal/application`, so an infra-side exact
comparison cannot call an application-owned normalization function.

**Options considered:**

- **A — persisted `normalized_concept` column.** `NormalizeConcept` moves to
  `internal/domain/knowledge`, next to the existing `NormalizeTopic` (same
  file or a sibling), which already solves the identical problem for topics.
  `knowledge_items` gains a `normalized_concept TEXT` column, computed on
  every `Save`/`Update`, indexed as `(topic, normalized_concept)`. The new
  `Repository.FindByNormalizedConcept(ctx, topic, normalizedConcept string)
  ([]Item, error)` becomes an indexed `WHERE` lookup.
- **B — in-memory scan.** `NormalizeConcept` stays in application, exactly as
  written in `10-duplicate-detection.md`. The "new" repository method is a
  thin restatement of `FindByTopic`; the application layer loads every item
  in the topic and filters by normalized-string equality in Go, on every
  extraction candidate.

**Chosen: A.** It follows the codebase's own precedent (`NormalizeTopic`
already lives in domain for exactly this reason) and keeps exact-match lookup
indexed instead of re-scanning a whole topic's items per candidate.

**Consequence — migration must backfill in Go, not SQL.** `ALTER TABLE
knowledge_items ADD COLUMN normalized_concept TEXT` leaves the column `NULL`
on every row that existed before the migration runs. SQL's `NULL = value` is
never true, so those rows would be permanently invisible to
`FindByNormalizedConcept` — silently defeating exact-match detection for
every item that predates the migration, regardless of whether the current
database happens to be empty at deploy time. Unlike
`migrateSessionForeignKeyActions`'s backfill (`SET folder_id = 'default'`,
a constant), this backfill's value is `NormalizeConcept(concept)` — a
function result that varies per row — so it cannot be expressed as a single
SQL statement. It runs as a Go step: `SELECT id, concept FROM
knowledge_items`, compute `domainknowledge.NormalizeConcept` per row, `UPDATE
... SET normalized_concept = ? WHERE id = ?`, inside the same transaction as
the `ALTER TABLE`.

## Decision 2 — Batch behavior when an embedding call fails mid-extraction

`ExtractFromSession` can return several candidates from one LLM extraction.
Each candidate without an exact match needs its own embedding call for the
semantic stage. If that call fails for candidate N, what happens to
candidates N+1 onward in the same batch?

**Options considered:**

- **A — short-circuit the semantic stage only.** After the first embedding
  failure in a batch, every later candidate that would need the semantic
  stage skips straight to the "semantic duplicate check unavailable" warning,
  without another embedding attempt. The exact stage is unaffected and always
  runs, for every candidate, regardless of this state — it never calls the
  LLM.
- **B — retry independently per candidate.** Every candidate attempts its own
  embedding call regardless of earlier failures in the same batch.

**Chosen: A**, mirroring the existing precedent in
`internal/application/knowledge/extraction.go`'s `saveCandidates`: once one
item's indexing fails, the rest of the batch is still saved but no longer
indexed, specifically to avoid paying for N further failing embedding calls
in one request. Option B would multiply user-visible latency by the batch
size for no benefit whenever the provider is genuinely unavailable (missing
key, network outage) — the common failure shape for an embedding call.

**Scope boundary:** the short-circuit is a property of the semantic stage
only. The exact stage (a database lookup, no embedding involved) always runs
independently for every candidate in the batch.

## Decision 3 — Save-time duplicate re-check: silent skip, no new signal

`10-duplicate-detection.md` requires the backend to repeat exact-match
detection when saving, so a crafted or stale frontend cannot bypass the save
policy. This lives inside `saveCandidates`, the single path shared by
`SaveDrafts` and `SaveAndApprove`.

**Options considered:**

- **A — same pattern as existing skips.** A candidate that turns out to be an
  exact duplicate at save time is skipped exactly like today's "invalid
  evidence" or `item.Validate()` failure cases: not persisted, its receipt
  restored for retry, its index simply absent from `SavedIndices`. No new
  field distinguishes the reason.
- **B — distinct signal.** `KnowledgeSaveResult` gains a field (e.g.
  `DuplicateIndices`) identifying which candidates were blocked specifically
  for being duplicates, so the dialog can show a specific message instead of
  leaving the candidate silently unmarked.

**Chosen: A.** This path is a defense-in-depth boundary — a well-behaved
frontend already disables Create before reaching it — so the extra DTO/
binding/UI surface of B is not justified for a case that should be rare in
normal use.

**Known, accepted interaction:** `knowledge-extraction-dialog.tsx`'s
`handleSave` calls `handleClose()` (which discards every remaining pending
receipt in the batch) whenever `result.error` is empty — including when a
candidate was silently skipped without any hard error. A skipped duplicate's
just-restored receipt is therefore immediately discarded by the dialog
closing right after, defeating the purpose of restoring it. This is **not a
regression introduced by 2.10** — the identical interaction already exists
today for the "invalid evidence" skip case. Fixing it is out of scope for
this increment; Option A is consistent with an already-accepted gap, not a
new one.

## Test plan implications

- A migration test seeding a pre-migration row (not just an empty database)
  and asserting `normalized_concept` is correctly populated after `Open`.
- A `saveCandidates`/`FindDuplicates` test proving the exact stage still runs
  for a later candidate after an earlier candidate's embedding call failed in
  the same batch, while the later candidate's own semantic stage is skipped
  only when it would otherwise have been needed.
- Mutation testing (`make mutation-go`) on the new domain/application code,
  per AGENTS.md.

## References

- `specs/phases/phase-02-knowledge-engine/10-duplicate-detection.md` — base
  spec this addendum implements.
- `specs/phases/phase-02-knowledge-engine/09-01-foreign-key-migration-gaps.md`
  — the Go-side backfill precedent this decision follows.
- `internal/domain/knowledge/item.go` — `NormalizeTopic`, the precedent for
  Decision 1.
- `internal/application/knowledge/extraction.go` — `saveCandidates`'s
  existing indexing short-circuit, the precedent for Decision 2, and the
  skip/restore pattern, the precedent for Decision 3.
- `frontend/src/components/knowledge-extraction-dialog.tsx` — `handleSave`,
  the pre-existing auto-close interaction noted in Decision 3.
