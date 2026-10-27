# ADR-005 — Exercise Engine

**Status:** Accepted
**Date:** 2026-10-27

---

## Context

Phase 3 ([phase-03-architecture-lab](../phases/phase-03-architecture-lab/)) introduces the Architecture Lab: challenges solved on a diagram whiteboard, evaluated against a rubric by an LLM, and defended orally with the tutor. A challenge can start from three entry points:

| Entry point | Study session link |
|---|---|
| The user asks the tutor for it in a study session, and accepts the offer | Required |
| The user picks a challenge from the lab catalog | None (standalone) |
| The user asks the AI to generate a challenge inside the lab | None (standalone) |

All three must share the same lab screen, the same persistence and the same evaluation. A second lab, an Algorithms Lab in the style of HackerRank (a code editor, user code run against test cases in a sandbox, a score from tests instead of an LLM rubric), is **planned** but not specified. Nothing in the product needs it in this phase.

`AGENTS.md` forbids abstractions for hypothetical future needs. This ADR therefore decides how the lifecycle is **named and layered**, not how many labs it supports.

---

## Decision

1. **One exercise lifecycle, independent of the whiteboard.** The entities `Challenge`, `Rubric`/`RubricVersion`, `ChallengeAttempt`, `Submission`, `Evaluation`/`CriterionResult`, `ChallengeProgress` and `SessionExercise` live in `internal/domain/exercise/` and carry no diagram-specific field. Diagram content is an opaque artifact (a typed payload plus a checksum) that the Architecture Lab interprets.

2. **The architecture-specific code lives in its own package** (`internal/domain/architecture` and its application and infrastructure counterparts): diagram format handling, the evaluator prompt, the defense questions. `exercise` never imports `architecture`; the dependency points the other way, per [ADR-001](ADR-001-hexagonal-architecture.md).

3. **No `LabProvider` interface and no second-lab mock in this phase.** A `LabProvider` would let the engine call any lab through one interface (create the challenge, build the editor, evaluate the submission, run the defense). We do not build it: the evaluator is a plain port in `internal/domain/exercise` with one implementation. The Algorithms Lab is planned, but it differs deeply from this one (running user code against tests in a sandbox, a score from tests, another editor and statement), so an interface drawn from a single example would likely have the wrong shape and become the thing that gets in the way. When the Algorithms Lab is specified, extracting a provider registry is the refactor that spec owns, with two real labs to draw it from. The boundary in (1) and (2) is what keeps that refactor cheap, and a package-import check in CI (`exercise` must not import `architecture`) guards it.

4. **Challenge origin and execution context are two separate concepts**, never one enum:
   - origin: `MANUAL` | `AI_GENERATED`
   - context of an attempt: `STUDY_SESSION` | `STANDALONE`, derived from whether the attempt has a `SessionExercise`
   - there is no modality: every attempt is an exam (decision 9)

5. **A challenge definition is never owned by a study session.** `SessionExercise` links a session to a challenge, and is created before the first attempt. Deleting a session deletes its `SessionExercise` rows and nothing else: challenges, attempts, submissions and evaluations stay in the global history. This is deliberate and different from knowledge documents, which a session owns ([spec 2.15](../phases/phase-02-knowledge-engine/15-session-scoped-knowledge.md)).

6. **Submitting is not completing.** An attempt is evaluated once, when the user submits. The diagram evaluation is immutable. Completion is a derived status from a pass policy (minimum score, mandatory criteria, finished defense), never a flag set by the UI. A criterion the diagram does not show is flagged, left out of the diagram score and settled by the defense; it is never inferred.

7. **The oral defense is bounded by domain rules, not by the LLM.** After the diagram evaluation the tutor asks exactly `N` questions (set per challenge, default 5), one at a time and adapted to the previous answers, one answer each, no follow-ups, and the defense ends when the count is reached. The model cannot extend it. Which criterion each question targets is chosen by code. The defense produces a second, separate `UNDERSTANDING` evaluation per rubric criterion, computed once after the last answer. Details belong to the defense spec.

8. **Rubric versions are immutable once an attempt uses them.** An attempt freezes the rubric version it was evaluated with, so later edits never rewrite history.

9. **An attempt is an exam, not a work-in-progress.** One attempt is one sitting: there is no draft, no pause, no resume and no intermediate version. The challenge defines a time limit (default by difficulty), the backend records the start and the deadline, and the clock never stops. The sitting has consecutive stages on one screen: drawing, the diagram evaluation, the tutor's questions and the result. The drawing ends in one of two ways: the user submits, or the time runs out and what is on the board is submitted. If the user leaves the lab, closes the app or the app crashes at any point before the last answer is evaluated, the attempt is **discarded**: no submission, no score, no history entry. Discarding is done on leave and, for crashes, on the next application start, which deletes every attempt that is not finished. Only a finished attempt is stored, read-only, and stays consultable; nothing about it can be edited. To practice again the user picks the same challenge in the catalog, which starts a new, independent attempt on the same challenge (same scenario). Challenges are reusable and attempts are never reopened.

10. **Local-first and cost.** All of it persists in the existing SQLite database (no CGO, [ADR-004](ADR-004-local-vector-store.md) rationale). The AI calls of the lab use one configurable model, with its own setting. LLM calls per attempt are bounded and known: one diagram evaluation, up to three hints, `N` defense questions and one understanding evaluation.

---

## Consequences

**Positive:**
- The three entry points are one code path, tested once.
- Session deletion can never destroy the user's evaluation history.
- The user can always see what they submitted and what they were told, and cannot rewrite it.
- A future second lab needs a new `architecture`-like package and one extracted port, not a rewrite of the lifecycle.
- The attempt cost and the defense length are predictable and testable.

**Negative:**
- Naming the lifecycle generically costs a little indirection (an opaque artifact payload) before a second lab exists. We accept it because the alternative, diagram fields on `Submission`, is the rewrite this ADR is meant to avoid.
- Without a mock second lab, the genericity of `exercise` is enforced only by the import check and review, not by an executable proof. When the Algorithms Lab is specified, its spec must include that proof.
- `Challenge` is not fully generic today. Its four lists (functional requirements, non-functional requirements, constraints, objectives) are shaped for architecture, and the `Type` field only has the value `ARCHITECTURE`. An algorithm challenge would carry test cases, languages and limits instead, so the Algorithms Lab will probably need a statement payload per type, or a split of `Challenge`. We accept that refactor as future work rather than designing the payload now.
- The `MANUAL` challenge origin has no screen that writes it yet. The field stays because manual creation is planned; until then every challenge is `AI_GENERATED`.
