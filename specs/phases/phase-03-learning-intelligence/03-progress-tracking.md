# Phase 3.3 — Progress Tracking

## Goal

Aggregate evaluation scores over time per topic and display the user's evolution.

## Schema

```sql
CREATE TABLE progress (
    id          TEXT PRIMARY KEY,
    topic       TEXT,
    subtopic    TEXT,
    mode        TEXT,       -- study | challenge | interview
    score       INTEGER,
    session_id  TEXT,
    recorded_at DATETIME
);
```

## Tasks

- [ ] `internal/domain/progress/` — metrics aggregation logic
- [ ] A progress record is written after every evaluated session
- [ ] Metrics per topic: average score, session count, hit rate, total time
- [ ] UI: progress screen with per-topic bars and trend indicators
- [ ] Wails binding: `GetProgress(topic string) ProgressSummary`

## Acceptance Criteria

- After 3 challenge sessions on the same topic, the progress screen shows the average score and session count
- Progress bars visually reflect the score (e.g., red < 50, yellow 50–80, green > 80)
- Navigating to a topic detail shows individual session history
