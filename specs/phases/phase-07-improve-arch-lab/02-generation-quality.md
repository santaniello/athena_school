# Phase 7.2 — Generation Quality

## Goal

Fix three problems found by reading the challenges the lab actually generated (six rows in the
local database, all on the topic "cache"):

1. **The statement gives the solution away.** The interview "Sistema de Recomendação de Filmes com
   Cache" has the technique in its title and a non-functional requirement that says "the cache hit
   rate must be above 80%". The learner is handed the answer to the question the exam asks.
2. **The rubric is bloated and its weights starve.** That same interview has 11 criteria for a
   beginner in 30 minutes, with merged weights of 13, 10, 10, 8, 7 and **2** for the six challenge
   criteria. A criterion worth 2% cannot orient the learner and only adds noise to the evaluation.
3. **Near-duplicates.** Three practice challenges are all "Cache-Aside for a product catalog", with
   different titles and almost identical rubrics (40/35/25, 40/30/30, 40/30/30). Two interviews are
   both "product recommendations".

These are small, independent fixes to the generation of [3.6.1](../phase-03-architecture-lab/06-01-challenge-generation.md),
[3.2](../phase-03-architecture-lab/02-rubrics.md) and [3.18](../phase-03-architecture-lab/18-practice-challenges.md).
[7.3](03-canonical-interviews.md) changes where interviews come from; the rules here still apply to
what it generates.

## Findings that shape the design

- `MergeRubric` scales **all** weights, base and challenge together, to sum to 100. The base
  criteria weigh 20 each (100 in total) and the model writes the challenge weights on a scale of its
  own choosing, because the prompt only says "a positive whole number". In the two generated
  interviews the challenge weights merged to 50, which means the model happened to write weights
  adding up to 100. A model that wrote weights adding up to 30 would give the challenge criteria
  about 23% and the five base criteria the rest. The base/challenge split is decided by an accident
  of the model's output, not by a rule.
- The 2% criterion ("Monitoramento e Métricas") is a consequence of the same thing: the smallest
  weight the model wrote (4 on a scale of 100) is halved by the merge.
- Rubric versions are immutable ([3.2](../phase-03-architecture-lab/02-rubrics.md) decision 4), so
  rubrics already stored keep their weights. Nothing here rewrites them.
- The practice titles differ in wording ("Implementando Cache Aside para Catálogo de Produtos" vs
  "Implementação do padrão Cache-Aside para consultas de produto"), so a check on equal titles would
  never fire.

## Decisions

1. **Fixed shares in an interview rubric.** The five base criteria take **50** points together and
   the challenge criteria take the other **50**. The base criteria split their share by their own
   weights (10 each today). The challenge criteria split theirs by the **relative** weights the
   model wrote, so the scale the model uses stops mattering. A practice rubric has no base
   criteria ([3.18](../phase-03-architecture-lab/18-practice-challenges.md) decision 6), so its
   criteria take all **100**.
2. **A floor of 5 points per challenge criterion.** Every challenge criterion first receives 5
   points; what remains of the share is distributed by the relative weights with the same largest
   remainder rule as today (ties by position). With at most 6 criteria in a 50-point share, at least
   20 points are always left to distribute, so the floor is always possible.
3. **Fewer challenge criteria for easier exams.** The most challenge criteria an interview may have
   depends on its difficulty: **3** for beginner, **4** for intermediate, **6** for advanced.
   `MaxPracticeCriteria` stays 3 for practice. `MaxChallengeCriteria` (6) stays the absolute limit
   of `RubricVersion.Validate`, which is **unchanged**, so rubric versions already stored stay
   valid. The cap is checked when the generated output is parsed: more criteria than the cap is
   `ErrInvalidGeneration`, nothing is stored and the user retries, like any other invalid output
   ([3.6.1](../phase-03-architecture-lab/06-01-challenge-generation.md) decision 9). Truncating the
   list was rejected: it could silently drop a mandatory criterion.
4. **The prompt states the weight scale and the cap.** It says the challenge weights are relative
   to each other and tells the model its criteria limit for the difficulty. A test asserts both
   instructions are present.
5. **A shorter beginner statement.** For a beginner interview the prompt asks for at most 3
   functional and 3 non-functional requirements, 2 constraints and 3 objectives. This is prompt
   guidance only, checked by a test that the instruction is present, not by validation.
6. **The statement names the target, never the mechanism.** The interview prompt gains an explicit
   rule: the title and every requirement, constraint and objective state a measurable target or a
   need, and never a technique or component that would solve it (cache, queue, replica, CDN,
   sharding and the like). It carries one allowed and one forbidden example: "p95 latency below
   300 ms" is allowed, "cache hit rate above 80%" is not. This is **prompt-only**: a list of
   forbidden words would be brittle and wrong for topics where the word is the problem itself, so
   there is no deterministic check, only a test that the instruction is present. This does not
   change practice challenges: there the user names the concept to train
   ([3.18](../phase-03-architecture-lab/18-practice-challenges.md) decision 4), so the technique may
   appear in the title.
