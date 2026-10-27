# Phase 3.2 — Rubrics

## Goal

A rubric is the list of criteria an evaluation scores a solution against. This spec adds the
rubric model, the built-in base rubric, immutable per-challenge rubric versions and their SQLite
schema. It does not evaluate anything: scoring and pass rules are [3.5](05-evaluation-engine.md)
and [3.6](06-completion-rules.md).

Part of the Exercise Engine ([ADR-005](../../decisions/ADR-005-exercise-engine.md)); like
[3.1](01-challenge-domain-and-persistence.md), it knows nothing about diagrams.

## Decisions

1. **Two layers, not three.** The original idea had a base rubric, a topic rubric and a challenge
   rubric. Topics now come from free-text study sessions (3.1), so a topic rubric would be matched
   by an unreliable string and would duplicate what the challenge rubric already says. The
   challenge rubric is written for that exact subject. A topic layer can be added by a later spec
   if a real need appears.
2. **The base rubric lives in code.** It ships with the app, is identical for everyone and is
   not editable in the UI. Changing it means shipping a new app version, which is intended: it
   keeps the evaluation floor consistent. Challenge criteria are data in SQLite.
3. **One merged, frozen rubric per challenge version.** When a challenge gets its rubric, the base
   criteria and the challenge criteria are merged into a single `RubricVersion` whose weights sum
   to 100. Each criterion records its origin (`BASE` or `CHALLENGE`). The UI shows one list, and
   evaluation never has to re-assemble layers.
4. **Versions are immutable.** A `RubricVersion` is never updated. Changing a challenge's rubric
   creates version N+1 for that challenge. An attempt references a specific version id
   ([3.3](03-attempts-and-session-link.md)), so later edits never rewrite past evaluations.
5. **Described levels, 0 to 4.** Every criterion has exactly five levels, each with a short
   description of what it means (0 absent … 4 excellent). The evaluator picks a level and
   justifies it; a criterion's score is `level / 4` and the rubric score is the weighted sum on
   0–100 (computed in 3.5). Fixed descriptors reduce variance between runs and are testable.
6. **Mandatory criteria are declared here, enforced in 3.6.** A criterion may be marked
   `Mandatory`. This spec only stores the flag. What a failed mandatory criterion does to
   completion is the pass policy's job. No base criterion is mandatory by default.
7. **Who writes challenge criteria.** The service accepts them as input and validates them. In
   this phase they come from AI generation ([3.14](14-ai-challenge-generation.md)) or from tests.
   There is no manual rubric editor UI.
8. **The understanding evaluation reuses these criteria.** The defense
   ([3.12](12-architecture-defense.md)) scores the user's explanations per criterion of the same
   frozen version, so no second rubric exists.

## The base rubric

Five criteria, initial weight 20 each, with the five descriptors written in the code and covered
by tests. When merged with challenge criteria the weights are rebalanced so the total is 100
(see below).

| Key | Name | What it looks at |
|---|---|---|
| `architecture` | Overall architecture | Components and data flow are defined and coherent |
| `scalability` | Scalability | Meets the stated load; names how it scales |
| `consistency` | Consistency | Data correctness, staleness and propagation are addressed |
| `resilience` | Resilience | Failure scenarios and degraded behavior are handled |
| `tradeoffs` | Trade-offs | Choices are justified and alternatives are discussed |

The base rubric carries its own identifier (`base-v1`). The merged version records which base
version it used.

## Domain

`internal/domain/exercise/`:

```go
type CriterionSource string // CriterionSourceBase, CriterionSourceChallenge

type RubricCriterion struct {
    ID          string
    Key         string        // stable within a version, e.g. "scalability"
    Name        string
    Description string
    Weight      int           // whole percentage points
    Source      CriterionSource
    Mandatory   bool
    Levels      [5]string     // descriptor for levels 0..4, index = level
}

type RubricVersion struct {
    ID              string
    ChallengeID     string
    Version         int       // 1, 2, 3 ... per challenge
    BaseVersion     string    // e.g. "base-v1"
    Criteria        []RubricCriterion
    CreatedAt       time.Time
}
```

`BaseRubric()` returns the built-in criteria. `MergeRubric(base, challengeCriteria)` returns the
final criteria list with weights normalized to sum to 100.

Merging rules:

- The base criteria keep their order first, then the challenge criteria in the given order.
- The input weights are relative: they are scaled to whole percentages that sum to exactly 100,
  distributing the rounding remainder deterministically (largest remainder, ties by position).
