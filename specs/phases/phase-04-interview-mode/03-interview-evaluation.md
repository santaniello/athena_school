# Phase 4.3 — Interview Evaluation

## Goal

Each answer is evaluated individually; a final aggregate score is computed and displayed in a report.

## Flow

```text
All answers collected
    ↓
Evaluation Engine called per answer → Evaluation[]
    ↓
Aggregate score = mean(Evaluation.Score)
    ↓
Report screen: per-question breakdown + overall score
```

## Tasks

- [ ] Reuse `Evaluation` domain and `EvaluationEngine` from Phase 3.2
- [ ] Per-answer evaluation result linked to the `interview_session` via `session_id`
- [ ] Aggregate score computed in `internal/application/interview/`
- [ ] Report UI:
  - Overall score badge
  - Per-question card: question text, answer text, strengths, improvements, score
- [ ] Report persisted: evaluations linked to `interview_session`

## Acceptance Criteria

- After an interview with 3 questions, 3 evaluation records exist in the `evaluations` table
- Report screen shows each question with its individual score and feedback
- Overall score equals the mean of individual scores (rounded to nearest integer)
- Report is accessible again from interview history (see 4.4)
