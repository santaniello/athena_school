# Phase 3.1 — Challenge Domain and Persistence

## Goal

A challenge is a system-design problem that exists independently of any attempt, session or
evaluation. This spec adds the `Challenge` model, its SQLite schema and a filterable repository.
It is the first block of the Exercise Engine ([ADR-005](../../decisions/ADR-005-exercise-engine.md)):
nothing here knows about diagrams, tutors or the UI.

Rubrics, attempts, evaluation and the catalog UI are later specs. This one only has to answer:
can a challenge be created, stored, read, updated and listed with filters.

## Decisions

1. **The topic comes from a studied session.** A challenge's topic is the `Topic` of a study
   session the user already studied, whether the challenge is created from a session, picked
   from the catalog or generated inside the lab. A challenge therefore always has a source
   session at creation time.
2. **The challenge outlives that session.** Deleting a session must not delete the challenge
   or its history (ADR-005, decision 5). The challenge stores `SourceSessionID` as a plain,
   nullable reference with **no `ON DELETE CASCADE`**, plus a `Topic` snapshot copied from the
   session at creation. The catalog filters by the snapshot. When the session is deleted the
   reference becomes `NULL` (`ON DELETE SET NULL`) and the topic text stays.
3. **Structured lists, not one free-text blob.** The statement is a scenario plus four lists:
   functional requirements, non-functional requirements, constraints and objectives, so the
   evaluator, the tutor and the defense can point at an individual item.
4. **No tags.** A challenge has one topic and one difficulty. The header chips in the layout
   sketch are derived from the topic in the UI spec, not stored.
5. **Type is fixed to `ARCHITECTURE`.** The column exists so the lifecycle stays generic, but
   no other value is accepted yet (no Algorithms Lab in this phase).
6. **Origin** is `MANUAL` or `AI_GENERATED`. Execution context and modality belong to the
   attempt, not to the challenge (ADR-005, decision 4).
7. **Difficulty** is `BEGINNER`, `INTERMEDIATE` or `ADVANCED`.
8. **No rubric and no defense-question count here.** The rubric arrives in
   [3.2](02-rubrics.md); the defense count arrives in 3.12, each with its own migration.
9. **Versioned by update timestamp only.** Editing a challenge updates it in place. Freezing what
   an attempt saw is the attempt's job ([3.3](03-attempts-and-session-link.md)), which snapshots
   what it needs.
10. **No data compatibility to keep.** The tables are new.

## Domain

`internal/domain/exercise/` (no import of `architecture`, `study` internals or Wails):

```go
type ChallengeType string      // ChallengeTypeArchitecture
type ChallengeOrigin string    // OriginManual, OriginAIGenerated
type Difficulty string         // DifficultyBeginner, DifficultyIntermediate, DifficultyAdvanced

type Challenge struct {
    ID                        string
    Type                      ChallengeType
    Title                     string
    Scenario                  string
    FunctionalRequirements    []string
    NonFunctionalRequirements []string
    Constraints               []string
    Objectives                []string
    Difficulty                Difficulty
    Topic                     string // snapshot of the source session's topic
    SourceSessionID           string // "" once the session is deleted
    Origin                    ChallengeOrigin
    CreatedAt                 time.Time
    UpdatedAt                 time.Time
}
```

`Validate()` rules, each with its own error:

- `Title`, `Scenario` and `Topic` are trimmed and non-empty.
- `Type`, `Difficulty` and `Origin` are known values.
- At least one functional requirement and at least one objective; no empty or whitespace-only
  list item; no duplicate items within one list.
- Lengths are bounded (title, scenario and item sizes) so a generated challenge cannot store an
  unbounded blob.

Port, in the same package:

```go
type ChallengeRepository interface {
    Create(ctx, Challenge) error
    Get(ctx, id string) (Challenge, error)       // ErrChallengeNotFound
    List(ctx, ChallengeFilter, Page) (ChallengePage, error)
}

type ChallengeFilter struct {
    Query      string       // case-insensitive over title, scenario and topic
    Topic      string       // exact, on the normalized snapshot
    Difficulty Difficulty
    Origin     ChallengeOrigin
}
```

`Page` carries `Limit` and `Offset`; `ChallengePage` returns the items and the total matching
count. Ordering is `UpdatedAt` descending, then `ID`, so pages are stable.

