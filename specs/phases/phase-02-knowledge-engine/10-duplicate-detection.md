# Phase 2.10 — Knowledge Duplicate Detection

## Goal

Detect when extracted knowledge is already represented before the user creates another Knowledge Item.

File-level idempotency in 2.3 is unrelated: it prevents duplicate chunks when a folder is re-imported. This spec detects duplicate **concepts and meanings** among Knowledge Items.

## Dependencies

- 2.2 supplies validated, unpersisted extraction candidates
- 2.8 keeps all Knowledge Item statuses searchable in the vector store
- 2.9 attaches evidence to every candidate that can be saved

## Two-stage detection

Detection is restricted to the candidate's topic. The same concept name in two different topics is not assumed to be the same knowledge.

### 1. Exact normalized match

`NormalizeConcept` trims, lowercases Unicode, turns every run of non-letter/non-digit characters into one space, and collapses whitespace. It does not strip accents. For example, `" Cache-Aside  Pattern "` becomes `"cache aside pattern"`.

All `draft`, `approved`, and `deprecated` items participate. Exact normalization does not spend an embedding call and always produces `MatchExact` with score `1`.

### 2. Semantic match

When no exact match exists, embed the candidate's rendered concept + definition and search `SourceAthena` chunks in the same topic across every status:

```go
const (
    MatchExact    = "exact"
    MatchSemantic = "semantic"

    DefaultDuplicateTopK       = 5
    DefaultDuplicateSimilarity = 0.90
)

type DuplicateMatch struct {
    ItemID    string
    Concept   string
    Status    string
    MatchType string
    Score     float64
}

func (s *Service) FindDuplicates(
    ctx context.Context,
    candidate Item,
    topK int,
    minScore float64,
) ([]DuplicateMatch, error)
```

The semantic threshold is constructor-injected and defaults to `0.90`. Results are ordered by score descending, then item ID ascending. Multiple chunks must never return the same item twice.

An empty vector store or an exact match makes no embedding call. An embedding/indexing failure does not discard the extraction candidate: it returns the deterministic matches plus a typed warning that the desktop adapter logs and presents as “semantic duplicate check unavailable”.

## Save policy and UI

- Exact match against a non-deprecated item disables direct **Create**; the user must reconcile with the existing item in 2.11
- Exact match against a deprecated item offers “create a successor” through reconciliation; the old item is never reactivated implicitly
- Semantic matches are warnings, not proof: the user may explicitly choose “Create separately”
- No match preserves the current save-as-draft/save-and-approve flow

The extraction dialog shows the existing concept, status, similarity reason, and a link to inspect it. The backend repeats exact duplicate detection when saving; a crafted or stale frontend cannot bypass it.

## Tasks

- [ ] `internal/domain/knowledge/duplicate.go` — match types, `DuplicateMatch`, defaults, typed partial-check warning
- [ ] `internal/application/knowledge/normalize.go` — pure `NormalizeConcept`
- [ ] `internal/application/knowledge/duplicates.go` — exact repository lookup + semantic shortlist
- [ ] `internal/domain/knowledge/repository.go` — normalized-concept lookup within a topic
- [ ] `internal/application/knowledge/extraction.go` — attach duplicate matches to every candidate
- [ ] `internal/interfaces/desktop/knowledge.go` — expose matches and enforce the save policy
- [ ] Extraction dialog — duplicate warning, existing-item preview, and explicit “Create separately” for semantic matches

## Acceptance Criteria

- Case, surrounding whitespace, repeated separators, and punctuation differences produce an exact match
- Accented and unaccented words are not silently treated as exact; semantic detection may still match them
- The same normalized concept in another topic is not an exact duplicate
- Exact matching works for draft, approved, and deprecated items without requesting an embedding
- Semantic results below `DefaultDuplicateSimilarity` are excluded and results contain each item at most once
- Exact duplicates cannot be saved through a forged desktop call
- A semantic match can be created separately only after an explicit user choice
- An unavailable semantic check leaves the candidate usable while visibly reporting the incomplete check
- Raising the injected threshold changes the semantic results in a deterministic unit test
