# Phase 3.2 — Evaluation Engine

## Goal

LLM evaluates answers and returns a structured result persisted to SQLite and displayed to the user.

## Domain

```go
type Evaluation struct {
    Score       int      `json:"score"`        // 0–100
    Strengths   []string `json:"strengths"`
    Weaknesses  []string `json:"weaknesses"`
    Missing     []string `json:"missing_topics"`
    Suggestions []string `json:"suggestions"`
}
```

## Schema

```sql
CREATE TABLE evaluations (
    id          TEXT PRIMARY KEY,
    session_id  TEXT,
    score       INTEGER,
    strengths   TEXT, -- JSON array
    weaknesses  TEXT, -- JSON array
    missing     TEXT, -- JSON array
    suggestions TEXT, -- JSON array
    created_at  DATETIME
);
```

## Tasks

- [ ] `internal/domain/evaluation/` — `Evaluation` struct + validation
- [ ] LLM prompt instructs strict JSON output; Go validates schema before persisting
- [ ] `internal/application/evaluation/` — evaluation use case
- [ ] UI: results screen with score badge, strengths list, improvements list, suggestions
- [ ] Evaluation criteria configurable by domain (injected from `UserProfile`)

## Acceptance Criteria

- Submitting a challenge answer produces an `Evaluation` with all fields populated
- Score and lists are shown in the results UI
- Malformed LLM JSON is caught and returned as an error (no silent data loss)
- Evaluation is persisted to `evaluations` table with the correct `session_id`
