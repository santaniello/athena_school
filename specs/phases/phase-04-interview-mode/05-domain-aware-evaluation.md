# Phase 4.5 — Domain-Aware Evaluation

## Goal

Evaluation criteria adapt to the user's professional area so feedback is contextually meaningful.

## Criteria by Domain

```text
IT / System Design:
    Scalability, Trade-offs, Correctness, Design quality

Law:
    Legal grounding, Argumentation, Legislation cited, Precision

Veterinary Medicine:
    Differential diagnosis, Protocol adherence, Clinical reasoning

Competitive exams:
    Completeness, Factual accuracy, Organization
```

## Tasks

- [ ] Evaluation prompt injected with domain-specific criteria from `UserProfile.Area`
- [ ] `internal/application/evaluation/criteria.go` — criteria registry (area → []Criterion)
- [ ] Criteria displayed in the evaluation report so the user knows what was measured
- [ ] Default criteria used for unrecognized areas

## Acceptance Criteria

- An IT user's evaluation prompt includes "Scalability" and "Trade-offs" as criteria
- A Law user's prompt includes "Legal grounding" and "Argumentation"
- The report shows the criteria that were applied
- An unknown area falls back to generic criteria without error