7. **Avoiding repeats.** Generation from a session passes the titles of that session's existing
   challenges of the **same kind** (at most the 10 most recent) in an `<existing_challenges>` tag,
   escaped like every other learner field, with the instruction to choose a different angle or
   sub-aspect of the topic than those. It is prompt-only, for the reason in the findings; there is
   no code check on titles. A session with no earlier challenge sends no tag.
8. **Existing challenges and rubrics are untouched.** No migration and no re-weighting.
9. **No data compatibility to keep.**

## Changes to earlier specs

- **3.2:** the merge rules gain fixed shares and the floor (decisions 1 and 2). The input weights
  stay relative; their scale no longer affects the result. `Validate` is unchanged.
- **3.6.1:** the interview prompt gains decisions 4 to 6, the output cap of decision 3 is checked
  after parsing, and the context gains `<existing_challenges>` (decision 7).
- **3.18:** the practice prompt gains the weight-scale line and `<existing_challenges>`; the floor
  applies to its criteria.

## Domain

`internal/domain/exercise/`:

- `MergeRubric(base, challenge)` keeps its signature. A non-empty `base` means an interview split
  (50/50); an empty one means the lean rubric (all 100 to the challenge criteria). Pure and
  deterministic, covered by tests for: the split, the floor, relative weights independent of scale,
  rounding remainder to the largest fractions, ties by position, and exact sum 100.
- `MaxChallengeCriteriaFor(kind ChallengeKind, difficulty Difficulty) int`: 3 for practice; 3, 4
  and 6 for an interview by difficulty.
- `ErrChallengeCriteriaCount` is returned when a generated rubric exceeds that cap.

## Application

`internal/application/exercise`:

- `generation_prompt.go`: the instructions of decisions 4 to 7, built as pure functions of the
  spec, difficulty and context, as today.
- `GenerationService`: after parsing, checks the criteria count against
  `MaxChallengeCriteriaFor`; loads the existing titles through a narrow port.
- `ChallengeRepository` gains `ListTitlesBySourceSession(ctx, sessionID string, kind ChallengeKind,
  limit int) ([]string, error)`, newest first. An empty session id returns none.

## SQLite

No migration. `ListTitlesBySourceSession` is a read-only query on `challenges`.

## Tasks

Each slice is TDD (Go with testify and Mockery mocks), keeps the build green and is committed on
its own.

- [ ] Domain: `MergeRubric` fixed shares and floor
- [ ] Domain: `MaxChallengeCriteriaFor`; generation rejects output above the cap
- [ ] Application: interview prompt (weight scale, cap, short beginner statement, target not
      mechanism) with tests that each instruction is present
- [ ] Application: practice prompt weight-scale line
- [ ] SQLite and application: `ListTitlesBySourceSession`, `<existing_challenges>` in the context
      for both kinds, escaped and bounded
- [ ] Mocks regenerated; docs: CHANGELOG (README unchanged: no new user-visible surface)

## Acceptance Criteria

- An interview rubric always has base criteria summing to 50 and challenge criteria summing to 50,
  whatever scale the model used for its weights: the challenge weights `[26,20,20,16,14,4]` and
  the same weights doubled (`[52,40,40,32,28,8]`) produce the same distribution.
- No challenge criterion of an interview ends below 5 points; no weight is zero; the total is
  exactly 100; the result is the same for the same input.
- A practice rubric sums to 100 over its 1 to 3 criteria, each at least 5.
- A generated interview with more criteria than the cap for its difficulty (more than 3 for a
  beginner) fails with `ErrInvalidGeneration`, stores nothing and costs no extra call.
- The interview prompt contains the weight-scale line, the criteria cap for the difficulty, the
  beginner statement sizes and the target-not-mechanism rule with its two examples.
- When the session already has challenges of the same kind, the prompt lists up to 10 of their
  titles inside an escaped tag; a title containing tag text cannot close the tag.
- Rubric versions already stored are unchanged and still read back and validate.
- `go test -race ./...`, coverage ≥ 80%, `make mutation-go` clean for the changed
  application/domain code, lint and vulnerability checks pass.

## Out of scope

- A deterministic check that a title or requirement does not name the solution.
- Detecting duplicates in code, by title or by embedding similarity.
- Re-weighting or regenerating existing rubrics and challenges.
- A different rubric for practice beyond the floor, and per-difficulty caps for practice.
- Where interviews come from ([7.3](03-canonical-interviews.md)).