- No criterion ends below 1 point; keys are unique and a challenge criterion may not reuse a
  base key.

`RubricVersion.Validate()` checks: at least the five base criteria are present, 1 to 6 challenge
criteria, at most 11 in total, weights sum to exactly 100, every name, description and level
descriptor is non-empty and bounded, keys are unique.

Port:

```go
type RubricRepository interface {
    Create(ctx, RubricVersion) error                  // immutable once stored
    Get(ctx, versionID string) (RubricVersion, error)  // ErrRubricNotFound
    Latest(ctx, challengeID string) (RubricVersion, error)
    ListVersions(ctx, challengeID string) ([]RubricVersion, error)
}
```

There is no `Update` and no `Delete`.

## Application

`internal/application/exercise.RubricService`:

- `SetChallengeRubric(ctx, challengeID, []ChallengeCriterionInput)`: loads the challenge
  (`ErrChallengeNotFound`), merges with `BaseRubric()`, validates, assigns the next version number
  and persists. Calling it again creates the next version.
- `Get(ctx, versionID)`, `Latest(ctx, challengeID)` and `ListVersions`.

A challenge has no rubric until `SetChallengeRubric` runs. Asking for the latest rubric of a
challenge without one returns `ErrRubricNotFound`; starting an attempt on such a challenge is
refused by [3.4](04-exercise-service.md).

## SQLite

New migration in `internal/infrastructure/sqlite/migrations.go`:

```sql
CREATE TABLE rubric_versions (
  id           TEXT PRIMARY KEY,
  challenge_id TEXT NOT NULL REFERENCES challenges(id),
  version      INTEGER NOT NULL,
  base_version TEXT NOT NULL,
  created_at   TEXT NOT NULL,
  UNIQUE (challenge_id, version)
);
CREATE TABLE rubric_criteria (
  id                TEXT PRIMARY KEY,
  rubric_version_id TEXT NOT NULL REFERENCES rubric_versions(id),
  position          INTEGER NOT NULL,
  key               TEXT NOT NULL,
  name              TEXT NOT NULL,
  description       TEXT NOT NULL,
  weight            INTEGER NOT NULL,
  source            TEXT NOT NULL,
  mandatory         INTEGER NOT NULL,
  levels            TEXT NOT NULL,   -- JSON array of 5 strings, see jsonlist.go
  UNIQUE (rubric_version_id, key),
  UNIQUE (rubric_version_id, position)
);
```

Neither table cascades: a challenge is never deleted (3.1), and rubric history must never be
removed by a side effect. A version and its criteria are written in one transaction.

## Tasks

Each slice is TDD, keeps the build green and is committed on its own.

- [ ] Domain: criterion and version types, `BaseRubric`, errors
- [ ] Domain: `MergeRubric` weight normalization (exact sum 100, deterministic remainder)
- [ ] Domain: `RubricVersion.Validate`
- [ ] SQLite: migration and `RubricRepository` (`Create` in one transaction, `Get`, `Latest`,
      `ListVersions`)
- [ ] Application: `RubricService` (`SetChallengeRubric`, versioning, reads)
- [ ] Mocks regenerated; docs: CHANGELOG entry

## Acceptance Criteria

- `BaseRubric()` returns the five criteria in order, each with five non-empty descriptors.
- Merging always yields weights that are whole numbers summing to exactly 100, the same for the
  same input, with no criterion below 1.
- Rejected: a challenge criterion reusing a base key, duplicate keys, zero or more than six
  challenge criteria, empty text, fewer or more than five levels, weights that cannot sum to 100.
- `SetChallengeRubric` on an unknown challenge fails with `ErrChallengeNotFound` and stores
  nothing.
- Setting the rubric twice creates versions 1 and 2; version 1 is unchanged and still readable;
  `Latest` returns version 2.
- A stored version round-trips every field, including the levels and the mandatory flag, after a
  restart.
- Nothing in the code path can modify or delete a stored version.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  domain/application code, lint and vulnerability checks pass.

## Out of scope

- Scoring, evaluation results and the pass policy ([3.5](05-evaluation-engine.md),
  [3.6](06-completion-rules.md)).
- Topic rubrics, a rubric editor UI, user-editable base criteria.
- AI generation of the challenge criteria ([3.14](14-ai-challenge-generation.md)).
- Linking a rubric to an attempt ([3.3](03-attempts-and-session-link.md)).
