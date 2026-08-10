# Phase 4.1 — Interview Session

## Goal

LLM conducts a multi-question progressive interview, deepening questions based on previous answers.

## Domain

```go
type InterviewSession struct {
    ID        string
    Topic     string
    Mode      string      // system_design | behavioral | domain_specific
    Questions []Question
    Answers   []Answer
    Score     int
    StartedAt time.Time
    EndedAt   time.Time
}
```

## Domain Mapping (from UserProfile)

| Area | Interview Mode |
|---|---|
| Software / IT | system_design |
| Law | domain_specific (legal cases) |
| Veterinary | domain_specific (clinical cases) |
| Other | behavioral / domain_specific |

## Tasks

- [ ] `internal/domain/interview/` — session domain, question/answer lifecycle
- [ ] LLM receives full conversation history per turn to enable progressive deepening
- [ ] Topic selectable from UI; mode inferred from `UserProfile.Area`
- [ ] Session persisted to `sessions` table with `mode = "interview"`
- [ ] Questions and answers stored in `messages` table

## Acceptance Criteria

- User selects a topic and starts an interview
- First question is generated; subsequent questions reference previous answers
- Session is persisted to SQLite on completion
- Interview mode matches the user's area from `UserProfile`