## Application

`internal/application/exercise.ChallengeService`:

- `Create(ctx, input)` validates, requires an existing source session (reads it through a narrow
  port, `SessionTopicReader`, so the package does not import `study`), copies that session's
  `Topic` into the snapshot (the port also returns the session's goal, which
  [3.6.1](06-01-challenge-generation.md) uses), sets origin and timestamps and persists. Unknown session:
  `ErrSourceSessionNotFound`.
- `Get` and `List`. There is no `Update`.

A challenge's content is **immutable after creation**, on purpose. An attempt freezes only its
rubric version, and the result screen ([3.13](13-evaluation-and-history-screen.md)) shows the
statement from the challenge, so editing it would make the history show a different statement
from the one the user answered. No screen edits challenges; when manual authoring is specified,
that spec must define how a challenge already used stays stable (a new version or a copy).

## SQLite

New migration, added to `internal/infrastructure/sqlite/migrations.go` in the existing style:

```sql
CREATE TABLE challenges (
  id                          TEXT PRIMARY KEY,
  type                        TEXT NOT NULL,
  title                       TEXT NOT NULL,
  scenario                    TEXT NOT NULL,
  functional_requirements     TEXT NOT NULL,   -- JSON array, see jsonlist.go
  non_functional_requirements TEXT NOT NULL,
  constraints                 TEXT NOT NULL,
  objectives                  TEXT NOT NULL,
  difficulty                  TEXT NOT NULL,
  topic                       TEXT NOT NULL,
  source_session_id           TEXT REFERENCES sessions(id) ON DELETE SET NULL,
  origin                      TEXT NOT NULL,
  created_at                  TEXT NOT NULL,
  updated_at                  TEXT NOT NULL
);
CREATE INDEX idx_challenges_topic      ON challenges(topic);
CREATE INDEX idx_challenges_difficulty ON challenges(difficulty);
```

Lists reuse the existing `jsonlist.go` helper. Search uses parameterized `LIKE` with escaped
wildcards; no user input is concatenated into SQL. The topic filter matches the normalized
snapshot (trimmed, lower-cased) through the same normalization the service applies on create.
Repository lives at `internal/infrastructure/sqlite/challenge_repository.go`; mocks are
generated into `exercise/mocks` (ADR-003).

## Tasks

Each slice is TDD (test first), keeps the build green and is committed on its own.

- [ ] Domain: types, `Challenge.Validate`, errors, repository and filter types
- [ ] SQLite: migration and `ChallengeRepository` (`Create`, `Get`)
- [ ] SQLite: `List` with filters, search, pagination and stable ordering
- [ ] SQLite: session deletion nulls `source_session_id` and keeps the challenge
- [ ] Application: `ChallengeService` (`Create` with topic snapshot, `Get`, `List`)
- [ ] Mocks regenerated; CI check that `internal/domain/exercise` does not import
      `internal/domain/architecture`
- [ ] Docs: CHANGELOG entry; no README change (no user-visible surface yet)

## Acceptance Criteria

- A challenge is created from an existing session; its `Topic` equals that session's topic at the
  time, and later renaming or deleting the session does not change it.
- Creating with an unknown session fails with `ErrSourceSessionNotFound`; invalid input fails
  with the specific validation error and persists nothing.
- Created challenges survive an application restart and round-trip every list field in order.
- `List` filters by query, topic, difficulty and origin, combines filters with AND, paginates with
  a correct total and returns a stable order.
- Deleting the source session keeps the challenge, sets `SourceSessionID` to empty and keeps the
  topic.
- Nothing changes a challenge's content after it is created.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint and vulnerability checks pass.

## Out of scope

- Rubrics ([3.2](02-rubrics.md)), attempts and the session exercise link
  ([3.3](03-attempts-and-session-link.md)), evaluation, completion status.
- The time limit (arrives with attempts in [3.3](03-attempts-and-session-link.md)), the defense question count (3.12), tags, a topic catalog or table.
- Any UI, Wails binding or AI generation ([3.14](14-ai-challenge-generation.md)).
- Deleting a challenge (history is never destroyed; a later spec decides archiving).
- Types other than `ARCHITECTURE`.
