# Phase 3.4 — Gap Detection

## Goal

Automatically identify topics where the user consistently struggles and surface them as actionable gaps.

## Algorithm

```text
For each topic with ≥ 2 sessions:
    average_score = mean(progress.score WHERE topic = T)
    if average_score < threshold (default: 60) → mark as gap
```

## Tasks

- [ ] `internal/application/gaps/` — gap analysis use case
- [ ] Configurable threshold (default 60); stored in `config.yaml`
- [ ] Gap detection runs after every new evaluation
- [ ] UI: gap dashboard with per-topic indicators: ✅ strong / ⚠️ weak / ❌ gap
- [ ] Automatic suggestion: "You haven't practiced X in a while — start a session?"
- [ ] Gaps feed into flashcard error data (see 3.5)

## Acceptance Criteria

- A topic with average score < 60 across 2+ sessions is flagged as a gap
- Gap dashboard shows the correct indicator for each topic
- Suggestion card appears for the topic with the lowest average score
- Changing the threshold in settings recalculates gaps immediately
