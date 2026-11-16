# ADR-007 — Interactive Quiz as a Separate Context

**Status:** Accepted
**Date:** 2026-11-16

---

## Context

The Interactive Quiz ([spec 3.21](../phases/phase-03-architecture-lab/21-interactive-quiz-core.md))
is an exam of multiple-choice and true/false questions about a study session, graded 0–10. The
exercise engine ([ADR-005](ADR-005-exercise-engine.md)) already covers generation, attempts and
history for the Architecture Lab.

## Decision

1. **The quiz gets its own packages**: `internal/domain/quiz`, `internal/application/quiz` and
   `quiz_*` SQLite repositories. It does not import `exercise`, and `exercise` does not import it,
   per [ADR-001](ADR-001-hexagonal-architecture.md).
2. **Why not extend the exercise engine.** It models a rubric with weighted criteria, an LLM
   evaluation, an oral defense, one in-progress attempt per challenge and a 0–100 score. A quiz has
   none of these: grading is a count of correct answers and there is no state between start and
   submit. Forcing it in would add quiz fields to a deliberately generic domain, or empty
   rubrics to fake the fit.
3. **What is shared is shared through ports, not imports.** Session context is read through a
   consumer-side port with its own adapter. The `Transactor`, the language helper and the
   generation cancellation pattern are copied in shape, not imported across contexts.
4. **The one place the contexts meet is the study chat offer** (spec 3.23). The quiz keeps its own
   `quiz_offers` table and offer service, and the study context sees both through one `OfferGate`
   composed in `main.go`. That is plumbing in the study context, not a dependency between the
   two. (An earlier draft planned a `kind` column on `exercise_offers`; it would have put a quiz
   concept inside the exercise context.)

## Consequences

Some duplication (language name helper, tx usage) is accepted, as in ADR-005's `languageName`.
If a third assessment type appears, extracting a shared offer and history layer is the refactor
that spec owns.
